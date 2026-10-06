package main

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/neko233-com/gocode/internal/copilotservice"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
	"github.com/neko233-com/godesktop/extensions"
)

type completionItem struct {
	Label               json.RawMessage         `json:"label"`
	InsertText          string                  `json:"insertText"`
	Range               *textbuffer.Range       `json:"range"`
	TextEdit            *textbuffer.Edit        `json:"textEdit"`
	Command             *copilotservice.Command `json:"command"`
	AdditionalTextEdits []textbuffer.Edit       `json:"additionalTextEdits,omitempty"`
}
type completionSuggestion struct {
	path     string
	version  int
	position textbuffer.Position
	item     completionItem
}

func (i completionItem) title() string {
	var text string
	if json.Unmarshal(i.Label, &text) == nil {
		return text
	}
	var value struct {
		Label string `json:"label"`
	}
	_ = json.Unmarshal(i.Label, &value)
	return value.Label
}
func wordRange(d *document, p textbuffer.Position) textbuffer.Range {
	column, _ := d.buffer.RuneColumn(p)
	line := []rune(d.buffer.Line(p.Line))
	start, end := column, column
	isWord := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }
	for start > 0 && isWord(line[start-1]) {
		start--
	}
	for end < len(line) && isWord(line[end]) {
		end++
	}
	return textbuffer.Range{Start: d.buffer.PositionFromRunes(p.Line, start), End: d.buffer.PositionFromRunes(p.Line, end)}
}
func (m *model) chooseCompletion(s completionSuggestion) {
	d := m.current()
	if d == nil || d.buffer == nil || d.path != s.path || d.buffer.Version() != s.version || d.cursor() != s.position {
		m.completions = nil
		return
	}
	edit := textbuffer.Edit{Range: wordRange(d, s.position), Text: s.item.InsertText}
	if edit.Text == "" {
		edit.Text = s.item.title()
	}
	if s.item.Range != nil {
		edit.Range = *s.item.Range
	}
	if s.item.TextEdit != nil {
		edit = *s.item.TextEdit
	}
	if err := m.applyDocumentEdits(s.path, s.version, append([]textbuffer.Edit{edit}, s.item.AdditionalTextEdits...)); err != nil {
		m.message = err.Error()
		return
	}
	if s.item.Command != nil && m.execute != nil {
		m.execute(s.item.Command.Command)
	}
}
func (m *model) bindCompletions(ctx context.Context, cx *ui.Context, host *extensions.Host) {
	m.requestCompletions = func(d *document) {
		if !d.serviceEligible() {
			return
		}
		state, position, generation := stateOf(d), d.cursor(), m.inlineGeneration
		go func() {
			c, stop := context.WithTimeout(ctx, 10*time.Second)
			defer stop()
			if err := m.awaitExtensions(c); err != nil {
				return
			}
			// A full snapshot is used for an explicit request to recover from delayed
			// background notifications before invoking an extension's provider.
			if err := host.Call(c, "syncDocument", map[string]any{"kind": "focus", "document": state}, nil); err != nil {
				return
			}
			var items []completionItem
			err := host.Call(c, "provideCompletionItems", map[string]any{"uri": copilotservice.FileURI(state.Path), "position": position, "context": map[string]int{"triggerKind": 0}}, &items)
			cx.Dispatch(func() {
				current := m.current()
				if current == nil || current.buffer == nil || current.path != state.Path || current.buffer.Version() != state.Version || current.cursor() != position || generation != m.inlineGeneration {
					return
				}
				if err != nil {
					m.message = "Extension completion: " + err.Error()
					return
				}
				var suggestions []completionSuggestion
				for _, item := range items[:min(8, len(items))] {
					if strings.TrimSpace(item.title()) != "" {
						suggestions = append(suggestions, completionSuggestion{state.Path, state.Version, position, item})
					}
				}
				m.publishCompletions("extensions", suggestions)
			})
		}()
	}
}

func (m *model) publishCompletions(source string, items []completionSuggestion) {
	if m.completionSources == nil {
		m.completionSources = map[string][]completionSuggestion{}
	}
	m.completionSources[source] = items
	var keys []string
	for key := range m.completionSources {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	m.completions = nil
	for _, key := range keys {
		for _, item := range m.completionSources[key] {
			if len(m.completions) < 8 {
				m.completions = append(m.completions, item)
			}
		}
	}
}
