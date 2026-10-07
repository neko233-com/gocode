package main

import (
	"context"
	"errors"
	"fmt"

	textbuffer "github.com/neko233-com/godesktop/editor"
)

const maxWorkspaceHistory = 32

type historyMember struct {
	path            string
	tabID, revision uint64
}
type historyGroup struct {
	label   string
	members []historyMember
	undone  bool
}
type historyPrompt struct {
	group  *historyGroup
	active historyMember
}
type workspaceHistory struct {
	groups     []*historyGroup
	prompt     *historyPrompt
	busy       bool
	generation uint64
	submit     func(func(context.Context) error, func(error))
}
type historyTarget struct {
	member   historyMember
	snapshot textbuffer.HistorySnapshot
}

// Groups retain metadata only: no closed buffer or document text is pinned.
// Eviction/splitting leaves each buffer's ordinary bounded history intact.
func (m *model) recordWorkspaceHistory(label string, documents []*document) {
	if len(documents) < 2 || len(documents) > 128 {
		return
	}
	g := &historyGroup{label: label}
	for _, d := range documents {
		m.tabKey(d)
		g.members = append(g.members, historyMember{d.path, d.tabID, d.buffer.HistorySnapshot().UndoRevision()})
	}
	m.history.groups = append(m.history.groups, g)
	if len(m.history.groups) > maxWorkspaceHistory {
		copy(m.history.groups, m.history.groups[1:])
		m.history.groups[len(m.history.groups)-1] = nil
		m.history.groups = m.history.groups[:maxWorkspaceHistory]
	}
}
func (m *model) historyDocument(member historyMember) *document {
	d := m.findDocument(member.path)
	if d == nil || d.buffer == nil || d.tabID != member.tabID || !m.ownsDocument(d) {
		return nil
	}
	return d
}
func historyRevision(s textbuffer.HistorySnapshot, redo bool) uint64 {
	if redo {
		return s.RedoRevision()
	}
	return s.UndoRevision()
}
func (m *model) historyTargets(g *historyGroup, redo bool) ([]historyTarget, error) {
	if g == nil || g.undone != redo {
		return nil, errors.New("workspace history direction changed")
	}
	registered := false
	for _, item := range m.history.groups {
		if item == g {
			registered = true
			break
		}
	}
	if !registered {
		return nil, errors.New("workspace history group expired or was split")
	}
	targets := make([]historyTarget, 0, len(g.members))
	for _, member := range g.members {
		d := m.historyDocument(member)
		if d == nil {
			return nil, errors.New("a grouped document was closed or reopened")
		}
		s := d.buffer.HistorySnapshot()
		if historyRevision(s, redo) != member.revision {
			return nil, errors.New("another grouped file has newer edits")
		}
		targets = append(targets, historyTarget{member, s})
	}
	return targets, nil
}
func (m *model) splitHistory(g *historyGroup) {
	for i, item := range m.history.groups {
		if item == g {
			copy(m.history.groups[i:], m.history.groups[i+1:])
			m.history.groups[len(m.history.groups)-1] = nil
			m.history.groups = m.history.groups[:len(m.history.groups)-1]
			return
		}
	}
}
func (m *model) localHistory(d *document, redo bool) {
	var change textbuffer.ChangeEvent
	var ok bool
	if redo {
		change, ok = d.buffer.Redo()
	} else {
		change, ok = d.buffer.Undo()
	}
	if ok {
		m.changed(d, change, nil)
	}
}
func (m *model) requestHistory(d *document, redo bool) {
	if m.history.busy {
		m.message = "Workspace undo/redo is still preparing"
		return
	}
	if d == nil || d.buffer == nil {
		return
	}
	m.tabKey(d)
	token := historyRevision(d.buffer.HistorySnapshot(), redo)
	for i := len(m.history.groups) - 1; i >= 0; i-- {
		g := m.history.groups[i]
		if g.undone != redo {
			continue
		}
		for _, member := range g.members {
			if member.tabID != d.tabID || pathKey(member.path) != pathKey(d.path) || member.revision != token {
				continue
			}
			if _, err := m.historyTargets(g, redo); err != nil {
				m.splitHistory(g)
				m.localHistory(d, redo)
				m.message = "Changed only this file: " + err.Error()
				return
			}
			if redo {
				m.prepareHistory(g, true)
			} else {
				m.history.prompt = &historyPrompt{g, member}
			}
			return
		}
	}
	m.localHistory(d, redo)
}
func (m *model) confirmHistory(all bool) {
	p := m.history.prompt
	m.history.prompt = nil
	if p == nil {
		return
	}
	if !all {
		d := m.historyDocument(p.active)
		if d == nil || d.buffer.HistorySnapshot().UndoRevision() != p.active.revision {
			m.message = "Undo cancelled: current file changed"
			return
		}
		m.splitHistory(p.group)
		m.localHistory(d, false)
		return
	}
	m.prepareHistory(p.group, false)
}
func (m *model) prepareHistory(g *historyGroup, redo bool) {
	if m.history.submit == nil {
		m.message = "Workspace undo/redo worker is unavailable"
		return
	}
	targets, err := m.historyTargets(g, redo)
	if err != nil {
		m.message = "Workspace undo/redo cancelled: " + err.Error()
		return
	}
	m.history.generation++
	generation := m.history.generation
	m.history.busy = true
	prepared := make([]*textbuffer.PreparedEdit, len(targets))
	m.history.submit(func(ctx context.Context) error {
		for i, target := range targets {
			var err error
			if redo {
				prepared[i], err = target.snapshot.PrepareRedo(ctx)
			} else {
				prepared[i], err = target.snapshot.PrepareUndo(ctx)
			}
			if err != nil {
				return err
			}
		}
		return ctx.Err()
	}, func(err error) {
		if generation != m.history.generation {
			return
		}
		m.history.busy = false
		if err != nil {
			m.message = "Workspace undo/redo cancelled: " + err.Error()
			return
		}
		if _, err := m.historyTargets(g, redo); err != nil {
			m.message = "Workspace undo/redo cancelled: " + err.Error()
			return
		}
		documents := make([]*document, len(targets))
		for i, target := range targets {
			documents[i] = m.historyDocument(target.member)
			if documents[i] == nil || !documents[i].buffer.CanCommit(prepared[i]) {
				m.message = "Workspace undo/redo cancelled: document or caret changed"
				return
			}
		}
		changes := make([]textbuffer.ChangeEvent, len(targets))
		// Complete preflight, then no callbacks between buffer commits.
		for i, d := range documents {
			change, err := d.buffer.CommitPrepared(prepared[i])
			if err != nil {
				panic("preflighted workspace history changed without interleaving: " + err.Error())
			}
			changes[i] = change
		}
		g.undone = !redo
		for i, d := range documents {
			m.changed(d, changes[i], nil)
		}
		action := "Undid"
		if redo {
			action = "Redid"
		}
		m.message = fmt.Sprintf("%s %s in %d files", action, g.label, len(documents))
	})
}
