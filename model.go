package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/neko233-com/gocode/internal/copilotservice"
	"github.com/neko233-com/gocode/internal/update"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

type document struct {
	path                         string
	buffer                       *textbuffer.Buffer
	large                        *largeDocument
	serviceBytes, serviceVersion int
	line, column, scroll         int
	diskHash                     [32]byte
	diskKnown                    bool
	saveID                       uint64
	watchID, reloadID            uint64
	diskConflict                 *diskConflict
	tabID                        uint64
	tabWidth                     float32
}
type model struct {
	logo                                               *ui.Bitmap
	workspace                                          string
	files                                              []string
	docs                                               []*document
	active                                             int
	tabs                                               editorTabState
	search                                             searchState
	activity, panel                                    string
	showPanel, palette, editing                        bool
	query, message, status                             string
	output                                             []string
	commands                                           []extensionCommand
	installed                                          []extensionInfo
	execute                                            func(string)
	awaitExtensions                                    func(context.Context) error
	pointerX                                           float32
	smoke                                              bool
	pointerShift                                       bool
	readClipboard                                      func() (string, error)
	writeClipboard                                     func(string) error
	onDocument                                         func(string, *document, textbuffer.ChangeEvent)
	requestInline                                      func(*document)
	cancelInline                                       func()
	acceptInline                                       func(copilotservice.InlineItem)
	acceptedInline                                     uint64
	suggestion                                         *inlineSuggestion
	askChat                                            func(string)
	cancelChat                                         func()
	signInCopilot                                      func()
	signInChat                                         func()
	chatLoginBusy                                      bool
	chatPrompt, chatAnswer, copilotStatus              string
	chatContext, chatContextLabel                      string
	chatFocused, chatBusy                              bool
	chatGeneration                                     uint64
	inlineGeneration                                   uint64
	requestCompletions                                 func(*document)
	completions                                        []completionSuggestion
	pointerSelecting                                   bool
	diagnostics                                        map[string][]diagnostic
	native                                             *ui.Context
	navigation, largeScrollbar                         bool
	requestLSP                                         func(*document, string)
	lspStatus                                          string
	languageBindings                                   []*languageBinding
	restartLanguages                                   func()
	completionSources                                  map[string][]completionSuggestion
	updatesConfig                                      update.Config
	configureUpdates                                   func(update.Config)
	requestUpdate                                      func()
	updateStatus, updateRoute, updateMirrorDraft       string
	updateBusy, updateFocused                          bool
	closePrompt, closeBusy                             bool
	closeEditing, closeChatFocused, closeUpdateFocused bool
	closeTarget                                        *document
	closeError                                         string
	saveForClose                                       func()
	discardForClose                                    func()
	requestSave                                        func(context.Context, []*document, func(error))
	saveJobs                                           []saveJob
	saveBusy                                           bool
	terminals                                          []*terminalTab
	activeTerminal                                     int
	terminalFocused, terminalSelecting, panelResizing  bool
	terminalHeight, panelDragY, panelDragHeight        float32
	newTerminal                                        func()
	killTerminal                                       func(*terminalTab)
	closeTerminalFocused                               bool
	terminalAcceptance                                 bool
	publishWatches                                     func()
	watchSequence, reloadSequence                      uint64
	reloadPrompt                                       *document
	reloadBusy                                         bool
	reloadError                                        string
	pathAliases                                        map[string]*document
	requestOpen                                        func(context.Context, string, func(*document, error))
	openJobs                                           []openJob
	openBusy                                           bool
	openingPath                                        string
	openSequence                                       uint64
	cancelCurrentOpen, cancelPendingOpens              func()
	closedDuringOpen                                   map[string]uint64
	workspaceBusy                                      bool
	workspaceStatus                                    string
	cancelWorkspace                                    func()
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

// Startup validates the root before launching services. Recursive traversal and
// document contents are loaded only after the native window has started.
func newWorkspaceModel(workspace string) (*model, error) {
	absolute, err := canonicalPath(workspace)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("workspace must be a directory: %s", absolute)
	}
	return &model{workspace: absolute, active: -1, activity: "files", panel: "TERMINAL", showPanel: true, output: []string{"gocode", "Workspace opened."}}, nil
}

// Synchronous fixture construction is kept for headless model/protocol tests.
func newModel(workspace string) (*model, error) {
	m, err := newWorkspaceModel(workspace)
	if err != nil {
		return nil, err
	}
	result := scanWorkspace(context.Background(), m.workspace)
	if result.err != nil {
		return nil, result.err
	}
	m.files, m.workspaceStatus = result.files, result.status()
	if preferred := preferredFile(m.files); preferred != "" {
		m.open(preferred)
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
	m.openThen(context.Background(), path, nil)
}
func (m *model) closeTab(index int) {
	if index < 0 || index >= len(m.docs) {
		return
	}
	if m.docs[index].dirty() {
		m.beginClose(m.docs[index])
		return
	}
	m.removeTab(index)
}
func (m *model) removeTab(index int) {
	if index < 0 || index >= len(m.docs) {
		return
	}
	if m.openBusy || len(m.openJobs) > 0 {
		if m.closedDuringOpen == nil {
			m.closedDuringOpen = map[string]uint64{}
		}
		m.openSequence++
		m.closedDuringOpen[pathKey(m.docs[index].path)] = m.openSequence
	}
	for alias, d := range m.pathAliases {
		if d == m.docs[index] {
			delete(m.pathAliases, alias)
		}
	}
	m.documentEvent("close", m.docs[index], textbuffer.ChangeEvent{})
	if l := m.docs[index].large; l != nil {
		l.cancel()
		go l.close()
	}
	copy(m.docs[index:], m.docs[index+1:])
	m.docs[len(m.docs)-1] = nil
	m.docs = m.docs[:len(m.docs)-1]
	if index < m.active {
		m.active--
	}
	m.active = min(m.active, len(m.docs)-1)
	m.documentEvent("focus", m.current(), textbuffer.ChangeEvent{})
}

// Existing files use one physical identity. On Windows EvalSymlinks also
// resolves 8.3 aliases and restores filesystem casing, preserving live buffers
// when a language server returns a canonical definition URI.
func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if physical, err := filepath.EvalSymlinks(abs); err == nil {
		return physical, nil
	}
	return abs, nil
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
