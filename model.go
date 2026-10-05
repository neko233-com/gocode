package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	ui "github.com/neko233-com/godesktop"
)

type document struct {
	path                 string
	lines                []string
	dirty                bool
	line, column, scroll int
}
type model struct {
	workspace                   string
	files                       []string
	docs                        []*document
	active                      int
	activity, panel             string
	showPanel, palette, editing bool
	query, message, status      string
	output                      []string
	commands                    []extensionCommand
	installed                   []extensionInfo
	execute                     func(string)
	pointerX                    float32
	smoke                       bool
}
type extensionCommand struct{ ID, Title string }
type extensionInfo struct{ Name, ID, Description, Version string }

func newModel(workspace string) (*model, error) {
	absolute, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("workspace must be a directory: %s", absolute)
	}
	m := &model{workspace: absolute, active: -1, activity: "files", panel: "TERMINAL", showPanel: true, output: []string{"gocode", "Workspace opened."}}
	err = filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == absolute {
			return nil
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".cache", "node_modules", "bin", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, _ := filepath.Rel(absolute, path)
		m.files = append(m.files, filepath.ToSlash(rel))
		if len(m.files) >= 250 {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	preferred := ""
	for _, p := range m.files {
		if strings.HasSuffix(p, "main.go") {
			preferred = p
			break
		}
		if preferred == "" && strings.HasSuffix(p, ".go") {
			preferred = p
		}
	}
	if preferred == "" && len(m.files) > 0 {
		preferred = m.files[0]
	}
	if preferred != "" {
		m.open(filepath.Join(absolute, preferred))
	}
	return m, nil
}
func (m *model) current() *document {
	if m.active < 0 || m.active >= len(m.docs) {
		return nil
	}
	return m.docs[m.active]
}
func (m *model) open(path string) {
	path, err := filepath.Abs(path)
	if err != nil {
		m.message = err.Error()
		return
	}
	for i, d := range m.docs {
		if d.path == path {
			m.active = i
			return
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		m.message = err.Error()
		return
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		m.message = "Only UTF-8 text files up to 1 MiB can be opened"
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		m.message = err.Error()
		return
	}
	if !utf8.Valid(data) || strings.ContainsRune(string(data), 0) {
		m.message = "Binary files are not supported"
		return
	}
	m.docs = append(m.docs, &document{path: path, lines: strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")})
	m.active = len(m.docs) - 1
	m.editing = true
}
func (m *model) closeTab(index int) {
	if m.docs[index].dirty {
		m.message = "Save this file before closing its tab (Ctrl/Cmd+S)"
		return
	}
	m.docs = append(m.docs[:index], m.docs[index+1:]...)
	if index < m.active {
		m.active--
	}
	m.active = min(m.active, len(m.docs)-1)
}
func (m *model) save() error {
	d := m.current()
	if d == nil {
		return errors.New("no active document")
	}
	info, err := os.Stat(d.path)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(d.path), ".gocode-save-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	_, writeErr := f.WriteString(strings.Join(d.lines, "\n"))
	syncErr := f.Sync()
	closeErr := f.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	if err = os.Chmod(name, info.Mode().Perm()); err != nil {
		return err
	}
	if err = os.Rename(name, d.path); err != nil {
		return err
	}
	d.dirty = false
	m.message = "Saved " + filepath.Base(d.path)
	return nil
}
func (m *model) input(_ *ui.Context, e ui.InputEvent) bool {
	if e.Kind == ui.PointerPressed {
		m.pointerX = e.X
		m.editing = false
		return false
	}
	if e.Kind == ui.Scroll {
		if d := m.current(); d != nil {
			d.scroll = max(0, min(len(d.lines)-1, d.scroll-int(e.Y)))
		}
		return true
	}
	if e.Kind == ui.KeyPressed && e.Modifiers&(ui.ModifierControl|ui.ModifierCommand) != 0 {
		switch e.Key {
		case 'S':
			if err := m.save(); err != nil {
				m.message = err.Error()
			}
			return true
		case 'P':
			m.palette = !m.palette
			m.query = ""
			return true
		case 'J':
			m.showPanel = !m.showPanel
			return true
		}
	}
	if m.palette || m.activity == "search" {
		if e.Kind == ui.Character {
			m.query += string(rune(e.Key))
			return true
		}
		if e.Kind == ui.KeyPressed {
			switch e.Key {
			case 27:
				m.palette = false
				m.query = ""
				return true
			case 8:
				r := []rune(m.query)
				if len(r) > 0 {
					m.query = string(r[:len(r)-1])
				}
				return true
			}
		}
	}
	d := m.current()
	if !m.editing || d == nil {
		return false
	}
	r := []rune(d.lines[d.line])
	d.column = min(d.column, len(r))
	if e.Kind == ui.Character {
		if e.Key < 32 || !utf8.ValidRune(rune(e.Key)) {
			return true
		}
		r = append(r[:d.column], append([]rune{rune(e.Key)}, r[d.column:]...)...)
		d.lines[d.line] = string(r)
		d.column++
		d.dirty = true
		return true
	}
	if e.Kind != ui.KeyPressed {
		return false
	}
	switch e.Key {
	case 32:
		// The following character event inserts the space. Consume its key event
		// so generic button activation does not reset the editor's caret.
		return true
	case 37:
		if d.column > 0 {
			d.column--
		} else if d.line > 0 {
			d.line--
			d.column = len([]rune(d.lines[d.line]))
		}
	case 39:
		if d.column < len(r) {
			d.column++
		} else if d.line+1 < len(d.lines) {
			d.line++
			d.column = 0
		}
	case 38:
		d.line = max(0, d.line-1)
	case 40:
		d.line = min(len(d.lines)-1, d.line+1)
	case 36:
		d.column = 0
	case 35:
		d.column = len(r)
	case 8:
		if d.column > 0 {
			d.lines[d.line] = string(append(r[:d.column-1], r[d.column:]...))
			d.column--
			d.dirty = true
		} else if d.line > 0 {
			before := d.lines[d.line-1]
			d.column = len([]rune(before))
			d.lines[d.line-1] = before + d.lines[d.line]
			d.lines = append(d.lines[:d.line], d.lines[d.line+1:]...)
			d.line--
			d.dirty = true
		}
	case 46:
		if d.column < len(r) {
			d.lines[d.line] = string(append(r[:d.column], r[d.column+1:]...))
			d.dirty = true
		} else if d.line+1 < len(d.lines) {
			d.lines[d.line] += d.lines[d.line+1]
			d.lines = append(d.lines[:d.line+1], d.lines[d.line+2:]...)
			d.dirty = true
		}
	case 13:
		d.lines[d.line] = string(r[:d.column])
		after := string(r[d.column:])
		d.lines = append(d.lines[:d.line+1], append([]string{after}, d.lines[d.line+1:]...)...)
		d.line++
		d.column = 0
		d.dirty = true
	case 9:
		d.lines[d.line] = string(r[:d.column]) + "    " + string(r[d.column:])
		d.column += 4
		d.dirty = true
	default:
		return false
	}
	d.column = min(d.column, len([]rune(d.lines[d.line])))
	if d.line < d.scroll {
		d.scroll = d.line
	}
	return true
}

type fragment struct {
	text  string
	color uint32
}

var keywords = map[string]bool{"package": true, "import": true, "func": true, "var": true, "const": true, "type": true, "struct": true, "interface": true, "return": true, "if": true, "else": true, "for": true, "range": true, "go": true, "defer": true, "switch": true, "case": true, "break": true, "map": true, "chan": true, "nil": true, "true": true, "false": true}

func highlight(line string) []fragment {
	r := []rune(strings.ReplaceAll(line, "\t", "    "))
	var result []fragment
	for i := 0; i < len(r); {
		start := i
		color := uint32(0xd4d4d4)
		if r[i] == '/' && i+1 < len(r) && r[i+1] == '/' {
			result = append(result, fragment{string(r[i:]), 0x6a9955})
			break
		}
		if r[i] == '"' || r[i] == '\'' || r[i] == '`' {
			quote := r[i]
			i++
			for i < len(r) {
				if r[i] == '\\' {
					i = min(len(r), i+2)
					continue
				}
				end := r[i] == quote
				i++
				if end {
					break
				}
			}
			color = 0xce9178
		} else if unicode.IsLetter(r[i]) || r[i] == '_' {
			i++
			for i < len(r) && (unicode.IsLetter(r[i]) || unicode.IsDigit(r[i]) || r[i] == '_') {
				i++
			}
			if keywords[string(r[start:i])] {
				color = 0x569cd6
			} else if i < len(r) && r[i] == '(' {
				color = 0xdcdcaa
			} else {
				color = 0x9cdcfe
			}
		} else if unicode.IsDigit(r[i]) {
			i++
			for i < len(r) && (unicode.IsDigit(r[i]) || r[i] == '.') {
				i++
			}
			color = 0xb5cea8
		} else {
			i++
		}
		text := string(r[start:i])
		if len(result) > 0 && result[len(result)-1].color == color {
			result[len(result)-1].text += text
		} else {
			result = append(result, fragment{text, color})
		}
	}
	return result
}
