package main

import (
	"strings"
	"time"

	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

type lspAcceptance struct {
	phase          int
	frame          uint64
	tick, verified bool
	failure        string
}

func (a *lspAcceptance) step(cx *ui.Context, m *model) {
	d := m.current()
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
		a.verified = true
		cx.Quit()
		return
	}
	if !a.tick {
		a.tick = true
		time.AfterFunc(50*time.Millisecond, func() { cx.Dispatch(func() { a.tick = false }) })
	}
}
