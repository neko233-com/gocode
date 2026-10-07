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
	m.captureActiveView()
	for i, candidate := range m.docs {
		if candidate == d {
			if g := m.findGroup(m.groups.active); g != nil {
				m.addGroupDocument(g, d)
				if g.current != d {
					g.generation++
				}
				g.current = d
				restoreView(d, vOrCreate(g, d), g.id)
			}
			m.active = i
			m.editing = true
			m.search.focus = -2
			m.terminalFocused, m.chatFocused, m.updateFocused = false, false, false
			m.pointerSelecting = false
			m.activeTabs().dragging = false
			m.documentEvent("focus", d, textbuffer.ChangeEvent{})
			return
		}
	}
}
func (m *model) closeDocumentTab(d *document) {
	if m.closeGroupView(d) {
		return
	}
	for i, candidate := range m.docs {
		if candidate == d {
			m.closeTab(i)
			return
		}
	}
}

func (m *model) recordTabEvent(kind string, d *document) {
	if kind == "close" && m.groups.root != nil {
		for _, g := range m.allGroups() {
			m.recordTabEventFor(g.tabs, kind, d)
		}
		m.recordTabEventFor(&m.tabs, kind, d)
		return
	}
	m.recordTabEventFor(m.activeTabs(), kind, d)
}
func (m *model) recordTabEventFor(state *editorTabState, kind string, d *document) {
	if kind != "focus" && kind != "close" {
		return
	}
	if !state.cycling {
		m.endTabSwitchFor(state)
	}
	kept := state.mru[:0]
	previousLength := len(state.mru)
	for _, entry := range state.mru {
		if entry != d {
			kept = append(kept, entry)
		}
	}
	clear(state.mru[len(kept):previousLength])
	if kind == "focus" && d != nil {
		kept = append(kept, d)
		state.reveal = true
	}
	state.mru = kept
}

func (m *model) endTabSwitch() {
	m.endTabSwitchFor(m.activeTabs())
}
func (m *model) endTabSwitchFor(state *editorTabState) {
	clear(state.switchOrder)
	state.switchOrder = nil
	state.switchIndex = 0
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
	return m.tabsViewFor(m.findGroup(m.groups.active), m.groupDocuments(), m.activeTabs(), m.current(), max(0, width-290))
}
func (m *model) tabsViewFor(g *editorGroup, documents []*document, state *editorTabState, current *document, width float32) *ui.Element {
	actions := float32(30)
	if g != nil {
		actions = 90
	}
	viewport := max(0, width-actions)
	prefix := make([]float32, len(documents)+1)
	for i, d := range documents {
		if d.tabWidth == 0 {
			w, _ := ui.MeasureText(filepath.Base(d.path), 13, "")
			d.tabWidth = max(120, w+64)
		}
		prefix[i+1] = prefix[i] + d.tabWidth + 1
	}
	total := prefix[len(documents)]
	reveal := state.reveal || state.lastActive != current || state.lastCount != len(documents) || state.viewport != viewport
	start, end := float32(0), float32(0)
	for i, d := range documents {
		if d == current {
			start, end = prefix[i], prefix[i+1]
			break
		}
	}
	state.offset = tabOffset(state.offset, start, end, viewport, total, reveal)
	state.viewport, state.total = viewport, total
	state.lastActive, state.lastCount, state.reveal = current, len(documents), false
	first := sort.Search(len(documents), func(i int) bool { return prefix[i+1] > state.offset })
	last := sort.Search(len(documents), func(i int) bool { return prefix[i] >= state.offset+viewport })
	entries := []*ui.Element{ui.Column().Width(prefix[first])}
	for i := first; i < last; i++ {
		d := documents[i]
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
		activate := func() bool {
			if g != nil {
				if m.findGroup(g.id) != g || !containsDocument(g.docs, d) {
					return false
				}
				if m.groups.active != g.id {
					m.focusGroup(g.id)
				}
			}
			return m.ownsDocument(d)
		}
		row := ui.Row(label("{} ").Foreground(ui.RGB(0x519aba)).Width(28), label(filepath.Base(d.path)).Flex(1), button(symbol, groupKey(g, m.tabCloseKey(d)), func(*ui.Context) {
			if activate() {
				m.closeDocumentTab(d)
			}
		}).Width(26)).Padding(5).Height(34)
		tab := ui.Column(ui.Column().Height(1).Background(top), row).Width(d.tabWidth).Background(ui.RGB(bg)).Key(groupKey(g, m.tabKey(d))).OnClick(func(*ui.Context) {
			if activate() {
				m.focusTab(d)
			}
		})
		entries = append(entries, tab, ui.Column().Width(1).Background(ui.RGB(border)))
	}
	entries = append(entries, ui.Column().Width(total-prefix[last]))
	content := ui.Row(entries...).Width(total).Height(35)
	layers := []*ui.Element{ui.Viewport(content).ScrollOffset(state.offset, 0).Key(groupKey(g, "editor-tabs"))}
	if total > viewport && viewport > 0 {
		thumb := min(viewport, max(24, viewport*viewport/total))
		left := state.offset / (total - viewport) * (viewport - thumb)
		track := ui.Row(ui.Column().Width(left), ui.Column().Width(thumb).Background(ui.RGB(0x555555)).Key(groupKey(g, "editor-tabs-thumb")), spacer()).Height(3).Key(groupKey(g, "editor-tabs-track"))
		layers = append(layers, ui.Column(spacer(), track))
	}
	split := func(down bool) {
		if !m.acceptGroupAction(g) {
			return
		}
		if g != nil {
			if m.findGroup(g.id) != g {
				return
			}
			m.focusGroup(g.id)
		}
		m.splitEditor(down)
	}
	buttons := []*ui.Element{icon("split", groupKey(g, "split"), func(*ui.Context) { split(false) })}
	if g != nil {
		buttons = append(buttons, button("↧", groupKey(g, "split-down"), func(*ui.Context) { split(true) }).Width(30), icon("close", groupKey(g, "close-group"), func(*ui.Context) {
			if m.acceptGroupAction(g) {
				m.requestCloseGroup(g.id)
			}
		}))
	}
	return ui.Row(ui.Stack(layers...).Flex(1), ui.Row(buttons...).Width(actions)).Height(35).Background(ui.RGB(outer))
}

