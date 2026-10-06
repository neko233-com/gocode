package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gocode/internal/languageserver"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

func (d *document) serviceEligible() bool {
	if d == nil || d.buffer == nil {
		return false
	}
	if d.serviceVersion != d.buffer.Version() {
		d.serviceBytes = max(0, d.buffer.LineCount()-1) * len(d.buffer.EOL())
		for i := 0; i < d.buffer.LineCount(); i++ {
			d.serviceBytes += len(d.buffer.Line(i))
		}
		d.serviceVersion = d.buffer.Version()
	}
	return d.serviceBytes <= languageserver.MaxDocumentBytes
}

func (m *model) startLanguages(parent context.Context, cx *ui.Context, configs []languageserver.Config) func() {
	ctx, cancel := context.WithCancel(parent)
	var workers sync.WaitGroup
	for _, config := range configs {
		config := config
		workers.Go(func() {
			client, err := languageserver.Start(ctx, m.workspace, config)
			if err != nil {
				cx.Dispatch(func() { m.lspStatus = config.Name + ": " + err.Error() })
				return
			}
			defer client.Close()
			jobs := make(chan func(context.Context) error, 128)
			enqueue := func(job func(context.Context) error) {
				select {
				case jobs <- job:
				case <-ctx.Done():
				default:
					m.message = config.Name + " document queue overflow; server stopped"
					cancel()
				}
			}
			cx.Dispatch(func() {
				m.lspStatus = config.Name + " ready"
				previous := m.onDocument
				m.onDocument = func(kind string, d *document, change textbuffer.ChangeEvent) {
					if previous != nil {
						previous(kind, d, change)
					}
					if d == nil || !config.Supports(d.path) {
						return
					}
					path := d.path
					if kind == "close" || !d.serviceEligible() {
						enqueue(func(c context.Context) error { return client.CloseDocument(c, path) })
						delete(m.diagnostics, "lsp:"+config.Name+"\x00"+path)
						return
					}
					snapshot := d.buffer.Snapshot()
					switch kind {
					case "open", "change", "focus":
						enqueue(func(c context.Context) error { return client.Sync(c, path, snapshot, &change) })
					case "save":
						enqueue(func(c context.Context) error {
							if err := client.Sync(c, path, snapshot, nil); err != nil {
								return err
							}
							return client.Save(c, path, snapshot)
						})
					}
				}
				for _, d := range m.docs {
					m.onDocument("open", d, textbuffer.ChangeEvent{})
				}
				previousRequest := m.requestLSP
				m.requestLSP = func(d *document, method string) {
					if !d.serviceEligible() {
						m.message = "Language services use source snapshots up to 2 MiB; large files remain available for native browsing"
						return
					}
					if !config.Supports(d.path) || !client.Supports(method) {
						if previousRequest != nil {
							previousRequest(d, method)
						} else {
							m.message = "No configured language server advertises " + method
						}
						return
					}
					path, snapshot, position, generation := d.path, d.buffer.Snapshot(), d.cursor(), m.inlineGeneration
					workers.Go(func() {
						requestCtx, stop := context.WithTimeout(ctx, 15*time.Second)
						defer stop()
						var raw json.RawMessage
						err := client.Request(requestCtx, path, snapshot, position, method, &raw)
						cx.Dispatch(func() {
							current := m.findDocument(path)
							if current == nil || !current.serviceEligible() || current.buffer.Version() != snapshot.Version || ctx.Err() != nil {
								return
							}
							if method != "textDocument/formatting" && (current != m.current() || current.cursor() != position || generation != m.inlineGeneration) {
								return
							}
							if err != nil {
								m.message = config.Name + ": " + err.Error()
								return
							}
							m.applyLSPResult(config.Name, method, current, snapshot.Version, position, raw)
						})
					})
				}
				previousCompletion := m.requestCompletions
				m.requestCompletions = func(d *document) {
					if previousCompletion != nil {
						previousCompletion(d)
					}
					if d.serviceEligible() && config.Supports(d.path) {
						m.requestLSP(d, "textDocument/completion")
					}
				}
			})
			for {
				select {
				case <-ctx.Done():
					return
				case job := <-jobs:
					c, stop := context.WithTimeout(ctx, 10*time.Second)
					err := job(c)
					stop()
					if err != nil && ctx.Err() == nil {
						cx.Dispatch(func() { m.lspStatus = config.Name + ": " + err.Error() })
					}
				case event, ok := <-client.RPC.Notifications():
					if !ok {
						cx.Dispatch(func() { m.lspStatus = config.Name + " stopped" })
						return
					}
					if event.Method == "textDocument/publishDiagnostics" {
						var p languageserver.PublishDiagnostics
						if json.Unmarshal(event.Params, &p) != nil {
							continue
						}
						path, err := languageserver.PathFromURI(p.URI)
						if err != nil {
							continue
						}
						cx.Dispatch(func() {
							d := m.findDocument(path)
							if d == nil || !d.serviceEligible() || (p.Version != nil && *p.Version != d.buffer.Version()) {
								return
							}
							var items []diagnostic
							for _, item := range p.Diagnostics[:min(2000, len(p.Diagnostics))] {
								items = append(items, diagnostic{Range: item.Range, Message: item.Message, Severity: max(0, min(3, item.Severity-1)), Path: path})
							}
							if m.diagnostics == nil {
								m.diagnostics = map[string][]diagnostic{}
							}
							m.diagnostics["lsp:"+config.Name+"\x00"+path] = items
						})
					}
				}
			}
		})
	}
	return func() { cancel(); workers.Wait() }
}

