package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/neko233-com/gocode/internal/copilotservice"
	"github.com/neko233-com/gocode/internal/languageserver"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
	"github.com/neko233-com/godesktop/extensions"
	"github.com/neko233-com/godesktop/lsp"
)

type documentState struct {
	Path       string               `json:"path"`
	Text       *string              `json:"text,omitempty"`
	LanguageID string               `json:"languageId"`
	Version    int                  `json:"version"`
	Dirty      bool                 `json:"dirty"`
	Selection  textbuffer.Selection `json:"selection"`
	Changes    []textbuffer.Change  `json:"changes,omitempty"`
}

func stateOf(d *document) *documentState {
	return documentStateOf(d, true)
}
func documentStateOf(d *document, full bool) *documentState {
	if !d.serviceEligible() {
		return nil
	}
	state := &documentState{Path: d.path, LanguageID: copilotservice.LanguageID(d.path), Version: d.buffer.Version(), Dirty: d.buffer.Dirty(), Selection: d.buffer.Selection()}
	if full {
		text := d.buffer.Text()
		state.Text = &text
	}
	return state
}

func onUI(cx *ui.Context, fn func(context.Context, json.RawMessage) (any, error)) lsp.Handler {
	return func(ctx context.Context, raw json.RawMessage) (any, error) {
		type answer struct {
			value any
			err   error
		}
		result := make(chan answer, 1)
		if !cx.Dispatch(func() {
			if ctx.Err() != nil {
				result <- answer{err: ctx.Err()}
				return
			}
			value, err := fn(ctx, raw)
			result <- answer{value, err}
		}) {
			return nil, errors.New("native workbench closed")
		}
		select {
		case r := <-result:
			return r.value, r.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
func (m *model) findDocument(path string) *document {
	path, _ = filepath.Abs(path)
	for _, d := range m.docs {
		if d.path == path || (runtime.GOOS == "windows" && strings.EqualFold(d.path, path)) {
			return d
		}
	}
	physical, err := canonicalPath(path)
	if err == nil && physical != path {
		for _, d := range m.docs {
			if d.path == physical || (runtime.GOOS == "windows" && strings.EqualFold(d.path, physical)) {
				return d
			}
		}
	}
	return nil
}

func (m *model) startExtensions(ctx context.Context, cx *ui.Context, host *extensions.Host, storage string) <-chan struct{} {
	host.Register("workspace/applyEdit", onUI(cx, func(_ context.Context, raw json.RawMessage) (any, error) {
		var request struct {
			Documents []struct {
				Path    string            `json:"path"`
				Version int               `json:"version"`
				Edits   []textbuffer.Edit `json:"edits"`
			} `json:"documents"`
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		seen := map[*document]bool{}
		// Preflight the complete transaction before mutating any live buffer.
		for _, entry := range request.Documents {
			d := m.findDocument(entry.Path)
			if d == nil || d.buffer == nil || seen[d] || d.buffer.Version() != entry.Version {
				return map[string]bool{"applied": false}, nil
			}
			seen[d] = true
			candidate, _ := textbuffer.New(d.buffer.Text())
			if _, err := candidate.Apply(entry.Edits, nil); err != nil {
				return map[string]bool{"applied": false}, nil
			}
			if len(candidate.Text()) > languageserver.MaxDocumentBytes {
				return map[string]bool{"applied": false}, nil
			}
		}
		states := make([]*documentState, 0, len(request.Documents))
		for _, entry := range request.Documents {
			d := m.findDocument(entry.Path)
			change, err := d.buffer.Apply(entry.Edits, nil)
			if err != nil {
				return nil, err
			}
			m.changed(d, change, nil)
			state := stateOf(d)
			state.Changes = change.Changes
			states = append(states, state)
		}
		return map[string]any{"applied": true, "documents": states}, nil
	}))
	host.Register("workspace/saveDocument", onUI(cx, func(_ context.Context, raw json.RawMessage) (any, error) {
		var request struct {
			Path    string `json:"path"`
			Version int    `json:"version"`
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		d := m.findDocument(request.Path)
		if d == nil || d.buffer == nil || d.buffer.Version() != request.Version {
			return map[string]bool{"saved": false}, nil
		}
		if err := m.saveDocument(d); err != nil {
			return nil, err
		}
		return map[string]any{"saved": true, "document": stateOf(d)}, nil
	}))
	host.Register("window/showTextDocument", onUI(cx, func(_ context.Context, raw json.RawMessage) (any, error) {
		var request struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		m.open(request.Path)
		d := m.findDocument(request.Path)
		if !d.serviceEligible() {
			return nil, errors.New(m.message)
		}
		return map[string]any{"document": stateOf(d)}, nil
	}))
	host.Register("window/setSelection", onUI(cx, func(_ context.Context, raw json.RawMessage) (any, error) {
		var request struct {
			Path      string               `json:"path"`
			Selection textbuffer.Selection `json:"selection"`
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		d := m.findDocument(request.Path)
		if !d.serviceEligible() {
			return nil, errors.New("document is not open")
		}
		if err := d.buffer.SetSelection(request.Selection); err != nil {
			return nil, err
		}
		d.followSelection()
		m.documentEvent("selection", d, textbuffer.ChangeEvent{})
		return true, nil
	}))
	jobs := make(chan map[string]any, 128)
	previous := m.onDocument
	m.onDocument = func(kind string, d *document, change textbuffer.ChangeEvent) {
		if previous != nil {
			previous(kind, d, change)
		}
		params := map[string]any{"kind": kind, "document": documentStateOf(d, kind == "open"), "changes": change.Changes}
		if d != nil && !d.serviceEligible() && kind != "focus" {
			params = map[string]any{"kind": "close", "document": &documentState{Path: d.path}}
		}
		select {
		case jobs <- params:
		case <-ctx.Done():
		default:
			m.message = "Extension document queue overflow; extension host stopped"
			go host.Close()
		}
	}
	documents := make([]*documentState, 0, len(m.docs))
	for _, d := range m.docs {
		if state := stateOf(d); state != nil {
			documents = append(documents, state)
		}
	}
	params := map[string]any{"documents": documents, "active": stateOf(m.current()), "storageRoot": storage, "clientCapabilities": map[string]bool{"showDocument": true, "applyEdit": true}}
	initialized := make(chan struct{})
	m.bindCompletions(ctx, cx, host)
	go func() {
		c, stop := context.WithTimeout(ctx, 10*time.Second)
		err := host.Call(c, "initialize", params, nil)
		stop()
		close(initialized)
		if err != nil {
			cx.Dispatch(func() { m.message = "Extension initialization: " + err.Error() })
			return
		}
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-jobs:
				c, stop := context.WithTimeout(ctx, 10*time.Second)
				err := host.Call(c, "syncDocument", job, nil)
				stop()
				if err != nil {
					cx.Dispatch(func() { m.message = err.Error() })
					return
				}
			}
		}
	}()
	return initialized
}
