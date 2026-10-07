package main

import (
	"context"
	"fmt"
	"slices"
	"strings"

	textbuffer "github.com/neko233-com/godesktop/editor"
)

const maxEditorGroups = 9

func containsDocument(documents []*document, d *document) bool { return slices.Contains(documents, d) }
func documentIndex(documents []*document, d *document) int     { return slices.Index(documents, d) }

type editorView struct {
	selection            textbuffer.Selection
	line, column, scroll int
	large                *largeDocument
	holdScroll           bool
}
type editorGroup struct {
	id                      uint64
	generation              uint64
	docs                    []*document
	current                 *document
	views                   map[*document]*editorView
	tabs                    *editorTabState
	editorID                uint64
	editorDocument          *document
	visibleRows             int
	rangeBuffer             *textbuffer.Buffer
	rangeVersion, rangeLine int
	rangeEnd                textbuffer.Position
}
type editorSplit struct {
	id          uint64
	group       *editorGroup
	left, right *editorSplit
	down        bool
	ratio       float32
}
type editorGroups struct {
	root             *editorSplit
	active, sequence uint64
	drag             *editorSplit
	pressed          *editorGroup
	pressGeneration  uint64
}

func (m *model) allGroups() []*editorGroup {
	var result []*editorGroup
	var visit func(*editorSplit)
	visit = func(n *editorSplit) {
		if n == nil {
			return
		}
		if n.group != nil {
			result = append(result, n.group)
			return
		}
		visit(n.left)
		visit(n.right)
	}
	visit(m.groups.root)
	return result
}
func (m *model) findGroup(id uint64) *editorGroup {
	if id == 0 {
		id = 1
	}
	for _, g := range m.allGroups() {
		if g.id == id {
			return g
		}
	}
	return nil
}
func groupKey(g *editorGroup, key string) string {
	if g == nil || g.id == 1 {
		return key
	}
	return fmt.Sprintf("group-%d-%s", g.id, key)
}
func (m *model) editorKey(key string) string { return groupKey(m.findGroup(m.groups.active), key) }
func (m *model) activeTabs() *editorTabState {
	if g := m.findGroup(m.groups.active); g != nil {
		return g.tabs
	}
	return &m.tabs
}
func (m *model) groupDocuments() []*document {
	if g := m.findGroup(m.groups.active); g != nil {
		return g.docs
	}
	return m.docs
}
func captureView(d *document) *editorView {
	v := &editorView{line: d.line, column: d.column, scroll: d.scroll, large: d.large, holdScroll: d.holdScroll}
	if d.buffer != nil {
		v.selection = d.buffer.Selection()
	}
	return v
}
func (m *model) ensureGroups() {
	if m.groups.root != nil {
		return
	}
	m.groups.sequence++
	g := &editorGroup{id: m.groups.sequence, docs: slices.Clone(m.docs), current: m.current(), views: map[*document]*editorView{}, tabs: &m.tabs}
	for _, d := range g.docs {
		g.views[d] = captureView(d)
		d.selectionOwner = g.id
		if d.large != nil {
			d.largeBase = d.large
		}
	}
	m.groups.root = &editorSplit{id: g.id, group: g}
	m.groups.active = g.id
}
func (m *model) captureActiveView() {
	g := m.findGroup(m.groups.active)
	if g == nil || g.current == nil {
		return
	}
	d := g.current
	if m.current() != d {
		return
	}
	*vOrCreate(g, d) = *captureView(d)
}
func vOrCreate(g *editorGroup, d *document) *editorView {
	if v := g.views[d]; v != nil {
		return v
	}
	v := captureView(d)
	g.views[d] = v
	return v
}
func restoreView(d *document, v *editorView, owner uint64) {
	d.line, d.column, d.scroll = v.line, v.column, v.scroll
	d.holdScroll = v.holdScroll
	if v.large != nil {
		d.large = v.large
	}
	if d.buffer != nil {
		if err := d.buffer.SetSelection(v.selection); err != nil {
			p := d.buffer.PositionFromRunes(max(0, min(v.selection.Active.Line, d.buffer.LineCount()-1)), 0)
			_ = d.buffer.SetSelection(textbuffer.Selection{Anchor: p, Active: p})
		}
		p := d.buffer.Selection().Active
		d.line = p.Line
		d.column, _ = d.buffer.RuneColumn(p)
	}
	d.selectionOwner = owner
}
func (m *model) addGroupDocument(g *editorGroup, d *document) {
	if g == nil || slices.Contains(g.docs, d) {
		return
	}
	g.docs = append(g.docs, d)
	g.generation++
	g.views[d] = captureView(d)
	if d.large != nil {
		if d.largeBase == nil {
			d.largeBase = d.large
		}
		// A second view owns request/page state, sharing only the concurrent file.
		if m.groupReferences(d) > 1 {
			g.views[d].large = cloneLargeView(d.largeBase, d.large)
		}
	}
}
func cloneLargeView(base, current *largeDocument) *largeDocument {
	ctx, cancel := context.WithCancel(base.ctx)
	return &largeDocument{file: base.file, ctx: ctx, cancel: cancel, shared: true, start: current.start, byteOffset: current.byteOffset, byteMode: current.byteMode, stats: current.stats, page: current.page, window: current.window}
}
func (m *model) groupReferences(d *document) int {
	count := 0
	for _, g := range m.allGroups() {
		if slices.Contains(g.docs, d) {
			count++
		}
	}
	return count
}
func (m *model) restoreActiveGroup() {
	g := m.findGroup(m.groups.active)
	if g == nil {
		groups := m.allGroups()
		if len(groups) == 0 {
			return
		}
		g = groups[0]
		m.groups.active = g.id
	}
	if !slices.Contains(g.docs, g.current) {
		g.current = nil
		if len(g.docs) > 0 {
			g.current = g.docs[len(g.docs)-1]
		}
	}
	m.active = -1
	if g.current != nil {
		m.active = slices.Index(m.docs, g.current)
		restoreView(g.current, vOrCreate(g, g.current), g.id)
	}
}
func (m *model) focusGroup(id uint64) {
	g := m.findGroup(id)
	if g == nil {
		return
	}
	m.captureActiveView()
	m.endTabSwitch()
	m.groups.active = g.id
	m.restoreActiveGroup()
	m.editing = true
	m.search.focus = -2
	m.terminalFocused = false
	m.chatFocused = false
	m.updateFocused = false
	m.pointerSelecting = false
	m.documentEvent("focus", m.current(), textbuffer.ChangeEvent{})
}
func (m *model) splitEditor(down bool) {
	next := m.splitEditorGroup(m.groups.active, down, true)
	if next != nil {
		m.focusGroup(next.id)
	}
}
func (m *model) splitEditorGroup(groupID uint64, down, duplicate bool) *editorGroup {
	m.ensureGroups()
	m.captureActiveView()
	if len(m.allGroups()) >= maxEditorGroups {
		m.message = "At most 9 editor groups can be open"
		return nil
	}
	g := m.findGroup(groupID)
	if g == nil {
		return nil
	}
	m.groups.sequence++
	next := &editorGroup{id: m.groups.sequence, views: map[*document]*editorView{}, tabs: &editorTabState{}}
	if duplicate && g.current != nil {
		m.addGroupDocument(next, g.current)
		next.current = g.current
		if g.current.large != nil {
			next.views[g.current].large = cloneLargeView(g.current.largeBase, g.current.large)
		}
	}
	var split func(*editorSplit)
	split = func(n *editorSplit) {
		if n.group == g {
			old := *n
			n.group = nil
			n.down = down
			n.ratio = .5
			n.left = &old
			n.right = &editorSplit{id: next.id, group: next}
			return
		}
		if n.group == nil {
			split(n.left)
			split(n.right)
		}
	}
	split(m.groups.root)
	return next
}
func (m *model) forgetGroupDocument(d *document) {
	for _, g := range m.allGroups() {
		if v := g.views[d]; v != nil && v.large != nil && v.large != d.largeBase {
			v.large.cancel()
			go v.large.close()
		}
		delete(g.views, d)
		if i := slices.Index(g.docs, d); i >= 0 {
			g.docs = slices.Delete(g.docs, i, i+1)
			g.generation++
		}
		if g.current == d {
			g.current = nil
			if len(g.docs) > 0 {
				g.current = g.docs[len(g.docs)-1]
			}
		}
	}
}
func (m *model) closeGroupView(d *document) bool {
	g := m.findGroup(m.groups.active)
	if g == nil || m.groupReferences(d) <= 1 {
		return false
	}
	m.captureActiveView()
	if v := g.views[d]; v != nil && v.large != nil && v.large != d.largeBase {
		v.large.cancel()
		go v.large.close()
	}
	delete(g.views, d)
	g.generation++
	if i := slices.Index(g.docs, d); i >= 0 {
		g.docs = slices.Delete(g.docs, i, i+1)
	}
	if g.current == d {
		g.current = nil
		if len(g.docs) > 0 {
			g.current = g.docs[len(g.docs)-1]
		}
	}
	if len(g.docs) == 0 && len(m.allGroups()) > 1 {
		m.dropEditorGroup(g.id)
		return true
	}
	m.restoreActiveGroup()
	m.documentEvent("focus", m.current(), textbuffer.ChangeEvent{})
	return true
}
func (m *model) requestCloseGroup(id uint64) {
	g := m.findGroup(id)
	if g == nil {
		return
	}
	for _, d := range g.docs {
		if d.dirty() && m.groupReferences(d) == 1 {
			m.beginClose(nil)
			m.closeGroupID = id
			return
		}
	}
	m.dropEditorGroup(id)
}
func (m *model) dropEditorGroup(id uint64) {
	g := m.findGroup(id)
	if g == nil {
		return
	}
	m.captureActiveView()
	unique := []*document{}
	for _, d := range g.docs {
		if m.groupReferences(d) == 1 {
			unique = append(unique, d)
		}
		if v := g.views[d]; v != nil && v.large != nil && v.large != d.largeBase {
			v.large.cancel()
			go v.large.close()
		}
	}
	var remove func(*editorSplit) *editorSplit
	remove = func(n *editorSplit) *editorSplit {
		if n.group != nil {
			if n.group == g {
				return nil
			}
			return n
		}
		n.left = remove(n.left)
		n.right = remove(n.right)
		if n.left == nil {
			return n.right
		}
		if n.right == nil {
			return n.left
		}
		return n
	}
	next := remove(m.groups.root)
	if g.tabs == &m.tabs {
		sequence := m.tabs.sequence
		m.tabs = editorTabState{sequence: sequence}
	}
	if next == nil {
		g.docs = nil
		g.current = nil
		clear(g.views)
		m.groups.root = &editorSplit{id: g.id, group: g}
	} else {
		m.groups.root = next
	}
	for _, d := range unique {
		if i := slices.Index(m.docs, d); i >= 0 {
			m.removeTab(i)
		}
	}
	m.groups.drag = nil
	m.restoreActiveGroup()
	m.documentEvent("focus", m.current(), textbuffer.ChangeEvent{})
}
func (m *model) closeScope(d *document) bool {
	if m.closeTarget != nil {
		return m.closeTarget == d
	}
	if m.closeGroupID != 0 {
		g := m.findGroup(m.closeGroupID)
		return g != nil && slices.Contains(g.docs, d) && m.groupReferences(d) == 1
	}
	return true
}

