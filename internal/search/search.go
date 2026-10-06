// Package search performs bounded, cancellable native workspace text searches.
package search

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"regexp/syntax"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/neko233-com/godesktop/editor"
)

const (
	MaxResults      = 5000
	MaxFiles        = 50000
	MaxEntries      = 1000000
	MaxDirectories  = 10000
	MaxQueryBytes   = 4096
	MaxOverlayBytes = 64 << 20
)

type Query struct {
	Text, Include, Exclude                     string
	CaseSensitive, WholeWord, Regex, UseIgnore bool
}
type Overlay struct {
	Snapshot editor.Snapshot
	Identity uint64
}
type Match struct {
	Path                         string
	Start, End                   int64
	Range                        editor.Range
	Preview, Before, Text, After string
	Hash                         [32]byte
	Version                      int
	Identity                     uint64
}
type Report struct {
	Matches                            []Match
	Files, Binary, Unreadable, Changed int
	Bytes                              int64
	Limited                            bool
	Err                                error
}

type expression struct {
	first, next                 *regexp.Regexp
	literal, scratch            []byte
	raw                         []byte
	starts, ends                []int64
	fold                        bool
	reader                      *bufio.Reader
	word                        bool
	prefix                      *expression
	anchoredFirst, anchoredNext *regexp.Regexp
}

func compile(q Query) (*expression, error) {
	if q.Text == "" || len(q.Text) > MaxQueryBytes || !utf8.ValidString(q.Text) || strings.ContainsRune(q.Text, 0) {
		return nil, errors.New("search needs 1–4096 bytes of valid text")
	}
	body := q.Text
	if q.Regex {
		parsed, err := syntax.Parse(body, syntax.Perl)
		if err != nil {
			return nil, err
		}
		if regexCost(parsed) > 8192 {
			return nil, errors.New("regular expression exceeds the search complexity limit")
		}
		if _, err := regexp.Compile(body); err != nil {
			return nil, err
		}
	} else {
		body = regexp.QuoteMeta(body)
	}
	flags := "m"
	if !q.CaseSensitive {
		flags += "i"
	}
	body = "(?" + flags + ":" + body + ")"
	first, err := regexp.Compile("(" + body + ")")
	if err != nil {
		return nil, err
	}
	// FindReader may look ahead beyond a match. Restart from ReaderAt, including
	// exactly one preceding rune. Consume that rune before the lazy search prefix
	// so ^/\A/\b retain their real context without matching before the restart.
	next, err := regexp.Compile("\\A(?s:.)(?s:.*?)(" + body + ")")
	if err != nil {
		return nil, err
	}
	e := &expression{first: first, next: next, word: q.WholeWord}
	if !q.Regex {
		e.literal = []byte(q.Text)
		e.fold = !q.CaseSensitive
		if e.fold {
			e.literal = []byte(strings.Map(foldRune, q.Text))
			e.raw = make([]byte, 64<<10)
			capacity := (3 * 64 << 10) + len(e.literal)
			e.scratch = make([]byte, capacity)
			e.starts = make([]int64, capacity)
			e.ends = make([]int64, capacity)
		} else {
			e.scratch = make([]byte, (64<<10)+len(e.literal))
		}
	}
	e.reader = bufio.NewReaderSize(strings.NewReader(""), 64<<10)
	if q.Regex {
		if prefix, _ := first.LiteralPrefix(); prefix != "" {
			e.prefix = &expression{literal: []byte(prefix), scratch: make([]byte, (64<<10)+len(prefix))}
			e.anchoredFirst = regexp.MustCompile("\\A(" + body + ")")
			e.anchoredNext = regexp.MustCompile("\\A(?s:.)(" + body + ")")
		}
	}
	return e, nil
}

func regexCost(r *syntax.Regexp) int {
	cost := 1 + len(r.Rune)
	for _, child := range r.Sub {
		cost += regexCost(child)
		if cost > 8192 {
			return 8193
		}
	}
	if r.Op == syntax.OpRepeat {
		count := r.Max
		if count < 0 {
			count = r.Min + 1
		}
		count = max(1, count)
		if cost > 8192/count {
			return 8193
		}
		cost *= count
	}
	return cost
}

