package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	ui "github.com/neko233-com/godesktop"
)

func TestBuiltInKeymapsDispatchAndLabels(t *testing.T) {
	for _, profile := range []string{"vscode", "jetbrains"} {
		m, err := newModel(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer m.closeDocuments()
		m.keyboard.profile = profile
		m.requestLSP = func(d *document, method string) { m.message = method }
		for _, b := range keyboardBindings(profile) {
			if m.keymapBinding(b.command, "") != b.label {
				t.Fatal("label differs from dispatcher", profile, b.command)
			}
		}
		key, mods := 'P', ui.ModifierControl|ui.ModifierShift
		if profile == "jetbrains" {
			key = 'A'
		}
		m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: int(key), Modifiers: mods})
		if !m.palette || m.query != ">" {
			t.Fatal(profile, "Find Action/Command Palette did not open")
		}
		m.closeQuickInput()
		m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'N', Modifiers: ui.ModifierControl | ui.ModifierShift})
		if profile == "jetbrains" && (!m.palette || m.query != "") {
			t.Fatal("JetBrains Go to File was interpreted as New Window")
		}
		if m.palette {
			m.closeQuickInput()
		}
		m.newTextFile()
		before := m.current()
		if profile == "jetbrains" {
			m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'W', Modifiers: ui.ModifierControl})
			if m.current() != before {
				t.Fatal("JetBrains Ctrl+W closed source editor")
			}
			m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'L', Modifiers: ui.ModifierControl | ui.ModifierAlt})
			if m.message != "textDocument/formatting" {
				t.Fatal("format shortcut did not request real LSP")
			}
			m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'S', Modifiers: ui.ModifierControl | ui.ModifierAlt})
			if m.activity != "settings" {
				t.Fatal("settings shortcut failed")
			}
			m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'R', Modifiers: ui.ModifierControl | ui.ModifierShift})
			if !m.search.replaceShown || m.search.focus != 3 {
				t.Fatal("JetBrains replace input failed")
			}
			m.activity = "files"
			m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 115, Modifiers: ui.ModifierControl})
			if m.current() == before {
				t.Fatal("JetBrains Ctrl+F4 failed to close")
			}
		}
	}
}
func TestVSCodeLanguageShortcutsPassThroughMenuInput(t *testing.T) {
	m, err := newModel(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.closeDocuments()
	m.newTextFile()
	var methods []string
	m.requestLSP = func(d *document, method string) { methods = append(methods, method) }
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'F', Modifiers: ui.ModifierShift | ui.ModifierAlt})
	if len(methods) != 1 || methods[0] != "textDocument/formatting" || m.menu.name != "" {
		t.Fatal("Shift+Alt+F was consumed by File menu", methods, m.menu.name)
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'K', Modifiers: ui.ModifierControl})
	if len(methods) != 1 {
		t.Fatal("incomplete hover chord executed early")
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'I', Modifiers: ui.ModifierControl})
	if len(methods) != 2 || methods[1] != "textDocument/hover" {
		t.Fatal("Ctrl+K Ctrl+I did not request hover", methods)
	}
}

func TestJetBrainsDoubleShiftAndCancellation(t *testing.T) {
	m, err := newModel(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.closeDocuments()
	m.keyboard.profile = "jetbrains"
	tap := func() {
		m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 16})
		m.input(nil, ui.InputEvent{Kind: ui.KeyReleased, Key: 16})
	}
	tap()
	m.input(nil, ui.InputEvent{Kind: ui.InputCancelled})
	tap()
	if m.palette {
		t.Fatal("focus loss did not reset double Shift")
	}
	m.keyboard.lastShift = time.Time{}
	tap()
	m.input(nil, ui.InputEvent{Kind: ui.Character, Key: 'A'})
	tap()
	if m.palette {
		t.Fatal("ordinary Shift typing activated Search Everywhere")
	}
	m.keyboard.lastShift = time.Time{}
	tap()
	tap()
	if !m.palette || !m.quick.everywhere {
		t.Fatal("Search Everywhere missing")
	}
	m.query = "Keymap"
	items := m.quickItems()
	if len(items) != 2 || !items[0].Command {
		t.Fatal("Search Everywhere omitted native commands")
	}
	m.chooseQuickItem(items[1])
	if m.keymapProfile() != "jetbrains" {
		t.Fatal("command result treated as file path")
	}
}
func TestKeyboardSettingsIdempotentAndBounded(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "gocode", "keyboard.json")
	for run := 0; run < 3; run++ {
		for _, profile := range []string{"jetbrains", "vscode", "vscode"} {
			if err := writeKeymap(path, profile); err != nil {
				t.Fatal(err)
			}
			actual, err := readKeymap(path)
			if err != nil || actual != profile {
				t.Fatal(actual, err)
			}
		}
		entries, err := os.ReadDir(filepath.Dir(path))
		if err != nil || len(entries) != 1 || entries[0].Name() != "keyboard.json" {
			t.Fatal("repeated settings left temporary files", entries, err)
		}
	}
	info, _ := os.Stat(path)
	if err := writeKeymap(path, "vscode"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if !info.ModTime().Equal(after.ModTime()) {
		t.Fatal("identical preset rewrote settings")
	}
	if err := writeKeymap(path, "unknown"); err == nil {
		t.Fatal("unknown preset accepted")
	}
	if value, _ := readKeymap(path); value != "vscode" {
		t.Fatal("invalid preset changed settings")
	}
	if err := os.WriteFile(path, make([]byte, 4097), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readKeymap(path); err == nil {
		t.Fatal("unbounded config accepted")
	}
}

func TestSearchEverywhereRemainsBoundedWithExtensionCommands(t *testing.T) {
	m, err := newModel(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.closeDocuments()
	m.execute = func(string) {}
	for i := 0; i < 200; i++ {
		m.commands = append(m.commands, extensionCommand{fmt.Sprint(i), fmt.Sprint("command ", i)})
	}
	m.quick.everywhere = true
	m.files = []string{"file.go"}
	if items := m.quickItems(); len(items) != 100 {
		t.Fatal("command/file results exceeded shared limit", len(items))
	}
	m.query = ">"
	if items := m.quickItems(); len(items) != 100 {
		t.Fatal("command results exceeded limit", len(items))
	}
}
