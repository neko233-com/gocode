package search

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/neko233-com/godesktop/editor"
)

const MaxReplaceFileBytes = 8 << 20
const MaxReplaceBytes = 64 << 20
const MaxReplaceFiles = 128

// ReplacementPlan and its snapshots belong to one worker until publication.
// Afterward they are immutable. New buffers transfer to the UI exactly once.
type ReplacementPlan struct {
	Files  []ReplacementFile
	Count  int
	Bytes  int
	Values []string
}
type ReplacementFile struct {
	Path     string
	Identity uint64
	Before   editor.Snapshot
	Prepared *editor.PreparedEdit
	New      *editor.Buffer // Only a previously closed file owns this private buffer.
	DiskHash [32]byte
	Edits    []editor.Edit
}

// PrepareReplacement verifies the complete result set against current immutable
// sources before producing any edits. It never changes a buffer or writes disk.
func PrepareReplacement(ctx context.Context, root string, q Query, report Report, replacement string, overlays map[string]Overlay) (*ReplacementPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if report.Err != nil || report.Limited || report.Unreadable+report.Changed != 0 || len(report.Matches) == 0 || len(report.Matches) > MaxResults {
		return nil, errors.New("replace requires a complete successful search; refine filters or search again")
	}
	if len(replacement) > MaxQueryBytes || !utf8.ValidString(replacement) || strings.ContainsRune(replacement, 0) {
		return nil, errors.New("replacement needs valid UTF-8 without NUL, at most 4096 bytes")
	}
	expr, err := compile(q)
	if err != nil {
		return nil, err
	}
	fs, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer fs.Close()
	first, next := regexp.MustCompile("\\A"+expr.first.String()), regexp.MustCompile("\\A(?s:.)"+expr.first.String())
	plan := &ReplacementPlan{}
	seen := map[string]bool{}
	for from := 0; from < len(report.Matches); {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path := report.Matches[from].Path
		if !validRelative(path) || seen[path] || len(plan.Files) >= MaxReplaceFiles {
			return nil, errors.New("invalid or over-limit replacement paths")
		}
		seen[path] = true
		end := from + 1
		for end < len(report.Matches) && report.Matches[end].Path == path {
			end++
		}
		old := report.Matches[from:end]
		file := ReplacementFile{Path: path}
		if overlay, ok := overlays[path]; ok {
			file.Before, file.Identity = overlay.Snapshot, overlay.Identity
			for _, match := range old {
				if match.Identity != overlay.Identity || match.Version != overlay.Snapshot.Version {
					return nil, fmt.Errorf("%s: editor changed; search again", path)
				}
			}
		} else {
			for _, match := range old {
				if match.Identity != 0 {
					return nil, fmt.Errorf("%s: editor was closed or reopened", path)
				}
			}
			info, err := fs.Lstat(filepath.FromSlash(path))
			if err != nil {
				return nil, err
			}
			if !info.Mode().IsRegular() || info.Mode().Perm()&0222 == 0 || info.Size() > MaxReplaceFileBytes {
				return nil, fmt.Errorf("%s: replacement requires a writable editable file (at most 8 MiB)", path)
			}
			f, err := fs.Open(filepath.FromSlash(path))
			if err != nil {
				return nil, err
			}
			var readBytes int64
			data, readErr := io.ReadAll(io.LimitReader(&contextReader{ctx: ctx, r: f, bytes: &readBytes}, MaxReplaceFileBytes+1))
			after, statErr := fs.Stat(filepath.FromSlash(path))
			closeErr := f.Close()
			if err := errors.Join(readErr, statErr, closeErr); err != nil {
				return nil, err
			}
			if len(data) > MaxReplaceFileBytes || !os.SameFile(info, after) || info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) {
				return nil, fmt.Errorf("%s: disk changed during preview", path)
			}
			file.DiskHash = sha256.Sum256(data)
			file.New, err = editor.New(string(data))
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			file.Before = file.New.Snapshot()
		}
		// Reject oversized live snapshots before joining their line strings.
		sourceBytes := 0
		for i := 0; i < file.Before.LineCount(); i++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			sourceBytes += len(file.Before.Line(i))
			if i > 0 {
				sourceBytes += len(file.Before.EOL)
			}
			if sourceBytes > MaxReplaceFileBytes || plan.Bytes+sourceBytes > MaxReplaceBytes {
				return nil, errors.New("replacement exceeds 8 MiB file / 64 MiB batch source and output policy")
			}
		}
		text := file.Before.Text()
		if len(text) > MaxReplaceFileBytes || plan.Bytes+len(text) > MaxReplaceBytes {
			return nil, errors.New("replacement exceeds 8 MiB file / 64 MiB batch source and output policy")
		}
		var actual Report
		if err := searchSource(ctx, strings.NewReader(text), int64(len(text)), path, expr, file.Before.Version, file.Identity, &actual); err != nil {
			return nil, err
		}
		if len(actual.Matches) != len(old) {
			return nil, fmt.Errorf("%s: matches changed; search again", path)
		}
		outputBytes := len(text)
		for i, match := range old {
			current := actual.Matches[i]
			if current.Start != match.Start || current.End != match.End || current.Range != match.Range || current.Hash != match.Hash {
				return nil, fmt.Errorf("%s: result content or coordinates changed", path)
			}
			value := replacement
			if q.Regex {
				base, re := int(match.Start), first
				if base > 0 {
					_, width := utf8.DecodeLastRuneInString(text[:base])
					base -= width
					re = next
				}
				indices := re.FindStringSubmatchIndex(text[base:])
				if len(indices) < 4 || base+indices[2] != int(match.Start) || base+indices[3] != int(match.End) {
					return nil, fmt.Errorf("%s: regex captures changed", path)
				}
				for index := range indices {
					if indices[index] >= 0 {
						indices[index] += base
					}
				}
				value, err = expandReplacement(ctx, replacement, text, indices, re, MaxReplaceFileBytes)
				if err != nil {
					return nil, err
				}
			}
			outputBytes += len(value) - int(match.End-match.Start)
			if outputBytes > MaxReplaceFileBytes || plan.Bytes+len(text)+outputBytes > MaxReplaceBytes {
				return nil, errors.New("replacement output exceeds bounded preview policy")
			}
			file.Edits = append(file.Edits, editor.Edit{Range: match.Range, Text: value})
			plan.Values = append(plan.Values, value)
		}
		file.Prepared, err = file.Before.Prepare(ctx, file.Edits, nil)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		// Account actual normalized output too: replacement LF follows source EOL.
		outputBytes = 0
		after := file.Prepared.Snapshot()
		for i := 0; i < after.LineCount(); i++ {
			outputBytes += len(after.Line(i))
			if i > 0 {
				outputBytes += len(after.EOL)
			}
		}
		if outputBytes > MaxReplaceFileBytes || plan.Bytes+len(text)+outputBytes > MaxReplaceBytes {
			return nil, errors.New("normalized replacement exceeds preview policy")
		}
		plan.Bytes += len(text) + outputBytes
		plan.Count += len(file.Edits)
		plan.Files = append(plan.Files, file)
		from = end
	}
	return plan, nil
}

