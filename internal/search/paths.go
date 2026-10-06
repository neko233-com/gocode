package search

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

type pathRule struct {
	base                string
	pattern             *regexp.Regexp
	directory, negative bool
}

// Git-style slash/basename anchoring and ** use bounded compiled expressions.
// Rules are applied from parent to child, last match wins; excluded directories
// are pruned, so child negation cannot re-include a parent's ignored directory.
func globExpression(pattern string, basename bool) (*regexp.Regexp, error) {
	if len(pattern) > MaxQueryBytes {
		return nil, errors.New("glob exceeds 4096 bytes")
	}
	var b strings.Builder
	if basename {
		b.WriteString("(?:^|/)")
	} else {
		b.WriteByte('^')
	}
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '\\':
			i++
			if i >= len(pattern) {
				return nil, errors.New("glob ends in an escape")
			}
			b.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' && (i == 0 || pattern[i-1] == '/') && (i+2 == len(pattern) || pattern[i+2] == '/') {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					b.WriteString("(?:.*/)?")
					i++
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '[':
			end := i + 1
			if end < len(pattern) && (pattern[end] == '!' || pattern[end] == '^') {
				end++
			}
			if end < len(pattern) && pattern[end] == ']' {
				end++
			}
			for end < len(pattern) && pattern[end] != ']' {
				end++
			}
			if end == len(pattern) {
				return nil, errors.New("unclosed glob character class")
			}
			class := pattern[i : end+1]
			if len(class) > 2 && class[1] == '!' {
				class = "[^" + class[2:]
			}
			b.WriteString(class)
			i = end
		default:
			_, width := utf8.DecodeRuneInString(pattern[i:])
			b.WriteString(regexp.QuoteMeta(pattern[i : i+width]))
			i += width - 1
		}
	}
	b.WriteByte('$')
	return regexp.Compile(b.String())
}

func compileFilters(value string) ([]*regexp.Regexp, error) {
	if len(value) > MaxQueryBytes {
		return nil, errors.New("file filters exceed 4096 bytes")
	}
	var patterns []string
	start, depth := 0, 0
	for i, c := range value {
		if c == '{' {
			depth++
		}
		if c == '}' {
			depth--
			if depth < 0 {
				return nil, errors.New("unbalanced file-filter braces")
			}
		}
		if c == ',' && depth == 0 {
			patterns = append(patterns, strings.TrimSpace(value[start:i]))
			start = i + 1
		}
	}
	if depth != 0 {
		return nil, errors.New("unbalanced file-filter braces")
	}
	patterns = append(patterns, strings.TrimSpace(value[start:]))
	var expanded []string
	var expand func(string) error
	expand = func(p string) error {
		if len(expanded) >= 256 {
			return errors.New("file filters expand beyond 256 patterns")
		}
		begin := strings.IndexByte(p, '{')
		if begin < 0 {
			if p != "" {
				expanded = append(expanded, p)
			}
			return nil
		}
		level, end := 1, begin+1
		for end < len(p) && level > 0 {
			if p[end] == '{' {
				level++
			}
			if p[end] == '}' {
				level--
			}
			end++
		}
		if level != 0 {
			return errors.New("unclosed file-filter braces")
		}
		parts, partStart, level := []string{}, begin+1, 0
		for i := begin + 1; i < end-1; i++ {
			if p[i] == '{' {
				level++
			}
			if p[i] == '}' {
				level--
			}
			if p[i] == ',' && level == 0 {
				parts = append(parts, p[partStart:i])
				partStart = i + 1
			}
		}
		parts = append(parts, p[partStart:end-1])
		for _, part := range parts {
			if err := expand(p[:begin] + part + p[end:]); err != nil {
				return err
			}
		}
		return nil
	}
	for _, p := range patterns {
		if err := expand(p); err != nil {
			return nil, err
		}
	}
	var result []*regexp.Regexp
	for _, p := range expanded {
		p = strings.TrimPrefix(strings.ReplaceAll(p, "\\", "/"), "./")
		p = strings.TrimPrefix(p, "/")
		if strings.HasSuffix(p, "/") {
			p += "**"
		}
		r, err := globExpression(p, !strings.Contains(p, "/"))
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}
func acceptFilter(filters []*regexp.Regexp, p string, empty bool) bool {
	if len(filters) == 0 {
		return empty
	}
	for _, r := range filters {
		if r.MatchString(p) {
			return true
		}
	}
	return false
}

func readRules(fs *os.Root, base, file string) ([]pathRule, error) {
	f, err := fs.Open(filepath.Join(filepath.FromSlash(base), file))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return nil, fmt.Errorf("%s exceeds the 64 KiB ignore-file limit", file)
	}
	scanner := bufio.NewScanner(io.LimitReader(f, (64<<10)+1))
	scanner.Buffer(make([]byte, 4096), 64<<10)
	var result []pathRule
	for scanner.Scan() {
		p := strings.TrimSuffix(scanner.Text(), "\r")
		for strings.HasSuffix(p, " ") && !strings.HasSuffix(p, "\\ ") {
			p = strings.TrimSuffix(p, " ")
		}
		if p == "" || p[0] == '#' {
			continue
		}
		negative := p[0] == '!'
		if negative {
			p = p[1:]
		}
		if p == "" {
			continue
		}
		dir := strings.HasSuffix(p, "/")
		p = strings.TrimSuffix(p, "/")
		anchored := strings.HasPrefix(p, "/")
		p = strings.TrimPrefix(p, "/")
		r, err := globExpression(p, !anchored && !strings.Contains(p, "/"))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		result = append(result, pathRule{base, r, dir, negative})
		if len(result) > 2048 {
			return nil, errors.New("ignore file exceeds 2048 rules")
		}
	}
	return result, scanner.Err()
}
func ignored(rules []pathRule, p string, dir bool) bool {
	return applyRules(rules, p, dir, false)
}
func applyRules(rules []pathRule, p string, dir, value bool) bool {
	for _, r := range rules {
		relative := p
		if r.base != "" {
			if !strings.HasPrefix(p, r.base+"/") {
				continue
			}
			relative = strings.TrimPrefix(p, r.base+"/")
		}
		if (!r.directory || dir) && r.pattern.MatchString(relative) {
			value = !r.negative
		}
	}
	return value
}

