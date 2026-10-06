package main

import (
	"os"
	"strings"
	"testing"

	"github.com/neko233-com/gocode/internal/copilotservice"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

func shortcut(m *model, key int, shift bool) {
	mod := ui.ModifierControl
	if shift {
		mod |= ui.ModifierShift
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: key, Modifiers: mod})
}
func TestSelectionClipboardUndoAndCRLFSave(t *testing.T) {
	m := testModel(t)
	d := m.current()
	d.buffer, _ = textbuffer.New("你😀\r\nsecond\r\n")
	var clipboard string
	m.writeClipboard = func(s string) error { clipboard = s; return nil }
	m.readClipboard = func() (string, error) { return clipboard, nil }
	var versions []int
	m.onDocument = func(kind string, _ *document, change textbuffer.ChangeEvent) {
		if kind == "change" {
			versions = append(versions, change.Version)
		}
	}
	m.moveCursor(d, 0, 1, false)
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 39, Modifiers: ui.ModifierShift})
	shortcut(m, 'C', false)
	if clipboard != "😀" {
		t.Fatalf("copied %q", clipboard)
	}
	text(m, "界")
	if d.buffer.Line(0) != "你界" {
		t.Fatal("selection was not replaced")
	}
	shortcut(m, 'Z', false)
	if d.buffer.Line(0) != "你😀" {
		t.Fatal("undo lost surrogate pair")
	}
	shortcut(m, 'Z', true)
	if d.buffer.Line(0) != "你界" {
		t.Fatal("redo failed")
	}
	m.moveCursor(d, 1, 6, false)
	shortcut(m, 'V', false)
	if d.buffer.Line(1) != "second😀" {
		t.Fatal("Unicode paste")
	}
	if err := m.save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(d.path)
	if string(data) != "你界\r\nsecond😀\r\n" || d.buffer.Dirty() {
		t.Fatalf("saved %q", data)
	}
	shortcut(m, 'Z', false)
	if !d.buffer.Dirty() {
		t.Fatal("undo after save must mark modified")
	}
	shortcut(m, 'Y', false)
	if d.buffer.Dirty() {
		t.Fatal("redo to saved revision should be clean")
	}
	for i, v := range versions {
		if v != i+2 {
			t.Fatalf("non-monotonic provider versions %v", versions)
		}
	}
}

func TestInlineAcceptanceRejectsStaleAndUsesUTF16Range(t *testing.T) {
	m := testModel(t)
	d := m.current()
	d.buffer, _ = textbuffer.New("你😀")
	m.moveCursor(d, 0, 2, false)
	r := textbuffer.Range{Start: textbuffer.Position{Character: 1}, End: textbuffer.Position{Character: 3}}
	m.suggestion = &inlineSuggestion{d.path, d.buffer.Version(), d.cursor(), copilotservice.InlineItem{InsertText: "😀 + sum", Range: &r}}
	if ghostText(d, m.suggestion) != " + sum" {
		t.Fatal("ghost prefix should use UTF-16 range")
	}
	accepted := 0
	m.acceptInline = func(copilotservice.InlineItem) { accepted++ }
	key(m, 9)
	if d.buffer.Text() != "你😀 + sum" || accepted != 1 {
		t.Fatalf("accept %q, %d", d.buffer.Text(), accepted)
	}
	if err := m.applyDocumentEdits(d.path, 1, []textbuffer.Edit{{Range: r, Text: "stale"}}); err == nil {
		t.Fatal("stale provider edit accepted")
	}
	m.suggestion = &inlineSuggestion{d.path, 1, d.cursor(), copilotservice.InlineItem{InsertText: "stale"}}
	key(m, 9)
	if strings.Contains(d.buffer.Text(), "stale") || accepted != 1 {
		t.Fatal("stale suggestion inserted or reported accepted")
	}
}

func TestDiagnosticsReplaceAndClearCollections(t *testing.T) {
	m := testModel(t)
	m.setDiagnostics(m.current().path, "one", []byte(`[{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"message":"error","severity":0}]`))
	m.setDiagnostics(m.current().path, "two", []byte(`[{"range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}},"message":"warning","severity":1}]`))
	if errors, warnings := m.diagnosticCounts(); errors != 1 || warnings != 1 {
		t.Fatal("collections were merged incorrectly")
	}
	m.setDiagnostics(m.current().path, "one", []byte(`[]`))
	if errors, warnings := m.diagnosticCounts(); errors != 0 || warnings != 1 || len(m.problems()) != 1 {
		t.Fatal("clear did not replace collection")
	}
}

func TestMultilineCompletionKeepsCaretAfterInsertedText(t *testing.T) {
	m := testModel(t)
	d := m.current()
	d.buffer, _ = textbuffer.New("package main\n\n// add returns the sum of two integers.\nfunc add(a,b int) int {\n    ")
	m.moveCursor(d, 4, 4, false)
	r := textbuffer.Range{Start: d.cursor(), End: d.cursor()}
	m.suggestion = &inlineSuggestion{d.path, d.buffer.Version(), d.cursor(), copilotservice.InlineItem{InsertText: "return a + b", Range: &r}}
	key(m, 9)
	if d.line != 4 || d.column != 16 {
		t.Fatalf("completion caret %d:%d, selection %+v", d.line, d.column, d.buffer.Selection())
	}
}
