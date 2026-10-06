package main

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"

	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

type editorTabState struct {
	offset, viewport, total float32
	lastActive              *document
	lastCount               int
	reveal, dragging        bool
	anchorX, anchorOffset   float32
	sequence                uint64
	mru                     []*document
	switchOrder             []*document
	switchIndex             int
	cycling                 bool
}

func (m *model) tabKey(d *document) string {
	if d.tabID == 0 {
		m.tabs.sequence++
		d.tabID = m.tabs.sequence
	}
	return fmt.Sprintf("tab-%d", d.tabID)
}
func (m *model) tabCloseKey(d *document) string { return "close-" + m.tabKey(d) }

func (m *model) focusTab(d *document) {
	for i, candidate := range m.docs {
		if candidate == d {
			m.active = i
			m.editing = true
			m.search.focus = -2
			m.terminalFocused, m.chatFocused, m.updateFocused = false, false, false
			m.pointerSelecting = false
			m.tabs.dragging = false
			m.documentEvent("focus", d, textbuffer.ChangeEvent{})
			return
		}
	}
}
func (m *model) closeDocumentTab(d *document) {
	for i, candidate := range m.docs {
		if candidate == d {
			m.closeTab(i)
			return
		}
	}
}

func (m *model) recordTabEvent(kind string, d *document) {
	if kind != "focus" && kind != "close" {
		return
	}
	if !m.tabs.cycling {
		m.endTabSwitch()
	}
	kept := m.tabs.mru[:0]
	previousLength := len(m.tabs.mru)
	for _, entry := range m.tabs.mru {
		if entry != d {
			kept = append(kept, entry)
		}
	}
	clear(m.tabs.mru[len(kept):previousLength])
	if kind == "focus" && d != nil {
		kept = append(kept, d)
		m.tabs.reveal = true
	}
	m.tabs.mru = kept
}

func (m *model) endTabSwitch() {
	clear(m.tabs.switchOrder)
	m.tabs.switchOrder = nil
	m.tabs.switchIndex = 0
}

func tabOffset(offset, start, end, width, total float32, reveal bool) float32 {
	if math.IsNaN(float64(offset)) || math.IsInf(float64(offset), 0) {
		offset = 0
	}
	if reveal && width > 0 {
		if end-start > width || start < offset {
			offset = start
		} else if end > offset+width {
			offset = end - width
		}
	}
	return min(max(0, total-width), max(0, offset))
}

func (m *model) tabsView(cx *ui.Context) *ui.Element {
	width, _ := cx.WindowSize()
	viewport := max(0, width-320) // activity/borders/sidebar = 290; editor action = 30
	prefix := make([]float32, len(m.docs)+1)
	for i, d := range m.docs {
		if d.tabWidth == 0 {
			w, _ := ui.MeasureText(filepath.Base(d.path), 13, "")
			d.tabWidth = max(120, w+64)
		}
		prefix[i+1] = prefix[i] + d.tabWidth + 1
	}
	total := prefix[len(m.docs)]
	current := m.current()
	reveal := m.tabs.reveal || m.tabs.lastActive != current || m.tabs.lastCount != len(m.docs) || m.tabs.viewport != viewport
	start, end := float32(0), float32(0)
	if m.active >= 0 && m.active < len(m.docs) {
		start, end = prefix[m.active], prefix[m.active+1]
	}
	m.tabs.offset = tabOffset(m.tabs.offset, start, end, viewport, total, reveal)
	m.tabs.viewport, m.tabs.total = viewport, total
	m.tabs.lastActive, m.tabs.lastCount, m.tabs.reveal = current, len(m.docs), false
	first := sort.Search(len(m.docs), func(i int) bool { return prefix[i+1] > m.tabs.offset })
	last := sort.Search(len(m.docs), func(i int) bool { return prefix[i] >= m.tabs.offset+viewport })
	entries := []*ui.Element{ui.Column().Width(prefix[first])}
	for i := first; i < last; i++ {
		d := m.docs[i]
		bg := uint32(outer)
		top := ui.Color{}
		if d == current {
			bg = editor
			top = ui.RGB(accent)
		}
		symbol := "×"
		if d.dirty() {
			symbol = "●"
		}
		row := ui.Row(label("{} ").Foreground(ui.RGB(0x519aba)).Width(28), label(filepath.Base(d.path)).Flex(1), button(symbol, m.tabCloseKey(d), func(*ui.Context) { m.closeDocumentTab(d) }).Width(26)).Padding(5).Height(34)
		tab := ui.Column(ui.Column().Height(1).Background(top), row).Width(d.tabWidth).Background(ui.RGB(bg)).Key(m.tabKey(d)).OnClick(func(*ui.Context) { m.focusTab(d) })
		entries = append(entries, tab, ui.Column().Width(1).Background(ui.RGB(border)))
	}
	entries = append(entries, ui.Column().Width(total-prefix[last]))
	content := ui.Row(entries...).Width(total).Height(35)
	layers := []*ui.Element{ui.Viewport(content).ScrollOffset(m.tabs.offset, 0).Key("editor-tabs")}
	if total > viewport && viewport > 0 {
		thumb := min(viewport, max(24, viewport*viewport/total))
		left := m.tabs.offset / (total - viewport) * (viewport - thumb)
		track := ui.Row(ui.Column().Width(left), ui.Column().Width(thumb).Background(ui.RGB(0x555555)).Key("editor-tabs-thumb"), spacer()).Height(3).Key("editor-tabs-track")
		layers = append(layers, ui.Column(spacer(), track))
	}
	return ui.Row(ui.Stack(layers...).Flex(1), icon("split", "split", func(*ui.Context) { m.message = "Split editors are not implemented in this preview" })).Height(35).Background(ui.RGB(outer))
}

