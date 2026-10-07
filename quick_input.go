package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	ui "github.com/neko233-com/godesktop"
)

type quickInputState struct {
	commands           bool
	recent, everywhere bool
	index              int
	editing, terminal  bool
}
type quickInputItem struct {
	ID, Title, Detail, Binding string
	Command                    bool
}

func (m *model) openQuickInput(commands bool) {
	if !m.palette {
		m.quick.editing, m.quick.terminal = m.editing, m.terminalFocused
	}
	m.palette = true
	m.navigation = false
	m.quick.commands = commands
	m.quick.recent, m.quick.everywhere = false, false
	m.quick.index = 0
	m.query = ""
	if commands {
		m.query = ">"
	}
	m.terminalFocused, m.editing, m.chatFocused = false, false, false
}
func (m *model) closeQuickInput() {
	m.palette = false
	m.query = ""
	m.editing, m.terminalFocused = m.quick.editing, m.quick.terminal
}
func quickMatches(text, query string) bool {
	text, query = strings.ToLower(text), strings.ToLower(strings.TrimSpace(query))
	for _, word := range strings.Fields(query) {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}
func (m *model) quickItems() []quickInputItem {
	var items []quickInputItem
	if strings.HasPrefix(m.query, ">") || m.quick.everywhere {
		query := strings.TrimPrefix(m.query, ">")
		for _, c := range m.workbenchCommands() {
			if c.Enabled && quickMatches(c.Title, query) {
				items = append(items, quickInputItem{ID: c.ID, Title: c.Title, Binding: c.Binding, Command: true})
				if len(items) >= 100 {
					break
				}
			}
		}
	}
	if !strings.HasPrefix(m.query, ">") && len(items) < 100 {
		files := m.files
		if m.quick.recent {
			files = nil
			for _, d := range m.activeTabs().mru {
				if !d.untitled {
					files = append(files, d.path)
				}
			}
		}
		for _, file := range files {
			if quickMatches(file, m.query) {
				items = append(items, quickInputItem{ID: file, Title: filepath.Base(file), Detail: filepath.ToSlash(filepath.Dir(file))})
				if len(items) >= 100 {
					break
				}
			}
		}
	}
	return items
}
func (m *model) chooseQuickItem(item quickInputItem) {
	commands := item.Command || strings.HasPrefix(m.query, ">")
	m.closeQuickInput()
	if commands {
		m.runWorkbenchCommand(item.ID)
	} else {
		m.open(item.ID)
	}
}
func (m *model) quickOverlay(cx *ui.Context, base *ui.Element) *ui.Element {
	w, h := cx.WindowSize()
	width := min(float32(600), max(200, w-32))
	items := m.quickItems()
	m.quick.index = max(0, min(m.quick.index, max(0, len(items)-1)))
	query := m.query + "▏"
	if m.query == "" {
		query = "Search files by name"
	}
	children := []*ui.Element{ui.Column(label(query).PaddingXY(8, 0).Height(28).ClipRounded(3).Background(ui.RGB(0x313131))).Padding(1).ClipRounded(4).Background(ui.RGB(accent)).Key("quick-input")}
	visible := max(1, min(12, int((h-80)/24)))
	start := max(0, m.quick.index-visible+1)
	if len(items) == 0 {
		children = append(children, label("No matching results").PaddingXY(8, 0).Height(24))
	}
	for i := start; i < len(items) && i < start+visible; i++ {
		item := items[i]
		bg := ui.Color{}
		if i == m.quick.index {
			bg = ui.RGB(0x04395e)
		}
		children = append(children, ui.Row(label(item.Title), label(item.Detail).FontSize(11).Foreground(ui.RGB(muted)).Flex(1), label(item.Binding).FontSize(11).Foreground(ui.RGB(muted))).Gap(8).PaddingXY(8, 0).Height(24).ClipRounded(4).Background(bg).Key(fmt.Sprintf("quick-item-%d", i)).OnClick(func(*ui.Context) { m.chooseQuickItem(item) }))
	}
	popup := ui.Column(children...).Gap(4).Padding(6).Width(width).ClipRounded(8).Background(ui.RGB(0x222222)).Key("quick-popup")
	return ui.Stack(base, ui.Column().OnClick(func(*ui.Context) { m.closeQuickInput() }), ui.Column(ui.Column().Height(6), ui.Row(spacer(), popup, spacer()), spacer()))
}
func (m *model) quickInput(e ui.InputEvent) (bool, bool) {
	if !m.palette {
		return false, false
	}
	if e.Kind == ui.PointerPressed || e.Kind == ui.PointerReleased {
		return true, false
	}
	if e.Kind == ui.InputCancelled {
		m.closeQuickInput()
		return true, true
	}
	command := e.Modifiers&(ui.ModifierControl|ui.ModifierCommand) != 0
	if e.Kind == ui.Character && !command && e.Key >= 32 && utf8.ValidRune(rune(e.Key)) && len(m.query) < 1024 {
		m.query += string(rune(e.Key))
		m.quick.index = 0
		return true, true
	}
	if e.Kind == ui.KeyPressed {
		switch e.Key {
		case 27:
			m.closeQuickInput()
		case 8:
			r := []rune(m.query)
			if len(r) > 0 {
				m.query = string(r[:len(r)-1])
			}
			m.quick.index = 0
		case 38:
			m.quick.index = max(0, m.quick.index-1)
		case 40:
			m.quick.index = min(max(0, len(m.quickItems())-1), m.quick.index+1)
		case 13:
			items := m.quickItems()
			if m.quick.index < len(items) {
				m.chooseQuickItem(items[m.quick.index])
			}
		case 'V':
			if command && m.readClipboard != nil {
				s, err := m.readClipboard()
				if err == nil && len(s)+len(m.query) <= 1024 {
					m.query += s
					m.quick.index = 0
				}
			}
		}
	}
	return true, true
}
