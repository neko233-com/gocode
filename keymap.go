package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	ui "github.com/neko233-com/godesktop"
)

type keyboardState struct {
	profile              string
	persist              func(string)
	shiftDown, shiftUsed bool
	lastShift            time.Time
}
type keyboardBinding struct {
	command, label string
	key, modifiers int
}

// Built-in Windows presets. JetBrains bindings follow the GoLand Windows
// reference card; unsupported refactoring/debug actions are not remapped to
// unrelated commands. The same table drives dispatch and displayed labels.
func keyboardBindings(profile string) []keyboardBinding {
	c, s, a := ui.ModifierControl, ui.ModifierShift, ui.ModifierAlt
	if profile == "jetbrains" {
		return []keyboardBinding{
			{"commands", "Ctrl+Shift+A", 'A', c | s}, {"quickOpen", "Ctrl+Shift+N", 'N', c | s},
			{"recentFiles", "Ctrl+E", 'E', c}, {"settings", "Ctrl+Alt+S", 'S', c | a},
			{"saveAll", "Ctrl+S", 'S', c}, {"closeEditor", "Ctrl+F4", 115, c},
			{"find", "Ctrl+Shift+F", 'F', c | s}, {"replace", "Ctrl+Shift+R", 'R', c | s},
			{"definition", "Ctrl+B", 'B', c}, {"hover", "Ctrl+Q", 'Q', c},
			{"format", "Ctrl+Alt+L", 'L', c | a}, {"goLine", "Ctrl+G", 'G', c},
			{"explorer", "Alt+1", '1', a}, {"scm", "Alt+9", '9', a},
			{"terminal", "Alt+F12", 123, a},
		}
	}
	return []keyboardBinding{
		{"newFile", "Ctrl+N", 'N', c}, {"newWindow", "Ctrl+Shift+N", 'N', c | s},
		{"openFile", "Ctrl+O", 'O', c}, {"save", "Ctrl+S", 'S', c},
		{"saveAs", "Ctrl+Shift+S", 'S', c | s}, {"closeEditor", "Ctrl+W", 'W', c},
		{"commands", "Ctrl+Shift+P", 'P', c | s}, {"quickOpen", "Ctrl+P", 'P', c},
		{"sidebar", "Ctrl+B", 'B', c}, {"settings", "Ctrl+,", 188, c},
		{"explorer", "Ctrl+Shift+E", 'E', c | s}, {"extensions", "Ctrl+Shift+X", 'X', c | s},
		{"debug", "Ctrl+Shift+D", 'D', c | s}, {"find", "Ctrl+Shift+F", 'F', c | s},
		{"replace", "Ctrl+Shift+H", 'H', c | s}, {"scm", "Ctrl+Shift+G", 'G', c | s},
		{"split", "Ctrl+\\", 220, c}, {"panel", "Ctrl+J", 'J', c},
		{"goLine", "Ctrl+G", 'G', c}, {"definition", "F12", 123, 0},
		{"format", "Shift+Alt+F", 'F', s | a}, {"terminal", "Ctrl+`", 192, c},
		{"newTerminal", "Ctrl+Shift+`", 192, c | s},
	}
}
func (m *model) keymapProfile() string {
	if m.keyboard.profile == "jetbrains" {
		return "jetbrains"
	}
	return "vscode"
}
func validKeymap(value string) bool { return value == "vscode" || value == "jetbrains" }
func (m *model) selectKeymap(value string) {
	if !validKeymap(value) || m.keymapProfile() == value {
		return
	}
	m.keyboard.profile = value
	m.keyboard.shiftDown, m.keyboard.shiftUsed, m.keyboard.lastShift = false, false, time.Time{}
	m.fileActions.chordUntil = time.Time{}
	if m.keyboard.persist != nil {
		m.keyboard.persist(value)
	}
}
func (m *model) keymapBinding(id, fallback string) string {
	for _, b := range keyboardBindings(m.keymapProfile()) {
		if b.command == id {
			return b.label
		}
	}
	if m.keymapProfile() == "jetbrains" {
		switch id {
		case "undo":
			return "Ctrl+Z"
		case "redo":
			return "Ctrl+Shift+Z"
		case "cut", "copy", "paste", "selectAll", "closeWindow":
			return fallback
		}
		return ""
	}
	return fallback
}
func (m *model) keyboardSidebar() *ui.Element {
	children := []*ui.Element{label("KEYBOARD SHORTCUTS").FontSize(11).Height(28)}
	for _, preset := range []struct{ id, title string }{{"vscode", "VS Code"}, {"jetbrains", "JetBrains"}} {
		title := preset.title
		if m.keymapProfile() == preset.id {
			title = "✓ " + title
		}
		children = append(children, button(title, "keymap-"+preset.id, func(*ui.Context) { m.selectKeymap(preset.id) }).Height(28))
	}
	return ui.Column(children...).PaddingXY(12, 0)
}
func keyboardConfigPath() (string, error) {
	root, err := os.UserConfigDir()
	return filepath.Join(root, "gocode", "keyboard.json"), err
}
func readKeymap(path string) (string, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return "vscode", nil
	}
	if err != nil {
		return "vscode", err
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil {
		return "vscode", err
	}
	if len(body) > 4096 {
		return "vscode", errors.New("keyboard settings exceed 4 KiB")
	}
	var config struct {
		Keymap string `json:"keymap"`
	}
	if err = json.Unmarshal(body, &config); err != nil {
		return "vscode", err
	}
	if !validKeymap(config.Keymap) {
		return "vscode", errors.New("unknown built-in keymap")
	}
	return config.Keymap, nil
}
func writeKeymap(path, profile string) error {
	if !validKeymap(profile) {
		return errors.New("unknown built-in keymap")
	}
	body := []byte(fmt.Sprintf("{\"keymap\":%q}\n", profile))
	if f, err := os.Open(path); err == nil {
		current, readErr := io.ReadAll(io.LimitReader(f, 4097))
		_ = f.Close()
		if readErr == nil && bytes.Equal(current, body) {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".keyboard-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(body); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
func (m *model) startKeyboardSettings(cx *ui.Context, path string) func() {
	requests, done := make(chan string, 1), make(chan struct{})
	m.keyboard.persist = func(value string) {
		select {
		case requests <- value:
		default:
			select {
			case <-requests:
			default:
			}
			select {
			case requests <- value:
			default:
			}
		}
	}
	go func() {
		defer close(done)
		for value := range requests {
			if err := writeKeymap(path, value); err != nil {
				cx.Dispatch(func() { m.message = "Keyboard shortcuts: " + err.Error() })
			}
		}
	}()
	return func() {
		m.keyboard.persist = nil
		close(requests)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}
}