// Run captures no mutable document state. Overlay strings are built one file at
// a time on this worker, retaining immutable snapshots rather than UI copies.
func Run(ctx context.Context, root string, q Query, overlays map[string]Overlay) (report Report) {
	expr, err := compile(q)
	if err != nil {
		report.Err = err
		return
	}
	include, err := compileFilters(q.Include)
	if err != nil {
		report.Err = err
		return
	}
	exclude, err := compileFilters(q.Exclude)
	if err != nil {
		report.Err = err
		return
	}
	fs, err := os.OpenRoot(root)
	if err != nil {
		report.Err = err
		return
	}
	defer fs.Close()
	paths, limited, unreadable, err := collect(ctx, fs, q.UseIgnore)
	report.Limited, report.Unreadable, report.Err = limited, unreadable, err
	if err != nil {
		return
	}
	seen := make(map[string]bool, len(paths))
	for _, p := range paths {
		seen[p] = true
	}
	var overlayBytes int64
	for p, o := range overlays {
		if !validRelative(p) {
			report.Err = fmt.Errorf("invalid overlay path %q", p)
			return
		}
		for i := 0; i < o.Snapshot.LineCount(); i++ {
			overlayBytes += int64(len(o.Snapshot.Line(i)) + len(o.Snapshot.EOL))
		}
		if overlayBytes > MaxOverlayBytes {
			report.Err = errors.New("open-document search exceeds 64 MiB snapshot limit")
			return
		}
		// Open editors are authoritative even if their disk path is ignored/new.
		if !seen[p] {
			paths = append(paths, p)
			seen[p] = true
		}
	}
	sort.Strings(paths)
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			report.Err = err
			return
		}
		if !acceptFilter(include, p, true) || acceptFilter(exclude, p, false) {
			continue
		}
		start := len(report.Matches)
		if overlay, ok := overlays[p]; ok {
			text := overlay.Snapshot.Text()
			err = searchSource(ctx, strings.NewReader(text), int64(len(text)), p, expr, overlay.Snapshot.Version, overlay.Identity, &report)
		} else {
			f, openErr := fs.Open(filepath.FromSlash(p))
			if openErr != nil {
				report.Unreadable++
				continue
			}
			before, statErr := f.Stat()
			if statErr != nil || !before.Mode().IsRegular() {
				f.Close()
				report.Unreadable++
				continue
			}
			err = searchSource(ctx, f, before.Size(), p, expr, 0, 0, &report)
			after, statErr := fs.Stat(filepath.FromSlash(p))
			f.Close()
			if statErr != nil || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
				clear(report.Matches[start:])
				report.Matches = report.Matches[:start]
				report.Changed++
				continue
			}
		}
		if err != nil {
			report.Err = err
			return
		}
		if len(report.Matches) >= MaxResults {
			report.Limited = true
			return
		}
	}
	return
}

type contextReader struct {
	ctx     context.Context
	r       io.Reader
	bytes   *int64
	failure *error
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(p)
	*r.bytes += int64(n)
	if err != nil && !errors.Is(err, io.EOF) && r.failure != nil {
		*r.failure = err
	}
	return n, err
}

type source struct {
	at    io.ReaderAt
	size  int64
	ctx   context.Context
	bytes *int64
}

func (s source) readAt(p []byte, off int64) (int, error) {
	if err := s.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := s.at.ReadAt(p, off)
	*s.bytes += int64(n)
	return n, err
}
func (s source) runeAt(off int64) (rune, int) {
	if off < 0 || off >= s.size {
		return -1, 0
	}
	var b [4]byte
	n, _ := s.readAt(b[:], off)
	r, w := utf8.DecodeRune(b[:n])
	return r, w
}
func (s source) previous(off int64) (rune, int64) {
	if off <= 0 {
		return -1, 0
	}
	start := max(0, off-4)
	var b [4]byte
	n, _ := s.readAt(b[:off-start], start)
	r, w := utf8.DecodeLastRune(b[:n])
	return r, off - int64(w)
}
func word(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r)
}

func foldRune(r rune) rune {
	if r < utf8.RuneSelf {
		if r >= 'a' && r <= 'z' {
			return r - 32
		}
		return r
	}
	minimum := r
	for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
		minimum = min(minimum, next)
	}
	return minimum
}

