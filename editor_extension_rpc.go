package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"

	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
	"github.com/neko233-com/godesktop/extensions"
)

type editorOpenRequest struct {
	Path          string              `json:"path"`
	ViewColumn    int                 `json:"viewColumn"`
	PreserveFocus bool                `json:"preserveFocus"`
	Selection     *textbuffer.Range   `json:"selection"`
	Preview       *bool               `json:"preview"`
	FocusReceipt  *editorFocusReceipt `json:"focusReceipt"`
}

type editorFocusReceipt struct {
	Sequence        uint64   `json:"sequence"`
	Group           uint64   `json:"group"`
	GroupGeneration uint64   `json:"groupGeneration"`
	Groups          []uint64 `json:"groups"`
}

func (m *model) editorFocusReceipt() *editorFocusReceipt {
	m.ensureGroups()
	g := m.findGroup(m.groups.active)
	result := &editorFocusReceipt{Sequence: m.openSequence, Group: g.id, GroupGeneration: g.generation}
	for _, group := range m.allGroups() {
		result.Groups = append(result.Groups, group.id)
	}
	return result
}

func (m *model) showExtensionDocument(ctx context.Context, request editorOpenRequest, done func(any, error)) {
	m.ensureGroups()
	groups := m.allGroups()
	origin := m.findGroup(m.groups.active)
	if receipt := request.FocusReceipt; receipt != nil {
		original := m.findGroup(receipt.Group)
		if original == nil || original.generation != receipt.GroupGeneration || !reflect.DeepEqual(receipt.Groups, m.editorFocusReceipt().Groups) || (!request.PreserveFocus && receipt.Sequence != m.openSequence) {
			done(nil, errOpenSuperseded)
			return
		}
		origin = original
	}
	column := request.ViewColumn
	if column == 0 || column == -1 {
		column = slices.Index(groups, origin) + 1
	} else if column == -2 {
		column = slices.Index(groups, origin) + 2
	}
	if column < 1 || column > maxEditorGroups {
		done(nil, errors.New("invalid ViewColumn"))
		return
	}
	var target *editorGroup
	var generation uint64
	if column <= len(groups) {
		target = groups[column-1]
		generation = target.generation
	}
	if !request.PreserveFocus {
		m.openSequence++
	}
	token := m.openSequence
	if m.requestDocumentOpen == nil {
		done(nil, errors.New("native file workers are not initialized"))
		return
	}
	m.requestDocumentOpen(ctx, request.Path, func(d *document, err error) {
		if err = errors.Join(err, ctx.Err()); err != nil {
			done(nil, err)
			return
		}
		if (!request.PreserveFocus && m.openSequence != token) || m.findGroup(origin.id) != origin {
			done(nil, errOpenSuperseded)
			return
		}
		if target != nil {
			if m.findGroup(target.id) != target || target.generation != generation {
				done(nil, errOpenSuperseded)
				return
			}
		} else {
			if !reflect.DeepEqual(groups, m.allGroups()) {
				done(nil, errOpenSuperseded)
				return
			}
		}
		if request.Selection != nil {
			if _, err := d.buffer.RangeText(*request.Selection); err != nil {
				done(nil, err)
				return
			}
		}
		if target == nil {
			target, err = m.editorColumn(column)
			if err != nil {
				done(nil, err)
				return
			}
		}
		m.showGroupDocument(target, d, !request.PreserveFocus)
		state := m.editorLayout(nil, 0)
		id := ""
		for _, entry := range state.Editors {
			if entry.ViewColumn == column && entry.Path == d.path {
				id = entry.ID
			}
		}
		if id == "" {
			done(nil, errors.New("native editor is not visible"))
			return
		}
		if request.Selection != nil {
			selection := textbuffer.Selection{Anchor: request.Selection.Start, Active: request.Selection.End}
			if err := m.setEditorSelection(id, d.path, d.buffer.Version(), selection); err != nil {
				done(nil, err)
				return
			}
			state = m.editorLayout(nil, 3)
		}
		done(map[string]any{"document": stateOf(d), "editors": state, "editorId": id}, nil)
	})
}

func (m *model) registerEditorRPC(ctx context.Context, cx *ui.Context, host *extensions.Host) {
	for _, method := range []string{"workspace/openTextDocument", "window/showTextDocument"} {
		show := method == "window/showTextDocument"
		host.Register(method, func(requestContext context.Context, raw json.RawMessage) (any, error) {
			var request editorOpenRequest
			if err := json.Unmarshal(raw, &request); err != nil {
				return nil, err
			}
			type answer struct {
				value any
				err   error
			}
			reply := make(chan answer, 1)
			if !cx.Dispatch(func() {
				done := func(value any, err error) { reply <- answer{value, err} }
				if err := errors.Join(ctx.Err(), requestContext.Err()); err != nil {
					done(nil, err)
					return
				}
				if show {
					m.showExtensionDocument(requestContext, request, done)
					return
				}
				if m.requestDocumentOpen == nil {
					done(nil, errors.New("native file workers are not initialized"))
					return
				}
				receipt := m.editorFocusReceipt()
				m.requestDocumentOpen(requestContext, request.Path, func(d *document, err error) {
					if err = errors.Join(err, requestContext.Err()); err != nil {
						done(nil, err)
						return
					}
					done(map[string]any{"document": stateOf(d), "editors": m.editorLayout(nil, 0), "focusReceipt": receipt}, nil)
				})
			}) {
				return nil, errors.New("native workbench closed")
			}
			select {
			case result := <-reply:
				return result.value, result.err
			case <-requestContext.Done():
				return nil, requestContext.Err()
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
	}
	host.Register("window/setSelection", onDocumentUI(m.workspace, cx, func(_ context.Context, raw json.RawMessage) (any, error) {
		var request struct {
			Path      string               `json:"path"`
			EditorID  string               `json:"editorId"`
			Version   int                  `json:"version"`
			Selection textbuffer.Selection `json:"selection"`
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		if request.EditorID == "" {
			d := m.findDocument(request.Path)
			if !d.serviceEligible() {
				return nil, errors.New("document is not open")
			}
			if err := d.buffer.SetSelection(request.Selection); err != nil {
				return nil, err
			}
			d.followSelection()
			m.documentEvent("selection", d, textbuffer.ChangeEvent{})
		} else if err := m.setEditorSelection(request.EditorID, request.Path, request.Version, request.Selection); err != nil {
			return nil, err
		}
		return map[string]any{"editors": m.editorLayout(nil, 3)}, nil
	}))
	host.Register("window/revealRange", onUI(cx, func(_ context.Context, raw json.RawMessage) (any, error) {
		var request struct {
			Path       string           `json:"path"`
			EditorID   string           `json:"editorId"`
			Version    int              `json:"version"`
			Range      textbuffer.Range `json:"range"`
			RevealType int              `json:"revealType"`
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		if err := m.revealEditorRange(request.EditorID, request.Path, request.Version, request.Range, request.RevealType); err != nil {
			return nil, err
		}
		return map[string]any{"editors": m.editorLayout(nil, 0)}, nil
	}))
}
