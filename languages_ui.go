package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/neko233-com/gocode/internal/languageserver"
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
		m.openThen(context.Background(), path, func(target *document, openErr error) {
			if openErr != nil || target == nil || target.buffer == nil {
				return
			}
			column, err := target.buffer.RuneColumn(locations[0].Range.Start)
			if err == nil {
				m.moveCursor(target, locations[0].Range.Start.Line, column, false)
			}
			m.editing = true
		})
		if len(locations) > 1 {
			m.message = fmt.Sprintf("Opened first of %d definitions", len(locations))
		}
	}
}
