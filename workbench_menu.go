package main

import (
	"fmt"
	"strings"

	ui "github.com/neko233-com/godesktop"
)

var workbenchMenuNames = []string{"File", "Edit", "Selection", "View", "Go", "Run", "Terminal", "Help"}

type workbenchMenu struct {
	visible, overflow       []string
	suppressCharacter       int
	name, submenu           string
	index, subindex         int
	x, y, subx, suby        float32
	pressed                 string
	editing, terminal, chat bool
}

func (m *model) openMenu(cx *ui.Context, name string) {
	if m.menu.name == "" {
		m.menu.editing, m.menu.terminal, m.menu.chat = m.editing, m.terminalFocused, m.chatFocused
	}
	m.menu.name, m.menu.submenu, m.menu.index, m.menu.subindex = name, "", -1, -1
	m.palette, m.navigation = false, false
	if cx != nil {
		b, ok := cx.ElementBounds("menu-" + name)
		if !ok {
			b, ok = cx.ElementBounds("menu-More")
		}
		if ok {
			m.menu.x, m.menu.y = b.X, b.Y+b.Height
		}
	}
}
func (m *model) visibleMenuNames() []string {
	if len(m.menu.visible) == 0 {
		return workbenchMenuNames
	}
	return m.menu.visible
}
func (m *model) closeMenu() {
	m.menu.name, m.menu.submenu, m.menu.pressed = "", "", ""
	m.editing, m.terminalFocused, m.chatFocused = m.menu.editing, m.menu.terminal, m.menu.chat
}
func menuRowKey(level, index int) string { return fmt.Sprintf("menupopup-%d-%d", level, index) }
func (m *model) selectMenuItem(cx *ui.Context, level, index int, activate bool) {
	name := m.menu.name
	if level == 1 {
		name = m.menu.submenu
	}
	entries := m.menuEntries(name)
	if index < 0 || index >= len(entries) {
		return
	}
	entry := entries[index]
	if level == 0 {
		m.menu.index = index
		if entry.Submenu != "" {
			if m.menu.submenu != entry.Submenu {
				m.menu.subindex = -1
			}
			m.menu.submenu = entry.Submenu
			if cx != nil {
				if b, ok := cx.ElementBounds(menuRowKey(level, index)); ok {
					m.menu.subx, m.menu.suby = b.X+b.Width+4, b.Y-4
				}
			}
			return
		}
		m.menu.submenu = ""
	} else {
		m.menu.subindex = index
	}
	if !activate || !entry.Enabled {
		return
	}
	m.closeMenu()
	if strings.HasPrefix(entry.ID, "recent:") {
		m.open(strings.TrimPrefix(entry.ID, "recent:"))
		return
	}
	if entry.ID == "about" {
		m.message = "gocode " + appVersion() + " · native Go/D3D12 · VS Code compatible API under development"
		return
	}
	m.runWorkbenchCommand(entry.ID)
}
func (m *model) menuPanel(name string, level int) *ui.Element {
	items := []*ui.Element{}
	entries := m.menuEntries(name)
	for i, entry := range entries {
		if entry.Title == "" {
			items = append(items, ui.Column(rule()).PaddingXY(8, 4).Height(9))
			continue
		}
		fg := uint32(foreground)
		if !entry.Enabled {
			fg = 0x6e6e6e
		}
		bg := ui.Color{}
		selected := m.menu.index
		if level == 1 {
			selected = m.menu.subindex
		}
		if i == selected && entry.Enabled {
			bg = ui.RGB(0x04395e)
			fg = 0xffffff
		}
		check := ""
		if entry.Checked {
			check = "✓"
		}
		binding := entry.Binding
		if entry.Submenu != "" {
			binding = "›"
		}
		items = append(items, ui.Row(label(check).Width(18), label(entry.Title).Flex(1).Foreground(ui.RGB(fg)), label(binding).Foreground(ui.RGB(fg))).PaddingXY(8, 0).Height(24).Radius(4).Background(bg).FocusRing(false).Key(menuRowKey(level, i)).OnClick(func(c *ui.Context) { m.selectMenuItem(c, level, i, true) }))
	}
	return ui.Column(ui.Column(items...).Padding(4).Background(ui.RGB(0x1f1f1f)).ClipRounded(7)).Padding(1).Background(ui.RGB(0x454545)).ClipRounded(8).Shadow(menuPopupShadow()).Width(324).Key(fmt.Sprintf("menu-panel-%d", level))
}
func positionedPopup(popup *ui.Element, x, y float32) *ui.Element {
	return popup.Position(max(0, x), max(0, y))
}
func (m *model) menuOverlay(cx *ui.Context, base *ui.Element) *ui.Element {
	w, h := cx.WindowSize()
	m.menu.x = min(m.menu.x, max(0, w-324))
	m.menu.y = min(m.menu.y, max(36, h-menuHeight(m.menuEntries(m.menu.name))))
	layers := []*ui.Element{base, ui.Column().Key("menu-dismiss").OnClick(func(*ui.Context) { m.closeMenu() }), positionedPopup(m.menuPanel(m.menu.name, 0), m.menu.x, m.menu.y)}
	if m.menu.submenu != "" {
		x := m.menu.subx
		if x+324 > w {
			x = max(0, m.menu.x-324)
		}
		y := min(m.menu.suby, max(36, h-menuHeight(m.menuEntries(m.menu.submenu))))
		layers = append(layers, positionedPopup(m.menuPanel(m.menu.submenu, 1), x, y))
	}
	return ui.Stack(layers...)
}
func menuHeight(entries []menuEntry) float32 {
	h := float32(10)
	for _, e := range entries {
		if e.Title == "" {
			h += 9
		} else {
			h += 24
		}
	}
	return h
}

