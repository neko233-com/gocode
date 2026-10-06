package main

import (
	"testing"

	"github.com/neko233-com/gocode/internal/terminal"
	ui "github.com/neko233-com/godesktop"
)

func TestTerminalFocusAndCloseCancelPreserveEditor(t *testing.T) {
	m := testModel(t)
	d := m.current()
	version := d.buffer.Version()
	m.focusTerminal()
	if m.editing || !m.showPanel || m.panel != "TERMINAL" {
		t.Fatal("terminal focus not exclusive")
	}
	m.input(nil, ui.InputEvent{Kind: ui.Character, Key: 'X'})
	if d.buffer.Version() != version {
		t.Fatal("starting terminal typed into editor")
	}
	m.beginClose(nil)
	if m.terminalFocused {
		t.Fatal("modal left terminal focused")
	}
	m.cancelClose()
	if !m.terminalFocused || m.editing {
		t.Fatal("close cancel lost terminal focus")
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'P', Modifiers: ui.ModifierControl})
	if !m.palette {
		t.Fatal("terminal swallowed workbench palette shortcut")
	}
}

func TestTerminalSelectionUsesImmutableUnicodeCells(t *testing.T) {
	f := &terminal.Frame{Size: terminal.Size{Columns: 6, Rows: 2}, Lines: [][]terminal.Cell{{{Text: "a", Width: 1}, {Text: "世", Width: 2}, {Width: 0}, {Text: "😀", Width: 2}, {Width: 0}, {Text: " ", Width: 1}}, {{Text: "b", Width: 1}, {Text: " ", Width: 1}, {Text: " ", Width: 1}, {Text: " ", Width: 1}, {Text: " ", Width: 1}, {Text: " ", Width: 1}}}}
	tab := terminalTab{selectionFrame: f, anchor: 0, active: 7}
	if got := tab.selectedText(); got != "a世😀\nb" {
		t.Fatalf("wide selection %q", got)
	}
	tab.anchor, tab.active = 7, 0
	if got := tab.selectedText(); got != "a世😀\nb" {
		t.Fatalf("reverse selection %q", got)
	}
}
