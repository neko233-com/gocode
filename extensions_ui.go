package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"strings"

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
	SaveID     uint64               `json:"saveId,omitempty"`
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
	state := &documentState{Path: d.path, LanguageID: copilotservice.LanguageID(d.path), Version: d.buffer.Version(), Dirty: d.buffer.Dirty(), Selection: d.buffer.Selection(), SaveID: d.saveID}
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

func canonicalRPCPath(ctx context.Context, root, path string) (string, error) {
	if path == "" || len(path) > 64<<10 {
		return "", errors.New("invalid document path")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	physical, err := canonicalPath(path)
	if err != nil {
		return "", err
	}
	return physical, ctx.Err()
}

// Path resolution runs in the RPC worker before onUI. UI document lookup itself
// is purely in-memory, including Windows short-name/symlink aliases.
func resolveExtensionPaths(ctx context.Context, root string, raw json.RawMessage) (json.RawMessage, error) {
	var request map[string]json.RawMessage
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, err
	}
	resolve := func(object map[string]json.RawMessage) error {
		if value, ok := object["path"]; ok {
			var path string
			if err := json.Unmarshal(value, &path); err != nil {
				return err
			}
			physical, err := canonicalRPCPath(ctx, root, path)
			if err != nil {
				return err
			}
			object["path"], _ = json.Marshal(physical)
		}
		return nil
	}
	if err := resolve(request); err != nil {
		return nil, err
	}
	if value, ok := request["documents"]; ok {
		var documents []map[string]json.RawMessage
		if err := json.Unmarshal(value, &documents); err != nil {
			return nil, err
		}
		if len(documents) > 128 {
			return nil, errors.New("edit exceeds 128-document policy")
		}
		for _, document := range documents {
			if err := resolve(document); err != nil {
				return nil, err
			}
		}
		request["documents"], _ = json.Marshal(documents)
	}
	return json.Marshal(request)
}

func onDocumentUI(root string, cx *ui.Context, fn func(context.Context, json.RawMessage) (any, error)) lsp.Handler {
	apply := onUI(cx, fn)
	return func(ctx context.Context, raw json.RawMessage) (any, error) {
		resolved, err := resolveExtensionPaths(ctx, root, raw)
		if err != nil {
			return nil, err
		}
		return apply(ctx, resolved)
	}
}
func (m *model) findDocument(path string) *document {
	path = m.lexicalPath(path)
	if d := m.pathAliases[pathKey(path)]; d != nil {
		return d
	}
	for _, d := range m.docs {
		if d.path == path || (runtime.GOOS == "windows" && strings.EqualFold(d.path, path)) {
			return d
		}
	}
	return nil
}

func (m *model) startExtensions(ctx context.Context, cx *ui.Context, host *extensions.Host, storage string) <-chan struct{} {
	root := m.workspace
	host.Register("workspace/applyEdit", onDocumentUI(root, cx, func(_ context.Context, raw json.RawMessage) (any, error) {
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
		if len(request.Documents) > 128 {
			return nil, errors.New("edit exceeds 128-document policy")
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
	host.Register("workspace/saveDocument", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var request struct {
			Path    string `json:"path"`
			Version int    `json:"version"`
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		physical, err := canonicalRPCPath(ctx, root, request.Path)
		if err != nil {
			return nil, err
		}
		request.Path = physical
		type answer struct {
			value any
			err   error
		}
		result := make(chan answer, 1)
		if !cx.Dispatch(func() {
			if err := ctx.Err(); err != nil {
				result <- answer{err: err}
				return
			}
			if m.closeBusy {
				result <- answer{err: errors.New("workbench is closing; save was not queued")}
				return
			}
			d := m.findDocument(request.Path)
			if d == nil || d.buffer == nil || d.buffer.Version() != request.Version {
				result <- answer{value: map[string]bool{"saved": false}}
				return
			}
			m.requestSave(ctx, []*document{d}, func(err error) {
				if errors.Is(err, errSaveChanged) {
					result <- answer{value: map[string]any{"saved": false, "document": stateOf(d)}}
					return
				}
				if err != nil {
					result <- answer{err: err}
					return
				}
				result <- answer{value: map[string]any{"saved": true, "document": stateOf(d)}}
			})
		}) {
			return nil, errors.New("native workbench closed")
		}
		select {
		case r := <-result:
			return r.value, r.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	host.Register("window/showTextDocument", func(requestContext context.Context, raw json.RawMessage) (any, error) {
		var request struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		type answer struct {
			state *documentState
			err   error
		}
		reply := make(chan answer, 1)
		if !cx.Dispatch(func() {
			m.openThen(requestContext, request.Path, func(d *document, err error) {
				if err == nil && !d.serviceEligible() {
					err = errors.New("document exceeds extension text policy")
				}
				if err != nil {
					reply <- answer{err: err}
					return
				}
				reply <- answer{state: stateOf(d)}
			})
		}) {
			return nil, errors.New("native workbench closed")
		}
		select {
		case result := <-reply:
			if result.err != nil {
				return nil, result.err
			}
			return map[string]any{"document": result.state}, nil
		case <-requestContext.Done():
			return nil, requestContext.Err()
		}
	})
	host.Register("window/setSelection", onDocumentUI(root, cx, func(_ context.Context, raw json.RawMessage) (any, error) {
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
	documents := make([]*documentState, 0, len(m.docs))
	for _, d := range m.docs {
		if state := stateOf(d); state != nil {
			documents = append(documents, state)
		}
	}
	params := map[string]any{"documents": documents, "active": stateOf(m.current()), "storageRoot": storage, "clientCapabilities": map[string]bool{"showDocument": true, "applyEdit": true}}
	queue := startExtensionDocumentSync(ctx,
		func(request context.Context) error { return host.Call(request, "initialize", params, nil) },
		func(request context.Context, job map[string]any) error {
			return host.Call(request, "syncDocument", job, nil)
		},
		func(err error) { cx.Dispatch(func() { m.message = "Extension synchronization: " + err.Error() }) })
	m.awaitExtensions = func(request context.Context) error { return queue.await(request, cx.Dispatch) }
	previous := m.onDocument
	m.onDocument = func(kind string, d *document, change textbuffer.ChangeEvent) {
		if previous != nil {
			previous(kind, d, change)
		}
		params := map[string]any{"kind": kind, "document": documentStateOf(d, kind == "open"), "changes": change.Changes}
		if d != nil && !d.serviceEligible() && kind != "focus" {
			params = map[string]any{"kind": "close", "document": &documentState{Path: d.path}}
		}
		if err := queue.push(extensionSyncJob{params: params}); err != nil && ctx.Err() == nil {
			m.message = err.Error() + "; extension host stopped"
			go host.Close()
		}
	}
	m.bindCompletions(ctx, cx, host)
	return queue.initialized
}
