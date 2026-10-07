package main

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"

	textbuffer "github.com/neko233-com/godesktop/editor"
)

type nativeEditorState struct {
	ID            string               `json:"id"`
	Path          string               `json:"path"`
	ViewColumn    int                  `json:"viewColumn"`
	Selection     textbuffer.Selection `json:"selection"`
	VisibleRanges []textbuffer.Range   `json:"visibleRanges"`
}
type editorLayoutState struct {
	Generation    uint64              `json:"generation"`
	Active        string              `json:"active"`
	Editors       []nativeEditorState `json:"editors"`
	SelectionKind int                 `json:"selectionKind,omitempty"`
}

func (m *model) editorLayout(exclude *document, selectionKind int) *editorLayoutState {
	m.ensureGroups()
	m.captureActiveView()
	state := &editorLayoutState{Editors: []nativeEditorState{}}
	for index, g := range m.allGroups() {
		d := g.current
		if d == exclude || !d.serviceEligible() {
			g.editorDocument = nil
			g.editorID = 0
			g.rangeBuffer = nil
			continue
		}
		if g.editorDocument != d {
			m.editorSequence++
			g.editorID = m.editorSequence
			g.editorDocument = d
		}
		v := vOrCreate(g, d)
		first := max(0, min(v.scroll, d.buffer.LineCount()-1))
		last := min(d.buffer.LineCount()-1, first+max(1, g.visibleRows)-1)
		if g.rangeBuffer != d.buffer || g.rangeVersion != d.buffer.Version() || g.rangeLine != last {
			g.rangeBuffer = d.buffer
			g.rangeVersion = d.buffer.Version()
			g.rangeLine = last
			g.rangeEnd = d.buffer.PositionFromRunes(last, d.serviceBytes)
		}
		end := g.rangeEnd
		entry := nativeEditorState{ID: strconv.FormatUint(g.editorID, 10), Path: d.path, ViewColumn: index + 1, Selection: v.selection, VisibleRanges: []textbuffer.Range{{Start: textbuffer.Position{Line: first}, End: end}}}
		state.Editors = append(state.Editors, entry)
		if g.id == m.groups.active {
			state.Active = entry.ID
		}
	}
	previous := m.lastEditors
	if previous == nil || previous.Active != state.Active || !reflect.DeepEqual(previous.Editors, state.Editors) {
		m.editorGeneration++
		state.Generation = m.editorGeneration
		state.SelectionKind = selectionKind
		m.lastEditors = state
		return state
	}
	return previous
}

func (m *model) editorByID(id, path string) (*editorGroup, *document, error) {
	for _, g := range m.allGroups() {
		if strconv.FormatUint(g.editorID, 10) == id && g.editorID != 0 && g.current == g.editorDocument && g.current.serviceEligible() && pathKey(g.current.path) == pathKey(m.lexicalPath(path)) {
			return g, g.current, nil
		}
	}
	return nil, nil, errors.New("native editor is no longer visible")
}

func (m *model) editorColumn(column int) (*editorGroup, error) {
	m.ensureGroups()
	groups := m.allGroups()
	active := m.findGroup(m.groups.active)
	if column == 0 || column == -1 {
		return active, nil
	}
	if column == -2 {
		column = slices.Index(groups, active) + 2
	}
	if column < 1 || column > maxEditorGroups {
		return nil, fmt.Errorf("viewColumn must be Active, Beside or 1–%d", maxEditorGroups)
	}
	for len(groups) < column {
		if m.splitEditorGroup(groups[len(groups)-1].id, false, false) == nil {
			return nil, errors.New("editor group limit reached")
		}
		groups = m.allGroups()
	}
	return groups[column-1], nil
}

func (m *model) showGroupDocument(g *editorGroup, d *document, focus bool) {
	m.captureActiveView()
	m.addGroupDocument(g, d)
	if g.current != d {
		g.generation++
	}
	g.current = d
	if focus {
		m.groups.active = g.id
		m.restoreActiveGroup()
		m.focusTab(d)
	} else if g.id == m.groups.active {
		m.restoreActiveGroup()
		m.documentEvent("focus", d, textbuffer.ChangeEvent{})
	} else if m.publishEditors != nil {
		m.publishEditors(0)
	}
}

func (m *model) setEditorSelection(id, path string, version int, selection textbuffer.Selection) error {
	g, d, err := m.editorByID(id, path)
	if err != nil {
		return err
	}
	if d.buffer.Version() != version {
		return errors.New("document changed before selection")
	}
	for _, p := range []textbuffer.Position{selection.Anchor, selection.Active} {
		if _, err := d.buffer.OffsetAt(p); err != nil {
			return err
		}
	}
	v := vOrCreate(g, d)
	v.selection = selection
	v.line = selection.Active.Line
	v.column, _ = d.buffer.RuneColumn(selection.Active)
	if g.id == m.groups.active {
		if err := d.buffer.SetSelection(selection); err != nil {
			return err
		}
		d.followSelection()
		m.editorSelectionKind = 3
		m.documentEvent("selection", d, textbuffer.ChangeEvent{})
	} else if m.publishEditors != nil {
		m.publishEditors(3)
	}
	return nil
}

func (m *model) revealEditorRange(id, path string, version int, r textbuffer.Range, kind int) error {
	g, d, err := m.editorByID(id, path)
	if err != nil {
		return err
	}
	if d.buffer.Version() != version {
		return errors.New("document changed before reveal")
	}
	if kind < 0 || kind > 3 {
		return errors.New("invalid TextEditorRevealType")
	}
	if _, err := d.buffer.RangeText(r); err != nil {
		return err
	}
	v := vOrCreate(g, d)
	rows := max(1, g.visibleRows)
	v.holdScroll = true
	outside := r.Start.Line < v.scroll || r.End.Line >= v.scroll+rows
	switch kind {
	case 0:
		if outside {
			if r.Start.Line < v.scroll {
				v.scroll = r.Start.Line
			} else {
				v.scroll = max(r.Start.Line, r.End.Line-rows+1)
			}
		}
	case 1:
		v.scroll = max(0, (r.Start.Line+r.End.Line-rows)/2)
	case 2:
		if outside {
			v.scroll = max(0, (r.Start.Line+r.End.Line-rows)/2)
		}
	case 3:
		v.scroll = r.Start.Line
	}
	v.scroll = min(v.scroll, max(0, d.buffer.LineCount()-1))
	if g.id == m.groups.active {
		d.scroll = v.scroll
		d.holdScroll = true
	}
	if m.publishEditors != nil {
		m.publishEditors(0)
	}
	return nil
}
