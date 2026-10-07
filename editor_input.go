package main

import (
	"errors"
	"fmt"
	"unicode"
	"unicode/utf8"

	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

func (m *model) documentEvent(kind string, d *document, change textbuffer.ChangeEvent) {
	m.groupDocumentEvent(kind, d, change)
	m.recordTabEvent(kind, d)
	if kind == "change" && m.search.query.Text != "" {
		m.searchChanged(m.native)
	}
	if kind == "focus" {
		m.extensionsView.detail = ""
		m.dismissSCMDiff()
		m.openSequence++
	}
	if m.publishWatches != nil && kind != "selection" {
		m.publishWatches()
	}
	if kind == "change" || kind == "selection" || kind == "focus" || kind == "close" {
		if m.cancelInline != nil {
			m.cancelInline()
		}
		m.suggestion = nil
		m.completions = nil
		m.completionSources = nil
		m.inlineGeneration++
	}
	if m.onDocument != nil {
		m.onDocument(kind, d, change)
	}
	if m.publishEditors != nil && kind != "close" {
		selectionKind := 0
		if kind == "selection" {
			selectionKind = m.editorSelectionKind
		}
		m.publishEditors(selectionKind)
	}
	if kind == "change" && m.requestInline != nil && d != nil {
		m.requestInline(d)
	}
}

func (d *document) cursor() textbuffer.Position {
	if d.buffer == nil {
		return textbuffer.Position{Line: d.line, Character: d.column}
	}
	return d.buffer.PositionFromRunes(d.line, d.column)
}
func (d *document) followSelection() {
	d.holdScroll = false
	p := d.buffer.Selection().Active
	d.line = p.Line
	d.column, _ = d.buffer.RuneColumn(p)
	if d.line < d.scroll {
		d.scroll = d.line
	}
}
func (m *model) moveCursor(d *document, line, column int, extend bool) {
	if d.buffer == nil {
		m.navigateLarge(d, int64(line), 0, false)
		return
	}
	p := d.buffer.PositionFromRunes(line, column)
	s := d.buffer.Selection()
	s.Active = p
	if !extend {
		s.Anchor = p
	}
	_ = d.buffer.SetSelection(s)
	d.followSelection()
	m.documentEvent("selection", d, textbuffer.ChangeEvent{})
}
func (m *model) changed(d *document, change textbuffer.ChangeEvent, err error) {
	if err != nil {
		m.message = err.Error()
		return
	}
	d.followSelection()
	if len(change.Changes) > 0 {
		m.documentEvent("change", d, change)
	}
}
func (m *model) replaceSelection(d *document, text string) {
	change, err := d.buffer.ReplaceSelection(text)
	m.changed(d, change, err)
}
func (m *model) applyDocumentEdits(path string, version int, edits []textbuffer.Edit) error {
	for _, d := range m.docs {
		if d.path != path {
			continue
		}
		if d.buffer == nil || d.buffer.Version() != version {
			return errors.New("document changed before the edit was applied")
		}
		change, err := d.buffer.Apply(edits, nil)
		if err != nil {
			return err
		}
		m.changed(d, change, nil)
		return nil
	}
	return errors.New("document is not open")
}

func (m *model) input(cx *ui.Context, e ui.InputEvent) bool {
	if e.Kind == ui.KeyPressed {
		m.menu.suppressCharacter = 0
	}
	if e.Kind == ui.Character && m.menu.suppressCharacter != 0 {
		key := m.menu.suppressCharacter
		m.menu.suppressCharacter = 0
		if int(unicode.ToUpper(rune(e.Key))) == key {
			return true
		}
	}
	if !m.closePrompt && m.reloadPrompt == nil && m.history.prompt == nil {
		if handled, consumed := m.menuInput(cx, e); handled {
			return consumed
		}
		if handled, consumed := m.quickInput(e); handled {
			return consumed
		}
		if m.workbenchShortcut(cx, e) {
			return true
		}
		if m.extensionInput(cx, e) {
			return true
		}
	}
	if e.Kind == ui.PointerPressed || e.Kind == ui.PointerMoved || e.Kind == ui.PointerReleased {
		m.editorSelectionKind = 2
	} else if e.Kind == ui.KeyPressed || e.Kind == ui.Character {
		m.editorSelectionKind = 1
	}
	if e.Kind == ui.InputCancelled {
		m.endTabSwitch()
		m.activeTabs().dragging = false
	}
	if e.Kind == ui.KeyReleased && e.Key == 17 {
		m.endTabSwitch()
		return true
	}
	if m.reloadPrompt != nil && !m.closePrompt {
		if e.Kind == ui.KeyPressed && !m.reloadBusy && e.Key == 27 {
			m.cancelReload()
		}
		return e.Kind != ui.PointerPressed && e.Kind != ui.PointerReleased && e.Kind != ui.PointerMoved && e.Kind != ui.InputCancelled
	}
	if m.closePrompt {
		if e.Kind == ui.KeyPressed && !m.closeBusy {
			if e.Key == 27 {
				m.cancelClose()
			}
			if e.Key == 13 && m.saveForClose != nil {
				m.saveForClose()
			}
		}
		return e.Kind != ui.PointerPressed && e.Kind != ui.PointerReleased && e.Kind != ui.PointerMoved && e.Kind != ui.InputCancelled
	}
	if m.history.prompt != nil {
		if e.Kind == ui.KeyPressed {
			if e.Key == 27 {
				m.history.prompt = nil
			}
			if e.Key == 13 {
				m.confirmHistory(true)
			}
		}
		return e.Kind != ui.PointerPressed && e.Kind != ui.PointerReleased && e.Kind != ui.PointerMoved && e.Kind != ui.InputCancelled
	}
	if m.scmInput(cx, e) {
		return true
	}
	if m.groupInput(cx, e) {
		return true
	}
	if m.tabInput(cx, e) {
		return true
	}
	if m.searchInput(cx, e) {
		return true
	}
	if m.terminalPointer(cx, e) {
		return true
	}
	if e.Kind == ui.PointerPressed {
		m.terminalFocused = false
	}
	if e.Kind == ui.Scroll && cx != nil {
		b, ok := cx.ElementBounds(m.editorKey("editor-content"))
		if !ok || e.PointerX < b.X || e.PointerX >= b.X+b.Width || e.PointerY < b.Y || e.PointerY >= b.Y+b.Height {
			return false
		}
	}
	if d := m.current(); d != nil && d.large != nil && (e.Kind == ui.PointerPressed || e.Kind == ui.PointerMoved || e.Kind == ui.PointerReleased || e.Kind == ui.InputCancelled || e.Kind == ui.Scroll) {
		return m.largeInput(cx, d, e)
	}
	if e.Kind == ui.InputCancelled {
		m.pointerSelecting = false
		return false
	}
	if e.Kind == ui.PointerReleased && m.pointerSelecting {
		m.pointerSelecting = false
		return true
	}
	if e.Kind == ui.PointerMoved && m.pointerSelecting {
		if d := m.current(); d != nil && cx != nil {
			if b, ok := cx.ElementBounds(m.editorKey(fmt.Sprintf("code-line-%d", d.scroll))); ok {
				line := max(0, min(d.buffer.LineCount()-1, d.scroll+int((e.Y-b.Y)/20)))
				m.moveCursor(d, line, hitColumn(d.buffer.Line(line), max(0, e.X-b.X-68)), true)
				return true
			}
		}
		return true
	}
	if e.Kind == ui.PointerPressed {
		m.pointerX, m.pointerShift = e.X, e.Modifiers&ui.ModifierShift != 0
		m.editing = false
		m.chatFocused = false
		m.updateFocused = false
		if d := m.current(); d != nil && cx != nil {
			for i := d.scroll; i < d.buffer.LineCount(); i++ {
				b, ok := cx.ElementBounds(m.editorKey(fmt.Sprintf("code-line-%d", i)))
				if !ok {
					break
				}
				if e.X >= b.X && e.X < b.X+b.Width && e.Y >= b.Y && e.Y < b.Y+b.Height {
					m.moveCursor(d, i, hitColumn(d.buffer.Line(i), max(0, e.X-b.X-68)), m.pointerShift)
					m.editing = true
					m.pointerSelecting = true
					return true
				}
			}
			// The editor's blank area still owns focus and places the caret at the
			// closest real line. Otherwise Undo/typing after a blank click is lost.
			if area, ok := cx.ElementBounds(m.editorKey("editor-content")); ok && e.X >= area.X && e.X < area.X+area.Width && e.Y >= area.Y && e.Y < area.Y+area.Height {
				if first, ok := cx.ElementBounds(m.editorKey(fmt.Sprintf("code-line-%d", d.scroll))); ok {
					line := max(0, min(d.buffer.LineCount()-1, d.scroll+int((e.Y-first.Y)/20)))
					m.moveCursor(d, line, hitColumn(d.buffer.Line(line), max(0, e.X-first.X-68)), m.pointerShift)
					m.editing, m.pointerSelecting = true, true
					return true
				}
			}
		}
		return false
	}
	if e.Kind == ui.Scroll {
		if d := m.current(); d != nil {
			d.scroll = max(0, min(d.buffer.LineCount()-1, d.scroll-int(e.Y)))
			d.holdScroll = true
		}
		return true
	}
	command := e.Modifiers&(ui.ModifierControl|ui.ModifierCommand) != 0
	shift := e.Modifiers&ui.ModifierShift != 0
	if m.keymapProfile() == "vscode" && e.Kind == ui.KeyPressed && !m.terminalFocused && m.requestLSP != nil && m.current() != nil {
		method := ""
		if e.Key == 123 {
			method = "textDocument/definition"
		} else if e.Key == 'F' && shift && e.Modifiers&ui.ModifierAlt != 0 {
			method = "textDocument/formatting"
		} else if e.Key == 'K' && command {
			method = "textDocument/hover"
		}
		if method != "" {
			m.requestLSP(m.current(), method)
			return true
		}
	}
	if m.keymapProfile() == "vscode" && e.Kind == ui.KeyPressed && command {
		switch e.Key {
		case 192, '`':
			if m.currentTerminal() == nil && m.newTerminal != nil {
				m.newTerminal()
			} else {
				m.focusTerminal()
			}
			return true
		case 'S':
			m.saveActive()
			return true
		case 'P':
			m.openQuickInput(shift)
			return true
		case 'J':
			m.togglePanel()
			return true
		case 'G':
			m.palette = false
			m.navigation = true
			m.query = ""
			m.chatFocused = false
			return true
		case 'I':
			m.palette = false
			m.editing = false
			m.panel = "COPILOT"
			m.showPanel = true
			m.chatFocused = true
			return true
		}
	}
	if m.terminalKeyboard(e) {
		return true
	}
	if m.chatFocused && m.panel == "COPILOT" && m.showPanel {
		if e.Kind == ui.KeyPressed && command && e.Key == 'V' {
			if m.readClipboard != nil {
				value, err := m.readClipboard()
				if err != nil {
					m.message = err.Error()
				} else if len(m.chatPrompt)+len(value) <= 16<<10 {
					m.chatPrompt += value
				}
			}
			return true
		}
		if e.Kind == ui.Character && e.Key >= 32 {
			if !command && len(m.chatPrompt) < 16<<10 {
				m.chatPrompt += string(rune(e.Key))
			}
			return true
		}
		if e.Kind == ui.KeyPressed {
			switch e.Key {
			case 8:
				r := []rune(m.chatPrompt)
				if len(r) > 0 {
					m.chatPrompt = string(r[:len(r)-1])
				}
				return true
			case 13:
				if m.askChat != nil && !m.chatBusy && m.chatPrompt != "" {
					prompt := m.chatPrompt
					m.chatPrompt = ""
					m.askChat(prompt)
				}
				return true
			case 27:
				if m.cancelChat != nil && (m.chatBusy || m.chatLoginBusy) {
					m.cancelChat()
				}
				m.chatFocused = false
				return true
			}
		}
		return false
	}
	if m.updateFocused && m.activity == "settings" {
		if e.Kind == ui.Character && e.Key >= 32 && !command && len(m.updateMirrorDraft) < 2048 {
			m.updateMirrorDraft += string(rune(e.Key))
			return true
		}
		if e.Kind == ui.KeyPressed {
			switch e.Key {
			case 8:
				r := []rune(m.updateMirrorDraft)
				if len(r) > 0 {
					m.updateMirrorDraft = string(r[:len(r)-1])
				}
				return true
			case 27:
				m.updateFocused = false
				return true
			case 13:
				config := m.updatesConfig
				config.Mirror = m.updateMirrorDraft
				config.Mode = "mirror"
				if m.configureUpdates != nil {
					m.configureUpdates(config)
				}
				m.updateFocused = false
				return true
			case 'V':
				if command && m.readClipboard != nil {
					value, err := m.readClipboard()
					if err == nil && len(value) <= 2048 {
						m.updateMirrorDraft = value
					}
					return true
				}
			}
		}
		return false
	}
	if m.navigation || m.palette {
		if e.Kind == ui.Character && e.Key >= 32 {
			m.query += string(rune(e.Key))
			return true
		}
		if e.Kind == ui.KeyPressed {
			switch e.Key {
			case 13:
				if m.navigation {
					m.goTo(m.query)
					return true
				}
			case 27:
				m.navigation = false
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
		return false
	}
	d := m.current()
	if !m.editing || d == nil {
		return false
	}
	if d.large != nil {
		return m.largeInput(cx, d, e)
	}
	// External navigation can set the caret before the next native input event.
	if d.cursor() != d.buffer.Selection().Active {
		m.moveCursor(d, d.line, d.column, false)
	}
	if e.Kind == ui.Character {
		if command && e.Modifiers&ui.ModifierAlt == 0 {
			return true
		}
		if e.Key >= 32 && utf8.ValidRune(rune(e.Key)) {
			m.replaceSelection(d, string(rune(e.Key)))
		}
		return true
	}
	if e.Kind != ui.KeyPressed {
		return false
	}
	if command && e.Key == 32 {
		if m.requestInline != nil {
			m.requestInline(d)
		}
		if m.requestCompletions != nil {
			m.requestCompletions(d)
		}
		return true
	}
	if command {
		switch e.Key {
		case 'A':
			end := d.buffer.PositionFromRunes(d.buffer.LineCount()-1, len([]rune(d.buffer.Line(d.buffer.LineCount()-1))))
			_ = d.buffer.SetSelection(textbuffer.Selection{Active: end})
			d.followSelection()
			m.documentEvent("selection", d, textbuffer.ChangeEvent{})
			return true
		case 'Z', 'Y':
			m.requestHistory(d, e.Key == 'Y' || shift)
			return true
		case 'C', 'X':
			r := d.buffer.Selection().Range()
			if r.Start == r.End {
				return true
			}
			value, err := d.buffer.RangeText(r)
			if err == nil && m.writeClipboard != nil {
				err = m.writeClipboard(value)
			}
			if m.writeClipboard == nil {
				err = errors.New("clipboard is unavailable")
			}
			if err != nil {
				m.message = err.Error()
			} else if e.Key == 'X' {
				m.replaceSelection(d, "")
			}
			return true
		case 'V':
			if m.readClipboard == nil {
				m.message = "clipboard is unavailable"
				return true
			}
			value, err := m.readClipboard()
			if err != nil {
				m.message = err.Error()
			} else {
				m.replaceSelection(d, value)
			}
			return true
		}
	}
	line, column := d.line, d.column
	switch e.Key {
	case 32:
		return true // WM_CHAR inserts space exactly once.
	case 37:
		if column > 0 {
			column--
		} else if line > 0 {
			line--
			column = len([]rune(d.buffer.Line(line)))
		}
	case 39:
		if column < len([]rune(d.buffer.Line(line))) {
			column++
		} else if line+1 < d.buffer.LineCount() {
			line++
			column = 0
		}
	case 38:
		line = max(0, line-1)
	case 40:
		line = min(d.buffer.LineCount()-1, line+1)
	case 36:
		column = 0
		if command {
			line = 0
		}
	case 35:
		if command {
			line = d.buffer.LineCount() - 1
		}
		column = len([]rune(d.buffer.Line(line)))
	case 8, 46:
		r := d.buffer.Selection().Range()
		if r.Start == r.End {
			if e.Key == 8 {
				if column > 0 {
					r.Start = d.buffer.PositionFromRunes(line, column-1)
				} else if line > 0 {
					r.Start = d.buffer.PositionFromRunes(line-1, len([]rune(d.buffer.Line(line-1))))
				}
			} else {
				if column < len([]rune(d.buffer.Line(line))) {
					r.End = d.buffer.PositionFromRunes(line, column+1)
				} else if line+1 < d.buffer.LineCount() {
					r.End = textbuffer.Position{Line: line + 1}
				}
			}
		}
		if r.Start != r.End {
			change, err := d.buffer.Apply([]textbuffer.Edit{{Range: r}}, nil)
			m.changed(d, change, err)
		}
		return true
	case 13:
		if len(m.completions) > 0 {
			m.chooseCompletion(m.completions[0])
			return true
		}
		m.replaceSelection(d, "\n")
		return true
	case 9:
		if m.suggestion != nil {
			s := *m.suggestion
			if s.path == d.path && s.version == d.buffer.Version() && s.position == d.cursor() {
				r := textbuffer.Range{Start: s.position, End: s.position}
				if s.item.Range != nil {
					r = *s.item.Range
				}
				if err := m.applyDocumentEdits(d.path, s.version, []textbuffer.Edit{{Range: r, Text: s.item.InsertText}}); err != nil {
					m.message = err.Error()
				} else if m.acceptInline != nil {
					m.acceptInline(s.item)
				}
				return true
			}
			m.suggestion = nil
		}
		if len(m.completions) > 0 {
			m.chooseCompletion(m.completions[0])
			return true
		}
		m.replaceSelection(d, "    ")
		return true
	case 27:
		m.suggestion = nil
		m.completions = nil
		m.moveCursor(d, line, column, false)
		return true
	default:
		return false
	}
	m.moveCursor(d, line, column, shift)
	return true
}