// Return handled separately from consumed: popup clicks still use framework
// press/release identity, while all underlying editor/terminal input is excluded.
func (m *model) menuInput(cx *ui.Context, e ui.InputEvent) (bool, bool) {
	if e.Kind == ui.KeyPressed && e.Modifiers&ui.ModifierAlt != 0 && e.Modifiers&(ui.ModifierControl|ui.ModifierCommand|ui.ModifierShift) == 0 {
		for _, name := range workbenchMenuNames {
			if int(name[0]) == e.Key {
				m.openMenu(cx, name)
				return true, true
			}
		}
	}
	if e.Kind == ui.KeyPressed && e.Key == 121 {
		if m.menu.name == "" {
			m.openMenu(cx, "File")
		} else {
			m.closeMenu()
		}
		return true, true
	}
	if m.menu.name == "" {
		if e.Kind == ui.PointerPressed || e.Kind == ui.PointerReleased {
			for _, name := range append(append([]string(nil), m.visibleMenuNames()...), "command-center") {
				key := "menu-" + name
				if name == "command-center" {
					key = name
				}
				if cx != nil {
					if b, ok := cx.ElementBounds(key); ok && insideBounds(b, e.X, e.Y) {
						return true, false
					}
				}
			}
		}
		return false, false
	}
	if e.Kind == ui.InputCancelled {
		m.closeMenu()
		return true, true
	}
	if e.Kind == ui.PointerMoved {
		for _, name := range m.visibleMenuNames() {
			if b, ok := cx.ElementBounds("menu-" + name); ok && insideBounds(b, e.X, e.Y) {
				if name != m.menu.name {
					m.openMenu(cx, name)
				}
				return true, true
			}
		}
		for level, name := range []string{m.menu.name, m.menu.submenu} {
			if name == "" {
				continue
			}
			for i := range m.menuEntries(name) {
				if b, ok := cx.ElementBounds(menuRowKey(level, i)); ok && insideBounds(b, e.X, e.Y) {
					m.selectMenuItem(cx, level, i, false)
					return true, true
				}
			}
		}
		return true, true
	}
	if e.Kind == ui.PointerPressed || e.Kind == ui.PointerReleased {
		return true, false
	}
	if e.Kind != ui.KeyPressed {
		return true, true
	}
	if e.Key == 27 {
		if m.menu.submenu != "" {
			m.menu.submenu = ""
		} else {
			m.closeMenu()
		}
		return true, true
	}
	name, index := m.menu.name, m.menu.index
	if m.menu.submenu != "" {
		name, index = m.menu.submenu, m.menu.subindex
	}
	entries := m.menuEntries(name)
	switch e.Key {
	case 38, 40, 36, 35:
		delta := 1
		if e.Key == 38 {
			delta = -1
		}
		if e.Key == 36 {
			index = -1
		}
		if e.Key == 35 {
			index = 0
			delta = -1
		}
		for range len(entries) {
			index = (index + delta + len(entries)) % len(entries)
			if entries[index].Enabled {
				break
			}
		}
		if m.menu.submenu != "" {
			m.menu.subindex = index
		} else {
			m.menu.index = index
		}
	case 13:
		level := 0
		if m.menu.submenu != "" {
			level = 1
		}
		m.selectMenuItem(cx, level, index, true)
	case 37, 39:
		if e.Key == 37 && m.menu.submenu != "" {
			m.menu.submenu = ""
			break
		}
		if e.Key == 39 && index >= 0 && index < len(entries) && entries[index].Submenu != "" {
			m.selectMenuItem(cx, 0, index, false)
			m.menu.subindex = 0
			break
		}
		names := m.visibleMenuNames()
		for i, n := range names {
			if n == m.menu.name {
				delta := 1
				if e.Key == 37 {
					delta = -1
				}
				m.openMenu(cx, names[(i+delta+len(names))%len(names)])
				break
			}
		}
	default:
		if e.Key >= 'A' && e.Key <= 'Z' {
			letter := strings.ToLower(string(rune(e.Key)))
			for i, entry := range entries {
				if entry.Enabled && strings.HasPrefix(strings.ToLower(entry.Title), letter) {
					m.menu.suppressCharacter = e.Key
					level := 0
					if m.menu.submenu != "" {
						level = 1
					}
					m.selectMenuItem(cx, level, i, true)
					break
				}
			}
		}
	}
	return true, true
}
