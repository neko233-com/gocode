package main

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/neko233-com/gocode/internal/terminal"
	ui "github.com/neko233-com/godesktop"
)

type terminalTab struct {
	id                    int
	name, state           string
	session               *terminal.Session
	frame, selectionFrame *terminal.Frame
	size                  terminal.Size
	cancel                context.CancelFunc
	anchor, active        int
}

// Only this controller's worker registry is shared; tabs/model fields are UI
// state. A single pending dispatch per session bounds a flood of terminal output.
func (m *model) startTerminals(parent context.Context, cx *ui.Context) func() {
	ctx, cancel := context.WithCancel(parent)
	var mu sync.Mutex
	sessions := map[*terminal.Session]bool{}
	var workers sync.WaitGroup
	sequence := 0
	m.terminalHeight = 280
	m.newTerminal = func() {
		if ctx.Err() != nil {
			return
		}
		if len(m.terminals) >= 8 {
			m.message = "Close a terminal before creating another (maximum 8)"
			return
		}
		sequence++
		tabCtx, tabCancel := context.WithCancel(ctx)
		tab := &terminalTab{id: sequence, name: "Starting…", state: "Starting shell…", cancel: tabCancel, size: terminal.Size{Columns: 80, Rows: 10}}
		m.terminals = append(m.terminals, tab)
		m.activeTerminal = len(m.terminals) - 1
		m.focusTerminal()
		workspace := m.workspace
		isolated := m.terminalAcceptance
		workers.Go(func() {
			config, err := terminal.Shell(workspace, isolated)
			if err == nil {
				var session *terminal.Session
				session, err = terminal.Start(tabCtx, config, terminal.Size{Columns: 80, Rows: 10})
				if err == nil {
					mu.Lock()
					sessions[session] = true
					mu.Unlock()
					defer func() { _ = session.CloseAndWait(); mu.Lock(); delete(sessions, session); mu.Unlock() }()
					if !cx.Dispatch(func() {
						if tabCtx.Err() == nil {
							tab.name, tab.state, tab.session, tab.frame = config.Name, "", session, session.Snapshot()
							tab.size = terminal.Size{}
						}
					}) {
						return
					}
					var pending atomic.Bool
					for {
						select {
						case <-tabCtx.Done():
							return
						case <-session.Done():
							cx.Dispatch(func() {
								if tabCtx.Err() == nil {
									tab.frame = session.Snapshot()
								}
							})
							return
						case <-session.Updates():
							if pending.CompareAndSwap(false, true) {
								if !cx.Dispatch(func() {
									pending.Store(false)
									if tabCtx.Err() == nil {
										tab.frame = session.Snapshot()
									}
								}) {
									return
								}
							}
						}
					}
				}
			}
			cx.Dispatch(func() {
				if tabCtx.Err() == nil {
					tab.name, tab.state = "Failed", err.Error()
				}
			})
		})
	}
	m.killTerminal = func(tab *terminalTab) {
		tab.cancel()
		for i, candidate := range m.terminals {
			if candidate == tab {
				m.terminals = append(m.terminals[:i], m.terminals[i+1:]...)
				if i < m.activeTerminal {
					m.activeTerminal--
				}
				m.activeTerminal = min(m.activeTerminal, len(m.terminals)-1)
				break
			}
		}
		m.terminalSelecting = false
		if len(m.terminals) == 0 {
			m.terminalFocused = false
		}
	}
	return func() {
		cancel()
		mu.Lock()
		for session := range sessions {
			session.Close()
		}
		mu.Unlock()
		done := make(chan struct{})
		go func() { workers.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}
}

func (m *model) currentTerminal() *terminalTab {
	if m.activeTerminal < 0 || m.activeTerminal >= len(m.terminals) {
		return nil
	}
	return m.terminals[m.activeTerminal]
}
func (m *model) focusTerminal() {
	m.panel, m.showPanel, m.terminalFocused = "TERMINAL", true, true
	m.editing, m.chatFocused, m.updateFocused, m.palette, m.navigation = false, false, false, false, false
	m.pointerSelecting = false
}
func (m *model) hidePanel() {
	m.showPanel, m.terminalFocused, m.terminalSelecting, m.chatFocused = false, false, false, false
	m.editing = m.current() != nil
}
func (m *model) togglePanel() {
	if m.showPanel {
		m.hidePanel()
		return
	}
	m.showPanel = true
	if m.panel == "TERMINAL" {
		if m.currentTerminal() == nil && m.newTerminal != nil {
			m.newTerminal()
		} else {
			m.focusTerminal()
		}
	}
}
func (m *model) terminalPanelHeight(windowHeight float32) float32 {
	return min(max(120, m.terminalHeight), max(120, windowHeight-180))
}
func terminalMetrics() (float32, float32) {
	w := ui.TextAdvance("M", 14, codeFont())
	_, h := ui.MeasureText("M", 14, codeFont())
	return max(1, w), max(20, float32(math.Ceil(float64(h))))
}
func (m *model) terminalBody(width, height float32) *ui.Element {
	cw, ch := terminalMetrics()
	size := terminal.Size{Columns: max(2, min(400, int((width-24)/cw))), Rows: max(2, min(160, int((height-70)/ch)))}
	tab := m.currentTerminal()
	header := []*ui.Element{}
	for index, item := range m.terminals {
		bg := uint32(outer)
		if index == m.activeTerminal {
			bg = 0x37373d
		}
		header = append(header, button(fmt.Sprintf("%d: %s", item.id, item.name), fmt.Sprintf("terminal-tab-%d", item.id), func(*ui.Context) { m.activeTerminal = index; m.focusTerminal() }).Background(ui.RGB(bg)).Height(24))
	}
	header = append(header, spacer(), button("+", "terminal-new", func(*ui.Context) {
		if m.newTerminal != nil {
			m.newTerminal()
		}
	}).Width(28).Height(24))
	if tab != nil {
		header = append(header, button("×", "terminal-kill", func(*ui.Context) {
			if m.killTerminal != nil {
				m.killTerminal(tab)
			}
		}).Width(28).Height(24))
	}
	if tab == nil {
		return ui.Column(ui.Row(header...).Height(24), spacer(), label("Create a terminal  ·  Ctrl+`").Foreground(ui.RGB(muted)).Height(26).Key("terminal-create").OnClick(func(*ui.Context) {
			if m.newTerminal != nil {
				m.newTerminal()
			}
		}), spacer()).PaddingXY(12, 4)
	}
	if tab.session != nil && tab.size != size && (tab.frame == nil || !tab.frame.Exited) {
		if err := tab.session.Resize(size); err == nil {
			tab.size = size
		}
	}
	frame := tab.frame
	if tab.selectionFrame != nil {
		frame = tab.selectionFrame
	}
	if frame == nil {
		return ui.Column(ui.Row(header...).Height(24), label(tab.state).Height(28), spacer()).PaddingXY(12, 4)
	}
	rows := []*ui.Element{}
	for y, cells := range frame.Lines[:min(len(frame.Lines), size.Rows)] {
		runs := []*ui.Element{}
		for x := 0; x < len(cells) && x < size.Columns; {
			cell := cells[x]
			if cell.Width == 0 {
				x++
				continue
			}
			fg, bg := cell.Foreground, cell.Background
			selected := func(column int) bool {
				pos := y*frame.Size.Columns + column
				return tab.selectionFrame != nil && pos >= min(tab.anchor, tab.active) && pos < max(tab.anchor, tab.active)
			}
			if selected(x) {
				bg = 0x264f78
			}
			var text strings.Builder
			start, columns := x, 0
			for x < len(cells) && x < size.Columns {
				next := cells[x]
				if next.Width == 0 {
					x++
					continue
				}
				nextBG := next.Background
				if selected(x) {
					nextBG = 0x264f78
				}
				if next.Foreground != fg || nextBG != bg || next.Underline != cell.Underline {
					break
				}
				text.WriteString(next.Text)
				columns += next.Width
				x += next.Width
			}
			if x == start {
				x++
				continue
			}
			value := text.String()
			var run *ui.Element
			if strings.TrimSpace(value) == "" {
				run = ui.Column().Background(ui.RGB(bg))
			} else {
				run = ui.Text(value).FontFamily(codeFont()).FontSize(14).Foreground(ui.RGB(fg)).Background(ui.RGB(bg))
			}
			run.Width(float32(columns) * cw).Height(ch)
			if cell.Underline != 0 {
				run = ui.Stack(run, ui.Column(spacer(), ui.Column().Height(1).Background(ui.RGB(fg))).Width(float32(columns)*cw)).Width(float32(columns) * cw).Height(ch)
			}
			runs = append(runs, run)
		}
		rows = append(rows, ui.Row(runs...).Height(ch))
	}
	layers := []*ui.Element{ui.Column(rows...)}
	if frame.CursorVisible && frame.Offset == 0 && !frame.Exited && m.terminalFocused && tab.selectionFrame == nil && frame.CursorY >= 0 && frame.CursorY < size.Rows {
		cursor := ui.Column().Width(cw).Height(ch).Background(ui.RGBA(0xffffff, .35))
		if frame.CursorStyle == 2 {
			cursor.Width(2).Background(ui.RGB(foreground))
		}
		if frame.CursorStyle == 1 {
			cursor = ui.Column(spacer(), ui.Column().Height(2).Background(ui.RGB(foreground))).Width(cw).Height(ch)
		}
		layers = append(layers, ui.Column(ui.Column().Height(float32(frame.CursorY)*ch), ui.Row(ui.Column().Width(float32(frame.CursorX)*cw), cursor).Height(ch)))
	}
	status := tab.state
	if frame.Exited {
		status = fmt.Sprintf("Process exited with code %d", frame.ExitCode)
	}
	if frame.Error != "" {
		status = frame.Error
	}
	if frame.Offset > 0 {
		status = fmt.Sprintf("Scrollback %d / %d", frame.Offset, frame.History)
	}
	grid := ui.Stack(layers...).Height(float32(size.Rows) * ch).Key("terminal-grid").OnClick(func(*ui.Context) { m.focusTerminal() })
	return ui.Column(ui.Row(header...).Height(24), grid, spacer(), label(status).FontSize(11).Foreground(ui.RGB(muted)).Height(12)).PaddingXY(12, 4)
}

func (m *model) terminalPointer(cx *ui.Context, e ui.InputEvent) bool {
	if cx == nil {
		return false
	}
	px, py := e.X, e.Y
	if e.Kind == ui.Scroll {
		px, py = e.PointerX, e.PointerY
	}
	inside := func(b ui.Bounds) bool { return px >= b.X && px < b.X+b.Width && py >= b.Y && py < b.Y+b.Height }
	if m.panelResizing {
		switch e.Kind {
		case ui.PointerMoved:
			_, h := cx.WindowSize()
			m.terminalHeight = min(max(120, m.panelDragHeight+m.panelDragY-e.Y), max(120, h-180))
			return true
		case ui.PointerReleased, ui.InputCancelled:
			m.panelResizing = false
			return true
		}
	}
	if e.Kind == ui.PointerPressed {
		if b, ok := cx.ElementBounds("panel-resize"); ok && inside(b) {
			m.panelResizing, m.panelDragY, m.panelDragHeight = true, e.Y, m.terminalHeight
			return true
		}
	}
	tab := m.currentTerminal()
	if tab == nil || tab.frame == nil || m.panel != "TERMINAL" || !m.showPanel {
		return false
	}
	b, ok := cx.ElementBounds("terminal-grid")
	if !ok {
		return false
	}
	position := func() int {
		cw, ch := terminalMetrics()
		f := tab.frame
		if tab.selectionFrame != nil {
			f = tab.selectionFrame
		}
		y := max(0, min(f.Size.Rows-1, int((e.Y-b.Y)/ch)))
		x := max(0, min(f.Size.Columns, int((e.X-b.X)/cw)))
		return y*f.Size.Columns + x
	}
	if e.Kind == ui.InputCancelled {
		m.terminalSelecting = false
		return false
	}
	if e.Kind == ui.PointerReleased && m.terminalSelecting {
		tab.active = position()
		if tab.anchor == tab.active {
			tab.selectionFrame = nil
		}
		m.terminalSelecting = false
		return true
	}
	if e.Kind == ui.PointerMoved && m.terminalSelecting {
		tab.active = position()
		return true
	}
	if e.Kind == ui.PointerPressed && inside(b) {
		m.focusTerminal()
		tab.selectionFrame = tab.frame
		tab.anchor = position()
		tab.active = tab.anchor
		m.terminalSelecting = true
		return true
	}
	if e.Kind == ui.Scroll && inside(b) {
		tab.selectionFrame = nil
		m.terminalSelecting = false
		if tab.session != nil {
			if err := tab.session.Scroll(int(e.Y)); err != nil {
				m.message = err.Error()
			}
		}
		return true
	}
	return false
}
func (t *terminalTab) selectedText() string {
	if t.selectionFrame == nil || t.anchor == t.active {
		return ""
	}
	f := t.selectionFrame
	start, end := min(t.anchor, t.active), max(t.anchor, t.active)
	var result strings.Builder
	for row := start / f.Size.Columns; row <= min(len(f.Lines)-1, (end-1)/f.Size.Columns); row++ {
		var line strings.Builder
		for column, cell := range f.Lines[row] {
			pos := row*f.Size.Columns + column
			if pos >= start && pos < end && cell.Width > 0 {
				line.WriteString(cell.Text)
			}
		}
		if result.Len() > 0 {
			result.WriteByte('\n')
		}
		result.WriteString(strings.TrimRight(line.String(), " "))
		if result.Len() > terminal.MaxInputBytes {
			return ""
		}
	}
	return result.String()
}

func (m *model) terminalKeyboard(e ui.InputEvent) bool {
	if !m.terminalFocused || m.panel != "TERMINAL" || !m.showPanel || m.palette || m.navigation {
		return false
	}
	tab := m.currentTerminal()
	if tab == nil || tab.session == nil || tab.frame == nil || tab.frame.Exited {
		if e.Kind == ui.KeyPressed || e.Kind == ui.Character {
			m.message = "Terminal is starting or has exited; input was not accepted"
		}
		return e.Kind == ui.KeyPressed || e.Kind == ui.Character
	}
	control, shift, command := e.Modifiers&ui.ModifierControl != 0, e.Modifiers&ui.ModifierShift != 0, e.Modifiers&ui.ModifierCommand != 0
	if e.Kind == ui.KeyPressed && ((control && shift) || command) && e.Key == 'C' {
		if value := tab.selectedText(); value != "" && m.writeClipboard != nil {
			if err := m.writeClipboard(value); err != nil {
				m.message = err.Error()
			}
		}
		return true
	}
	var err error
	if e.Kind == ui.KeyPressed && (control || command) && e.Key == 'V' {
		if m.readClipboard != nil {
			var value string
			value, err = m.readClipboard()
			if err == nil {
				err = tab.session.Paste(value)
			}
		}
	} else if e.Kind == ui.Character {
		if (!control || e.Modifiers&ui.ModifierAlt != 0) && !command && e.Key >= 32 {
			if !control && e.Modifiers&ui.ModifierAlt != 0 {
				err = tab.session.SendKey(uv.KeyPressEvent{Code: rune(e.Key), Mod: uv.ModAlt})
			} else {
				err = tab.session.SendText(string(rune(e.Key)))
			}
		}
	} else if e.Kind == ui.KeyPressed {
		key := uv.KeyPressEvent{}
		switch e.Key {
		case 8:
			key.Code = uv.KeyBackspace
		case 9:
			key.Code = uv.KeyTab
		case 13:
			key.Code = uv.KeyEnter
		case 27:
			key.Code = uv.KeyEscape
		case 33:
			key.Code = uv.KeyPgUp
		case 34:
			key.Code = uv.KeyPgDown
		case 35:
			key.Code = uv.KeyEnd
		case 36:
			key.Code = uv.KeyHome
		case 37:
			key.Code = uv.KeyLeft
		case 38:
			key.Code = uv.KeyUp
		case 39:
			key.Code = uv.KeyRight
		case 40:
			key.Code = uv.KeyDown
		case 45:
			key.Code = uv.KeyInsert
		case 46:
			key.Code = uv.KeyDelete
		default:
			if e.Key >= 112 && e.Key <= 123 {
				key.Code = uv.KeyF1 + rune(e.Key-112)
			} else if control && e.Key >= 'A' && e.Key <= 'Z' {
				key.Code = rune(e.Key + 32)
			} else {
				return true
			}
		}
		if control {
			key.Mod |= uv.ModCtrl
		}
		if shift {
			key.Mod |= uv.ModShift
		}
		if e.Modifiers&ui.ModifierAlt != 0 {
			key.Mod |= uv.ModAlt
		}
		err = tab.session.SendKey(key)
	} else {
		return false
	}
	tab.selectionFrame = nil
	m.terminalSelecting = false
	if err != nil {
		m.message = err.Error()
	}
	return true
}