func (m *model) applyLSPResult(name, method string, d *document, version int, position textbuffer.Position, raw json.RawMessage) {
	switch method {
	case "textDocument/completion":
		items, err := languageserver.Completions(raw)
		if err != nil {
			m.message = err.Error()
			return
		}
		var suggestions []completionSuggestion
		for _, item := range items {
			if item.InsertTextFormat == 2 {
				continue
			} // No snippet placeholders are inserted as plain code.
			value := completionItem{Label: item.Label, InsertText: item.InsertText}
			if len(item.TextEdit) > 0 && string(item.TextEdit) != "null" {
				var edit struct {
					Range   *textbuffer.Range `json:"range"`
					Insert  *textbuffer.Range `json:"insert"`
					NewText string            `json:"newText"`
				}
				if json.Unmarshal(item.TextEdit, &edit) != nil {
					continue
				}
				r := edit.Range
				if r == nil {
					r = edit.Insert
				}
				if r == nil {
					continue
				}
				value.TextEdit = &textbuffer.Edit{Range: *r, Text: edit.NewText}
			}
			for _, edit := range item.AdditionalTextEdits {
				value.AdditionalTextEdits = append(value.AdditionalTextEdits, textbuffer.Edit{Range: edit.Range, Text: edit.NewText})
			}
			if strings.TrimSpace(value.title()) != "" {
				suggestions = append(suggestions, completionSuggestion{d.path, version, position, value})
			}
			if len(suggestions) >= 8 {
				break
			}
		}
		m.publishCompletions("lsp:"+name, suggestions)
	case "textDocument/formatting":
		var edits []textbuffer.Edit
		if err := json.Unmarshal(raw, &edits); err != nil {
			m.message = err.Error()
			return
		}
		if err := m.applyDocumentEdits(d.path, version, edits); err != nil {
			m.message = err.Error()
		} else {
			m.message = "Formatted with " + name
		}
	case "textDocument/hover":
		value := languageserver.HoverText(raw)
		if value == "" {
			m.message = "No hover information"
			return
		}
		m.output = append(m.output, strings.Split(value, "\n")...)
		if len(m.output) > 200 {
			m.output = m.output[len(m.output)-200:]
		}
		m.panel = "OUTPUT"
		m.showPanel = true
	case "textDocument/definition":
		locations, err := languageserver.Locations(raw)
		if err != nil {
			m.message = err.Error()
			return
		}
		if len(locations) == 0 {
			m.message = "No definition found"
			return
		}
		path, err := languageserver.PathFromURI(locations[0].URI)
		if err != nil {
			m.message = err.Error()
			return
		}
		m.open(path)
		if target := m.findDocument(path); target != nil && target.buffer != nil {
			column, err := target.buffer.RuneColumn(locations[0].Range.Start)
			if err == nil {
				m.moveCursor(target, locations[0].Range.Start.Line, column, false)
			}
			m.editing = true
		}
		if len(locations) > 1 {
			m.message = fmt.Sprintf("Opened first of %d definitions", len(locations))
		}
	}
}