func (m *model) tabInput(cx *ui.Context, e ui.InputEvent) bool {
	state := m.activeTabs()
	documents := m.groupDocuments()
	if state.dragging {
		switch e.Kind {
		case ui.PointerMoved:
			travel := state.viewport - min(state.viewport, max(24, state.viewport*state.viewport/state.total))
			if travel > 0 {
				state.offset = tabOffset(state.anchorOffset+(e.X-state.anchorX)*(state.total-state.viewport)/travel, 0, 0, state.viewport, state.total, false)
			}
			return true
		case ui.PointerReleased, ui.InputCancelled:
			state.dragging = false
			return true
		}
	}
	if cx != nil {
		inside := func(key string, x, y float32) bool {
			b, ok := cx.ElementBounds(key)
			return ok && x >= b.X && x < b.X+b.Width && y >= b.Y && y < b.Y+b.Height
		}
		if e.Kind == ui.Scroll && inside(m.editorKey("editor-tabs"), e.PointerX, e.PointerY) {
			delta := e.X
			if delta == 0 {
				delta = -e.Y
			}
			state.offset = tabOffset(state.offset+delta*40, 0, 0, state.viewport, state.total, false)
			return true
		}
		if e.Kind == ui.PointerPressed && inside(m.editorKey("editor-tabs-track"), e.X, e.Y) {
			if !inside(m.editorKey("editor-tabs-thumb"), e.X, e.Y) {
				b, _ := cx.ElementBounds(m.editorKey("editor-tabs-thumb"))
				step := state.viewport * .8
				if e.X < b.X {
					step = -step
				}
				state.offset = tabOffset(state.offset+step, 0, 0, state.viewport, state.total, false)
			}
			state.dragging, state.anchorX, state.anchorOffset = true, e.X, state.offset
			return true
		}
	}
	if e.Kind != ui.KeyPressed || len(documents) == 0 {
		return false
	}
	command := e.Modifiers&(ui.ModifierControl|ui.ModifierCommand) != 0
	shift := e.Modifiers&ui.ModifierShift != 0
	if e.Key == 9 && e.Modifiers&ui.ModifierControl != 0 {
		// Freeze the MRU order until Control is released. Focus changes during
		// the gesture must not make repeated Tab oscillate between two files.
		if state.switchOrder == nil {
			order := make([]*document, 0, len(documents))
			for _, d := range state.mru {
				if containsDocument(documents, d) {
					order = append(order, d)
				}
			}
			for _, d := range documents {
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
			state.switchOrder = order
			for i, d := range order {
				if d == m.current() {
					state.switchIndex = i
					break
				}
			}
		}
		order := state.switchOrder
		if len(order) > 1 {
			step := -1
			if shift {
				step = 1
			}
			state.switchIndex = (state.switchIndex + step + len(order)) % len(order)
			state.cycling = true
			m.focusTab(order[state.switchIndex])
			state.cycling = false
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
		m.focusTab(documents[(max(0, documentIndex(documents, m.current()))+step+len(documents))%len(documents)])
		return true
	}
	return false
}