type ignoreGroup struct {
	parent *ignoreGroup
	rules  []pathRule
}

func (g *ignoreGroup) ignored(p string, dir bool) bool {
	if g == nil {
		return false
	}
	return applyRules(g.rules, p, dir, g.parent.ignored(p, dir))
}

func collect(ctx context.Context, fs *os.Root, useIgnore bool) (paths []string, limited bool, unreadable int, failure error) {
	type directory struct {
		path  string
		depth int
		rules *ignoreGroup
	}
	var initial []pathRule
	if useIgnore {
		var err error
		initial, err = readRules(fs, "", ".git/info/exclude")
		if err != nil {
			return nil, false, 0, err
		}
	}
	queue := []directory{{rules: &ignoreGroup{rules: initial}}}
	entries, dirs, ruleCount := 0, 0, len(initial)
	pathBytes := 0
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return paths, limited, unreadable, err
		}
		dir := queue[len(queue)-1]
		queue[len(queue)-1] = directory{}
		queue = queue[:len(queue)-1]
		dirs++
		if dirs > MaxDirectories {
			return paths, true, unreadable, nil
		}
		if useIgnore {
			rules, err := readRules(fs, dir.path, ".gitignore")
			if err != nil {
				return paths, limited, unreadable, err
			}
			ruleCount += len(rules)
			if ruleCount > 8192 {
				return paths, true, unreadable, errors.New("search exceeds 8192 compiled ignore rules")
			}
			if len(rules) > 0 {
				dir.rules = &ignoreGroup{parent: dir.rules, rules: rules}
			}
		}
		p := filepath.FromSlash(dir.path)
		if p == "" {
			p = "."
		}
		f, err := fs.Open(p)
		if err != nil {
			if dir.depth == 0 {
				return nil, limited, unreadable, err
			}
			unreadable++
			continue
		}
		stop := false
		for !stop {
			if err := ctx.Err(); err != nil {
				f.Close()
				return paths, limited, unreadable, err
			}
			batch, err := f.ReadDir(128)
			for _, entry := range batch {
				entries++
				if entries > MaxEntries {
					f.Close()
					return paths, true, unreadable, nil
				}
				if entry.Type()&os.ModeSymlink != 0 || entry.Name() == ".git" {
					continue
				}
				rel := entry.Name()
				if dir.path != "" {
					rel = dir.path + "/" + rel
				}
				if useIgnore && dir.rules.ignored(rel, entry.IsDir()) {
					continue
				}
				if entry.IsDir() {
					if dir.depth >= 64 || len(queue) >= 1024 {
						limited = true
						continue
					}
					queue = append(queue, directory{rel, dir.depth + 1, dir.rules})
					continue
				}
				if entry.Type()&os.ModeType != 0 {
					continue
				}
				paths = append(paths, rel)
				pathBytes += len(rel)
				if pathBytes > 16<<20 {
					f.Close()
					return paths, true, unreadable, nil
				}
				if len(paths) >= MaxFiles {
					f.Close()
					return paths, true, unreadable, nil
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					unreadable++
				}
				stop = true
			}
		}
		f.Close()
	}
	return
}