func (m *model) tabInput(cx *ui.Context, e ui.InputEvent) bool {
	if m.tabs.dragging {
		switch e.Kind {
		case ui.PointerMoved:
			travel := m.tabs.viewport - min(m.tabs.viewport, max(24, m.tabs.viewport*m.tabs.viewport/m.tabs.total))
			if travel > 0 {
				m.tabs.offset = tabOffset(m.tabs.anchorOffset+(e.X-m.tabs.anchorX)*(m.tabs.total-m.tabs.viewport)/travel, 0, 0, m.tabs.viewport, m.tabs.total, false)
			}
			return true
		case ui.PointerReleased, ui.InputCancelled:
			m.tabs.dragging = false
			return true
		}
	}
	if cx != nil {
		inside := func(key string, x, y float32) bool {
			b, ok := cx.ElementBounds(key)
			return ok && x >= b.X && x < b.X+b.Width && y >= b.Y && y < b.Y+b.Height
		}
		if e.Kind == ui.Scroll && inside("editor-tabs", e.PointerX, e.PointerY) {
			delta := e.X
			if delta == 0 {
				delta = -e.Y
			}
			m.tabs.offset = tabOffset(m.tabs.offset+delta*40, 0, 0, m.tabs.viewport, m.tabs.total, false)
			return true
		}
		if e.Kind == ui.PointerPressed && inside("editor-tabs-track", e.X, e.Y) {
			if !inside("editor-tabs-thumb", e.X, e.Y) {
				b, _ := cx.ElementBounds("editor-tabs-thumb")
				step := m.tabs.viewport * .8
				if e.X < b.X {
					step = -step
				}
				m.tabs.offset = tabOffset(m.tabs.offset+step, 0, 0, m.tabs.viewport, m.tabs.total, false)
			}
			m.tabs.dragging, m.tabs.anchorX, m.tabs.anchorOffset = true, e.X, m.tabs.offset
			return true
		}
	}
	if e.Kind != ui.KeyPressed || len(m.docs) == 0 {
		return false
	}
	command := e.Modifiers&(ui.ModifierControl|ui.ModifierCommand) != 0
	shift := e.Modifiers&ui.ModifierShift != 0
	if e.Key == 9 && e.Modifiers&ui.ModifierControl != 0 {
		// Freeze the MRU order until Control is released. Focus changes during
		// the gesture must not make repeated Tab oscillate between two files.
		if m.tabs.switchOrder == nil {
			order := append([]*document(nil), m.tabs.mru...)
			for _, d := range m.docs {
				found := false
				for _, entry := range order {
					if d == entry {
						found = true
						break
					}
				}
				if !found {
					order = append([]*document{d}, order...)
				}
			}
			m.tabs.switchOrder = order
			for i, d := range order {
				if d == m.current() {
					m.tabs.switchIndex = i
					break
				}
			}
		}
		order := m.tabs.switchOrder
		if len(order) > 1 {
			step := -1
			if shift {
				step = 1
			}
			m.tabs.switchIndex = (m.tabs.switchIndex + step + len(order)) % len(order)
			m.tabs.cycling = true
			m.focusTab(order[m.tabs.switchIndex])
			m.tabs.cycling = false
		}
		return true
	}
	if command && e.Key == 'W' && !m.terminalFocused {
		m.closeDocumentTab(m.current())
		return true
	}
	if command && (e.Key == 33 || e.Key == 34) {
		step := 1
		if e.Key == 33 {
			step = -1
		}
		m.focusTab(m.docs[(max(0, m.active)+step+len(m.docs))%len(m.docs)])
		return true
	}
	return false
}
