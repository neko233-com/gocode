package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/neko233-com/gocode/internal/languageserver"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

type lspAcceptance struct {
	phase          int
	frame          uint64
	tick, verified bool
	failure        string
	loggedPhase    int
	logged         bool
	previous       *languageserver.Session
	pixelsLogged   bool
}

func (a *lspAcceptance) step(cx *ui.Context, m *model) {
	d := m.current()
	if !a.logged || a.phase != a.loggedPhase {
		fmt.Fprintf(os.Stderr, "gocode LSP acceptance phase=%d status=%q message=%q open-documents=%d\n", a.phase, m.lspStatus, m.message, len(m.docs))
		a.loggedPhase = a.phase
		a.logged = true
	}
	if d == nil || d.buffer == nil {
		a.failure = "source buffer unavailable"
		cx.Quit()
		return
	}
	find := func(token string, definition bool) (int, int) {
		for i := 0; i < d.buffer.LineCount(); i++ {
			line := d.buffer.Line(i)
			if strings.Contains(line, token) && strings.HasPrefix(line, "func greeting") == definition {
				return i, strings.Index(line, token)
			}
		}
		return -1, 0
	}
	if a.phase == 0 && strings.HasSuffix(m.lspStatus, " ready") && m.requestLSP != nil {
		m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 'F', Modifiers: ui.ModifierShift | ui.ModifierAlt})
		a.phase = 1
	}
	if a.phase == 1 && strings.HasPrefix(m.message, "Formatted with ") {
		line, column := find("greeting", false)
		if line < 0 {
			a.failure = "formatted call missing"
			cx.Quit()
			return
		}
		m.moveCursor(d, line, column+2, false)
		m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 123})
		a.phase = 2
	}
	if a.phase == 2 {
		line, _ := find("greeting", true)
		if d.line == line {
			m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 'K', Modifiers: ui.ModifierControl})
			m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 'I', Modifiers: ui.ModifierControl})
			a.phase = 3
		}
	}
	if a.phase == 3 && strings.Contains(strings.Join(m.output, "\n"), "greeting() string") {
		line, column := find("greeting", false)
		r := textbuffer.Range{Start: d.buffer.PositionFromRunes(line, column), End: d.buffer.PositionFromRunes(line, column+8)}
		if err := m.applyDocumentEdits(d.path, d.buffer.Version(), []textbuffer.Edit{{Range: r, Text: "greet"}}); err != nil {
			a.failure = err.Error()
			cx.Quit()
			return
		}
		m.moveCursor(d, line, column+5, false)
		m.requestLSP(d, "textDocument/completion")
		a.phase = 4
	}
	if a.phase == 4 {
		for _, item := range m.completions {
			if strings.HasPrefix(item.item.title(), "greeting") {
				m.chooseCompletion(item)
				a.phase = 5
				break
			}
		}
	}
	if a.phase == 5 {
		found := false
		for key, items := range m.diagnostics {
			if strings.HasPrefix(key, "lsp:") {
				for _, p := range items {
					if strings.Contains(p.Message, "undefined: missing") {
						found = true
					}
				}
			}
		}
		if found {
			line, _ := find("_ = missing", false)
			if line < 0 {
				a.failure = "completion damaged source"
				cx.Quit()
				return
			}
			r := textbuffer.Range{Start: textbuffer.Position{Line: line}, End: textbuffer.Position{Line: line + 1}}
			if err := m.applyDocumentEdits(d.path, d.buffer.Version(), []textbuffer.Edit{{Range: r}}); err != nil {
				a.failure = err.Error()
				cx.Quit()
				return
			}
			a.phase = 6
		}
	}
	if a.phase == 6 {
		cleared := false
		for key, items := range m.diagnostics {
			if strings.HasPrefix(key, "lsp:") && len(items) == 0 {
				cleared = true
			}
		}
		if cleared {
			m.panel = "OUTPUT"
			a.frame = cx.RenderedFrames()
			a.phase = 7
		}
	}
	if a.phase == 7 && cx.RenderedFrames() > a.frame {
		if !strings.Contains(d.buffer.Text(), "func main()") || !strings.Contains(d.buffer.Text(), "_ = greeting()") || strings.Contains(d.buffer.Text(), "missing") || !d.dirty() {
			a.failure = "versioned unsaved source was not retained"
			cx.Quit()
			return
		}
		if err := captureLSPAcceptance(m.workspace); err != nil {
			a.failure = err.Error()
			cx.Quit()
			return
		}
		if len(m.languageBindings) == 0 || m.languageBindings[0].session == nil {
			a.failure = "language lifecycle binding unavailable"
			cx.Quit()
			return
		}
		a.previous = m.languageBindings[0].session
		// Close kills and reaps this owned real gopls process outside the UI.
		go a.previous.Client.RPC.Close()
		a.phase = 8
	}
	if a.phase == 8 && m.languageBindings[0].session == nil {
		for key := range m.diagnostics {
			if strings.HasPrefix(key, "lsp:"+m.languageBindings[0].config.Name+"\x00") {
				a.failure = "dead server diagnostics retained"
				cx.Quit()
				return
			}
		}
		line, _ := find("_ = greeting()", false)
		if line < 0 {
			a.failure = "source missing during recovery"
			cx.Quit()
			return
		}
		position := textbuffer.Position{Line: line}
		if err := m.applyDocumentEdits(d.path, d.buffer.Version(), []textbuffer.Edit{{Range: textbuffer.Range{Start: position, End: position}, Text: "\t_ = missingRestart\n"}}); err != nil {
			a.failure = err.Error()
			cx.Quit()
			return
		}
		a.phase = 9
	}
	if a.phase == 9 && m.languageBindings[0].session != nil && m.languageBindings[0].session != a.previous {
		for key, items := range m.diagnostics {
			if strings.HasPrefix(key, "lsp:") {
				for _, item := range items {
					if strings.Contains(item.Message, "undefined: missingRestart") {
						line, column := find("greeting", true)
						m.moveCursor(d, line, column+2, false)
						m.output = nil
						m.requestLSP(d, "textDocument/hover")
						a.phase = 10
					}
				}
			}
		}
	}
	if a.phase == 10 && strings.Contains(strings.Join(m.output, "\n"), "greeting() string") {
		line, column := find("greeting", false)
		r := textbuffer.Range{Start: d.buffer.PositionFromRunes(line, column), End: d.buffer.PositionFromRunes(line, column+8)}
		if err := m.applyDocumentEdits(d.path, d.buffer.Version(), []textbuffer.Edit{{Range: r, Text: "greet"}}); err != nil {
			a.failure = err.Error()
			cx.Quit()
			return
		}
		m.moveCursor(d, line, column+5, false)
		m.requestLSP(d, "textDocument/completion")
		a.phase = 11
	}
	if a.phase == 11 {
		for _, item := range m.completions {
			if strings.HasPrefix(item.item.title(), "greeting") {
				m.chooseCompletion(item)
				m.panel = "PROBLEMS"
				m.showPanel = true
				a.frame = cx.RenderedFrames()
				a.phase = 12
				break
			}
		}
	}
	if a.phase == 12 && cx.RenderedFrames() >= a.frame+3 {
		if !strings.Contains(d.buffer.Text(), "_ = greeting()") || !strings.Contains(d.buffer.Text(), "missingRestart") || !d.dirty() || a.previous.Valid() {
			a.failure = "recovered unsaved document or old generation invalidation failed"
			cx.Quit()
			return
		}
		line, _ := find("greeting", false)
		suffix, _ := cx.ElementBounds(fmt.Sprintf("code-line-%d", line))
		prefix := float32(68) // Native row's existing 52-DIP gutter + 16-DIP gap.
		for _, f := range highlight(d.buffer.Line(line)) {
			if f.text == "greeting" {
				advance := ui.TextAdvance("greet", 14, codeFont())
				suffix.X += prefix + advance
				suffix.Width = ui.TextAdvance("greeting", 14, codeFont()) - advance
				break
			}
			width, _ := ui.MeasureText(f.text, 14, codeFont())
			prefix += width
		}
		problem, _ := cx.ElementBounds("problem-0")
		if m.panel == "PROBLEMS" && suffix.Height > 0 && problem.Height > 0 {
			if err := captureRecoveredLSPAcceptance(m.workspace, suffix, problem); err != nil {
				if !errors.Is(err, errLSPPixelsPending) {
					a.failure = err.Error()
					cx.Quit()
					return
				}
				if !a.pixelsLogged {
					fmt.Fprintln(os.Stderr, "native LSP final pixels pending:", err)
					a.pixelsLogged = true
				}
			} else {
				a.verified = true
				cx.Quit()
				return
			}
		}
	}
	if !a.tick {
		a.tick = true
		time.AfterFunc(50*time.Millisecond, func() { cx.Dispatch(func() { a.tick = false }) })
	}
}