func searchSource(ctx context.Context, at io.ReaderAt, size int64, p string, e *expression, version int, id uint64, report *Report) error {
	s := source{at, size, ctx, &report.Bytes}
	var header [8192]byte
	n, err := s.readAt(header[:], 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if bytes.IndexByte(header[:n], 0) >= 0 {
		report.Binary++
		return nil
	}
	report.Files++
	pos := positionTracker{s: s}
	from, lastEnd := int64(0), int64(-1)
	for from <= size {
		if err := ctx.Err(); err != nil {
			return err
		}
		start, end, err := e.find(s, from)
		if err != nil {
			return err
		}
		if start < 0 {
			return nil
		}
		accept := true
		if e.word {
			left, _ := s.previous(start)
			right, _ := s.runeAt(end)
			accept = !word(left) && !word(right)
		}
		if accept && !(start == end && start == lastEnd) {
			begin, err := pos.advance(start)
			if err != nil {
				return err
			}
			finish, err := pos.advance(end)
			if err != nil {
				return err
			}
			before, text, after, hash, err := preview(s, start, end)
			if err != nil {
				return err
			}
			report.Matches = append(report.Matches, Match{Path: p, Start: start, End: end, Range: editor.Range{Start: begin, End: finish}, Before: before, Text: text, After: after, Preview: before + text + after, Hash: hash, Version: version, Identity: id})
			lastEnd = end
			if len(report.Matches) >= MaxResults {
				return nil
			}
		}
		if !accept || start == end {
			_, w := s.runeAt(start)
			if w == 0 {
				return nil
			}
			from = start + int64(w)
		} else {
			from = end
		}
	}
	return nil
}

func (e *expression) find(s source, from int64) (int64, int64, error) {
	if e.prefix != nil {
		for from <= s.size {
			candidate, _, err := e.prefix.find(s, from)
			if err != nil || candidate < 0 {
				return -1, -1, err
			}
			base, re := candidate, e.anchoredFirst
			if candidate > 0 {
				_, base = s.previous(candidate)
				re = e.anchoredNext
			}
			start, end, err := e.matchReader(s, base, re)
			if err != nil || start >= 0 {
				return start, end, err
			}
			_, width := s.runeAt(candidate)
			if width == 0 {
				return -1, -1, nil
			}
			from = candidate + int64(width)
		}
		return -1, -1, nil
	}
	if e.fold {
		return e.findFolded(s, from)
	}
	if e.literal != nil {
		block := e.scratch
		off := from
		carry := 0
		for off < s.size {
			n, err := s.readAt(block[carry:carry+(64<<10)], off)
			if n > 0 {
				total := carry + n
				if i := bytes.Index(block[:total], e.literal); i >= 0 {
					start := off - int64(carry) + int64(i)
					return start, start + int64(len(e.literal)), nil
				}
				carry = min(len(e.literal)-1, total)
				copy(block[:carry], block[total-carry:total])
			}
			off += int64(n)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return -1, -1, err
			}
			if n == 0 {
				break
			}
		}
		return -1, -1, nil
	}
	base := from
	re := e.first
	if from > 0 {
		_, base = s.previous(from)
		re = e.next
	}
	return e.matchReader(s, base, re)
}
func (e *expression) matchReader(s source, base int64, re *regexp.Regexp) (int64, int64, error) {
	var failure error
	e.reader.Reset(contextReader{s.ctx, io.NewSectionReader(s.at, base, s.size-base), s.bytes, &failure})
	indices := re.FindReaderSubmatchIndex(e.reader)
	if failure != nil {
		return -1, -1, failure
	}
	if err := s.ctx.Err(); err != nil {
		return -1, -1, err
	}
	if indices == nil {
		return -1, -1, nil
	}
	return base + int64(indices[2]), base + int64(indices[3]), nil
}

func (e *expression) findFolded(s source, from int64) (int64, int64, error) {
	off := from
	carry := 0
	for off < s.size {
		n, err := s.readAt(e.raw, off)
		if err != nil && !errors.Is(err, io.EOF) {
			return -1, -1, err
		}
		if n == 0 {
			break
		}
		valid := n
		if off+int64(n) < s.size {
			start := n - 1
			for start > 0 && !utf8.RuneStart(e.raw[start]) {
				start--
			}
			if !utf8.FullRune(e.raw[start:n]) {
				valid = start
			}
		}
		count := carry
		for i := 0; i < valid; {
			r, w := utf8.DecodeRune(e.raw[i:valid])
			encoded := utf8.AppendRune(e.scratch[count:count], foldRune(r))
			for j := 0; j < len(encoded); j++ {
				e.starts[count+j] = off + int64(i)
				e.ends[count+j] = off + int64(i+w)
			}
			count += len(encoded)
			i += w
		}
		if i := bytes.Index(e.scratch[:count], e.literal); i >= 0 {
			return e.starts[i], e.ends[i+len(e.literal)-1], nil
		}
		carry = min(len(e.literal)-1, count)
		copy(e.scratch[:carry], e.scratch[count-carry:count])
		copy(e.starts[:carry], e.starts[count-carry:count])
		copy(e.ends[:carry], e.ends[count-carry:count])
		off += int64(valid)
		if valid == 0 {
			return -1, -1, io.ErrNoProgress
		}
	}
	return -1, -1, nil
}

