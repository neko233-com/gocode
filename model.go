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

	"github.com/neko233-com/gocode/internal/copilotservice"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

type document struct {
	path                         string
	buffer                       *textbuffer.Buffer
	large                        *largeDocument
	serviceBytes, serviceVersion int
	line, column, scroll         int
}
type model struct {
	workspace                             string
	files                                 []string
	docs                                  []*document
	active                                int
	activity, panel                       string
	showPanel, palette, editing           bool
	query, message, status                string
	output                                []string
	commands                              []extensionCommand
	installed                             []extensionInfo
	execute                               func(string)
	pointerX                              float32
	smoke                                 bool
	pointerShift                          bool
	readClipboard                         func() (string, error)
	writeClipboard                        func(string) error
	onDocument                            func(string, *document, textbuffer.ChangeEvent)
	requestInline                         func(*document)
	cancelInline                          func()
	acceptInline                          func(copilotservice.InlineItem)
	acceptedInline                        uint64
	suggestion                            *inlineSuggestion
	askChat                               func(string)
	cancelChat                            func()
	signInCopilot                         func()
	signInChat                            func()
	chatLoginBusy                         bool
	chatPrompt, chatAnswer, copilotStatus string
	chatContext, chatContextLabel         string
	chatFocused, chatBusy                 bool
	chatGeneration                        uint64
	inlineGeneration                      uint64
	requestCompletions                    func(*document)
	completions                           []completionSuggestion
	pointerSelecting                      bool
	diagnostics                           map[string][]diagnostic
	native                                *ui.Context
	navigation, largeScrollbar            bool
	requestLSP                            func(*document, string)
	lspStatus                             string
	completionSources                     map[string][]completionSuggestion
}

type diagnostic struct {
	Range    textbuffer.Range `json:"range"`
	Message  string           `json:"message"`
	Severity int              `json:"severity"`
	Path     string           `json:"-"`
}

type inlineSuggestion struct {
	path     string
	version  int
	position textbuffer.Position
	item     copilotservice.InlineItem
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
			m.documentEvent("focus", d, textbuffer.ChangeEvent{})
			return
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		m.message = err.Error()
		return
	}
	if !info.Mode().IsRegular() {
		m.message = "Only regular UTF-8 text files can be opened"
		return
	}
	if info.Size() > editableFileLimit {
		d, err := openLargeDocument(path)
		if err != nil {
			m.message = err.Error()
			return
		}
		m.docs = append(m.docs, d)
		m.active = len(m.docs) - 1
		m.editing = true
		if m.native != nil {
			m.startLargeDocument(d)
		}
		m.documentEvent("focus", d, textbuffer.ChangeEvent{})
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
	buffer, err := textbuffer.New(string(data))
	if err != nil {
		m.message = err.Error()
		return
	}
	m.docs = append(m.docs, &document{path: path, buffer: buffer})
	m.active = len(m.docs) - 1
	m.editing = true
	m.documentEvent("open", m.current(), textbuffer.ChangeEvent{})
	m.documentEvent("focus", m.current(), textbuffer.ChangeEvent{})
}
func (m *model) closeTab(index int) {
	if index < 0 || index >= len(m.docs) {
		return
	}
	if m.docs[index].dirty() {
		m.message = "Save this file before closing its tab (Ctrl/Cmd+S)"
		return
	}
	m.documentEvent("close", m.docs[index], textbuffer.ChangeEvent{})
	if l := m.docs[index].large; l != nil {
		l.cancel()
		go l.close()
	}
	m.docs = append(m.docs[:index], m.docs[index+1:]...)
	if index < m.active {
		m.active--
	}
	m.active = min(m.active, len(m.docs)-1)
	m.documentEvent("focus", m.current(), textbuffer.ChangeEvent{})
}
func (m *model) save() error {
	d := m.current()
	if d == nil {
		return errors.New("no active document")
	}
	return m.saveDocument(d)
}
func (m *model) saveDocument(d *document) error {
	if d.buffer == nil {
		return errors.New("large-file browsing is read-only; the file on disk has not been changed")
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
	_, writeErr := f.WriteString(d.buffer.Text())
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
	d.buffer.MarkSaved()
	m.documentEvent("save", d, textbuffer.ChangeEvent{})
	m.message = "Saved " + filepath.Base(d.path)
	return nil
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