// Capture offsets include the search wrapper at slot 1. User $N starts at N+1.
// Literal replacements are not parsed. Regex supports JS-style captures, $0,
// escaped newline/tab/backslash and per-capture Unicode case operations.
func expandReplacement(ctx context.Context, pattern, source string, indices []int, re *regexp.Regexp, limit int) (string, error) {
	var output strings.Builder
	appendText := func(text string) error {
		if output.Len()+len(text) > limit {
			return errors.New("expanded replacement exceeds output limit")
		}
		output.WriteString(text)
		return nil
	}
	capture := func(index int) (string, bool) {
		if index < 1 || 2*index+1 >= len(indices) {
			return "", false
		}
		if indices[2*index] < 0 {
			return "", true
		}
		return source[indices[2*index]:indices[2*index+1]], true
	}
	for i := 0; i < len(pattern); {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		start := i
		var operations []byte
		for i+1 < len(pattern) && pattern[i] == '\\' && strings.ContainsRune("uUlL", rune(pattern[i+1])) {
			operations = append(operations, pattern[i+1])
			i += 2
		}
		var value string
		if i+1 < len(pattern) && pattern[i] == '$' {
			i += 2
			ch := pattern[i-1]
			switch ch {
			case '$':
				value = "$"
			case '0', '&':
				value, _ = capture(1)
			case '`':
				value = source[:indices[2]]
			case '\'':
				value = source[indices[3]:]
			case '<':
				if end := strings.IndexByte(pattern[i:], '>'); end >= 0 {
					name := pattern[i : i+end]
					if index := re.SubexpIndex(name); index > 1 {
						value, _ = capture(index)
						i += end + 1
					} else {
						i = start + 1
						value = pattern[start:i]
					}
				} else {
					i = start + 1
					value = pattern[start:i]
				}
			default:
				if ch >= '1' && ch <= '9' {
					number := int(ch - '0')
					value, ok := capture(number + 1)
					if i < len(pattern) && pattern[i] >= '0' && pattern[i] <= '9' {
						if two, exists := capture(number*10 + int(pattern[i]-'0') + 1); exists {
							value, ok = two, true
							i++
						}
					}
					if !ok {
						i = start + 1
						value = pattern[start:i]
					}
					if err := appendText(caseCapture(value, operations)); err != nil {
						return "", err
					}
					continue
				}
				i = start + 1
				value = pattern[start:i]
			}
			if len(operations) > 0 {
				value = pattern[start:start+len(operations)*2] + value
			}
		} else {
			i = start + 1
			value = pattern[start:i]
			if pattern[start] == '\\' && i < len(pattern) {
				switch pattern[i] {
				case 'n':
					value = "\n"
					i++
				case 't':
					value = "\t"
					i++
				case '\\':
					value = "\\"
					i++
				}
			}
		}
		if err := appendText(value); err != nil {
			return "", err
		}
	}
	return output.String(), nil
}
func caseCapture(value string, operations []byte) string {
	runes := []rune(value)
	for i, op := range operations {
		if i >= len(runes) {
			break
		}
		switch op {
		case 'u':
			runes[i] = unicode.ToUpper(runes[i])
		case 'l':
			runes[i] = unicode.ToLower(runes[i])
		case 'U':
			return string(runes[:i]) + strings.ToUpper(string(runes[i:]))
		case 'L':
			return string(runes[:i]) + strings.ToLower(string(runes[i:]))
		}
	}
	return string(runes)
}

// CloneResult prevents a later UI search adoption from retaining a mutable array.
func CloneResult(r Report) Report { r.Matches = slices.Clone(r.Matches); return r }