type positionTracker struct {
	s            source
	offset       int64
	line, column int
	block        []byte
}

func (p *positionTracker) advance(target int64) (editor.Position, error) {
	if target < p.offset {
		return editor.Position{}, errors.New("search positions moved backwards")
	}
	if p.block == nil {
		p.block = make([]byte, 64<<10)
	}
	block := p.block
	for p.offset < target {
		n, err := p.s.readAt(block[:min(int64(len(block)), target-p.offset)], p.offset)
		if n > 0 {
			data := block[:n]
			// Do not split a UTF-8 rune across tracking blocks.
			if p.offset+int64(n) < target {
				for n > 0 && !utf8.RuneStart(data[n-1]) {
					n--
				}
				if n > 0 && data[n-1] >= utf8.RuneSelf {
					n--
				}
				data = data[:n]
			}
			p.line += bytes.Count(data, []byte{'\n'})
			if i := bytes.LastIndexByte(data, '\n'); i >= 0 {
				p.column = 0
				data = data[i+1:]
			}
			absolute := p.offset + int64(n-len(data))
			for len(data) > 0 {
				r, w := utf8.DecodeRune(data)
				crlf := r == '\r' && ((len(data) > w && data[w] == '\n') || (len(data) == w && func() bool { next, _ := p.s.runeAt(absolute + int64(w)); return next == '\n' }()))
				if !crlf {
					p.column++
					if r > 0xffff {
						p.column++
					}
				}
				data = data[w:]
				absolute += int64(w)
			}
			p.offset += int64(n)
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return editor.Position{}, err
		}
		if n == 0 {
			return editor.Position{}, io.ErrUnexpectedEOF
		}
	}
	return editor.Position{Line: p.line, Character: p.column}, nil
}

func preview(s source, start, end int64) (string, string, string, [32]byte, error) {
	hash := sha256.New()
	var block [64 << 10]byte
	for off := start; off < end; {
		n, err := s.readAt(block[:min(int64(len(block)), end-off)], off)
		if n > 0 {
			hash.Write(block[:n])
			off += int64(n)
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return "", "", "", [32]byte{}, err
		}
		if n == 0 {
			return "", "", "", [32]byte{}, io.ErrUnexpectedEOF
		}
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	lo := max(0, start-96)
	hi := min(s.size, min(end, start+256)+96)
	data := make([]byte, hi-lo)
	n, err := s.readAt(data, lo)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", "", "", digest, err
	}
	data = data[:n]
	left := data[:min(int64(len(data)), start-lo)]
	right := data[min(int64(len(data)), end-lo):]
	if i := bytes.LastIndexByte(left, '\n'); i >= 0 {
		left = left[i+1:]
	}
	for len(left) > 0 && !utf8.RuneStart(left[0]) {
		left = left[1:]
	}
	if i := bytes.IndexByte(right, '\n'); i >= 0 {
		right = right[:i]
	}
	middle := data[start-lo : min(int64(len(data)), min(end, start+256)-lo)]
	clean := func(b []byte) string {
		return strings.ReplaceAll(strings.ReplaceAll(strings.ToValidUTF8(string(b), "�"), "\r", ""), "\n", "↵")
	}
	text := clean(middle)
	if end-start > 256 {
		text += "…"
	}
	return clean(left), text, clean(right), digest, nil
}

func validRelative(p string) bool {
	return p != "" && p != "." && !strings.HasPrefix(p, "/") && !strings.Contains(p, "\\") && filepath.IsLocal(filepath.FromSlash(p))
}

// Verify checks bytes and protocol coordinates before navigating a possibly
// stale result. Call on a worker, using a frozen editor snapshot or owned file.
func Verify(ctx context.Context, at io.ReaderAt, size int64, m Match) error {
	if m.Start < 0 || m.End < m.Start || m.End > size {
		return errors.New("search result is stale")
	}
	var count int64
	s := source{at, size, ctx, &count}
	p := positionTracker{s: s}
	start, err := p.advance(m.Start)
	if err != nil {
		return err
	}
	end, err := p.advance(m.End)
	if err != nil {
		return err
	}
	if (editor.Range{Start: start, End: end}) != m.Range {
		return errors.New("search result coordinates changed")
	}
	hash := sha256.New()
	_, err = io.CopyBuffer(hash, contextReader{ctx, io.NewSectionReader(at, m.Start, m.End-m.Start), &count, nil}, make([]byte, 64<<10))
	if err != nil {
		return err
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	if digest != m.Hash {
		return errors.New("search result text changed")
	}
	return nil
}
