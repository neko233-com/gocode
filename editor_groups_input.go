package main

import (
	"fmt"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

func vSelection(g *editorGroup, d *document) textbuffer.Selection {
	if d == nil || d.buffer == nil {
		return textbuffer.Selection{}
	}
	return vOrCreate(g, d).selection
}
func insideBounds(b ui.Bounds, x, y float32) bool {
	return x >= b.X && x < b.X+b.Width && y >= b.Y && y < b.Y+b.Height
}
func (m *model) groupInput(cx *ui.Context, e ui.InputEvent) bool {
	if e.Kind == ui.InputCancelled {
		m.groups.drag = nil
		m.groups.pressed = nil
	}
	if n := m.groups.drag; n != nil {
		if e.Kind == ui.PointerMoved && cx != nil {
			if b, ok := cx.ElementBounds(fmt.Sprintf("editor-split-%d", n.id)); ok {
				size, pos := b.Width, e.X-b.X
				minimum := float32(180)
				if n.down {
					size, pos, minimum = b.Height, e.Y-b.Y, 100
				}
				limit := min(float32(.45), minimum/max(1, size))
				n.ratio = max(limit, min(1-limit, pos/max(1, size)))
			}
			return true
		}
		if e.Kind == ui.PointerReleased {
			m.groups.drag = nil
			return true
		}
	}
	if e.Kind == ui.KeyPressed && e.Modifiers&(ui.ModifierControl|ui.ModifierCommand) != 0 && !m.terminalFocused {
		if e.Key == '\\' || e.Key == 220 {
			m.splitEditor(e.Modifiers&ui.ModifierShift != 0)
			return true
		}
		if e.Key >= '1' && e.Key <= '9' {
			gs := m.allGroups()
			i := e.Key - '1'
			if i < len(gs) {
				m.focusGroup(gs[i].id)
			}
			return true
		}
	}
	if cx == nil || m.groups.root == nil {
		return false
	}
	if e.Kind == ui.PointerPressed {
		m.groups.pressed = nil
		var drag func(*editorSplit) bool
		drag = func(n *editorSplit) bool {
			if n.group != nil {
				return false
			}
			if b, ok := cx.ElementBounds(fmt.Sprintf("editor-sash-%d", n.id)); ok && insideBounds(b, e.X, e.Y) {
				m.groups.drag = n
				m.pointerSelecting = false
				return true
			}
			return drag(n.left) || drag(n.right)
		}
		if drag(m.groups.root) {
			return true
		}
		for _, g := range m.allGroups() {
			if b, ok := cx.ElementBounds(groupKey(g, "editor-group")); ok && insideBounds(b, e.X, e.Y) {
				if m.groups.active != g.id {
					m.focusGroup(g.id)
				}
				break
			}
		}
		for _, g := range m.allGroups() {
			for _, key := range []string{"close-group", "split", "split-down"} {
				if b, ok := cx.ElementBounds(groupKey(g, key)); ok && insideBounds(b, e.X, e.Y) {
					m.groups.pressed = g
					m.groups.pressGeneration = g.generation
				}
			}
		}
	}
	if e.Kind == ui.Scroll {
		for _, g := range m.allGroups() {
			if g.id == m.groups.active {
				continue
			}
			if b, ok := cx.ElementBounds(groupKey(g, "editor-tabs")); ok && insideBounds(b, e.PointerX, e.PointerY) {
				delta := e.X
				if delta == 0 {
					delta = -e.Y
				}
				g.tabs.offset = tabOffset(g.tabs.offset+delta*40, 0, 0, g.tabs.viewport, g.tabs.total, false)
				return true
			}
			if b, ok := cx.ElementBounds(groupKey(g, "editor-content")); ok && insideBounds(b, e.PointerX, e.PointerY) && g.current != nil {
				d := g.current
				v := vOrCreate(g, d)
				if v.large != nil {
					copy := *d
					copy.large = v.large
					copy.scroll = v.scroll
					if copy.large.byteMode {
						m.navigateLarge(&copy, 0, copy.large.byteOffset-int64(e.Y)*1024, true)
					} else {
						m.navigateLarge(&copy, int64(max(0, min(int(copy.large.stats.Lines)-1, v.scroll-int(e.Y)))), 0, false)
					}
					v.scroll = copy.scroll
					v.line = copy.line
				} else {
					v.scroll = max(0, min(d.buffer.LineCount()-1, v.scroll-int(e.Y)))
					v.holdScroll = true
				}
				return true
			}
		}
	}
	return false
}

func (m *model) acceptGroupAction(g *editorGroup) bool {
	if g == nil {
		return true
	}
	if m.findGroup(g.id) != g {
		return false
	}
	if m.groups.pressed != nil {
		same := m.groups.pressed == g && m.groups.pressGeneration == g.generation
		m.groups.pressed = nil
		if !same {
			m.message = "Group changed during click; review again"
			return false
		}
	}
	return true
}