func (m *model) pruneEmptyGroups() {
	for {
		groups := m.allGroups()
		if len(groups) <= 1 {
			return
		}
		removed := false
		for _, g := range groups {
			if len(g.docs) == 0 {
				m.dropEditorGroup(g.id)
				removed = true
				break
			}
		}
		if !removed {
			return
		}
	}
}

func beforePosition(a, b textbuffer.Position) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Character < b.Character
}
func changeEnd(c textbuffer.Change) textbuffer.Position {
	p := c.Range.Start
	for _, r := range c.Text {
		if r == '\r' {
			continue
		}
		if r == '\n' {
			p.Line++
			p.Character = 0
		} else {
			p.Character++
			if r > 0xffff {
				p.Character++
			}
		}
	}
	return p
}
func moveViewPosition(p textbuffer.Position, c textbuffer.Change, end textbuffer.Position) textbuffer.Position {
	if beforePosition(p, c.Range.Start) {
		return p
	}
	if !beforePosition(c.Range.End, p) {
		return end
	}
	if p.Line == c.Range.End.Line {
		return textbuffer.Position{Line: end.Line, Character: end.Character + p.Character - c.Range.End.Character}
	}
	return textbuffer.Position{Line: p.Line + end.Line - c.Range.End.Line, Character: p.Character}
}
func (m *model) groupDocumentEvent(kind string, d *document, change textbuffer.ChangeEvent) {
	if d == nil || m.groups.root == nil {
		return
	}
	if kind == "selection" {
		m.captureActiveView()
		return
	}
	if kind != "change" || d.buffer == nil {
		return
	}
	ends := make([]textbuffer.Position, len(change.Changes))
	for i, c := range change.Changes {
		ends[i] = changeEnd(c)
	}
	for _, g := range m.allGroups() {
		v := g.views[d]
		if v == nil {
			continue
		}
		if g.id == d.selectionOwner {
			*v = *captureView(d)
			continue
		}
		for i, c := range change.Changes {
			v.selection.Anchor = moveViewPosition(v.selection.Anchor, c, ends[i])
			v.selection.Active = moveViewPosition(v.selection.Active, c, ends[i])
		}
		v.line = v.selection.Active.Line
		v.column, _ = d.buffer.RuneColumn(v.selection.Active)
		v.scroll = max(0, min(v.scroll, d.buffer.LineCount()-1))
	}
}
func (m *model) groupBreadcrumb(d *document) string {
	if d == nil {
		return "Welcome"
	}
	return strings.TrimPrefix(strings.ReplaceAll(d.path, "\\", "/"), strings.ReplaceAll(m.workspace, "\\", "/")+"/")
}
