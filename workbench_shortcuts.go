package main

import (
	ui "github.com/neko233-com/godesktop"
	"time"
)

func (m *model) workbenchShortcut(cx *ui.Context, e ui.InputEvent) bool {
	if e.Kind == ui.InputCancelled {
		m.fileActions.chordUntil = time.Time{}
		m.keyboard.shiftDown, m.keyboard.shiftUsed, m.keyboard.lastShift = false, false, time.Time{}
	}
	if m.keymapProfile() == "jetbrains" {
		if e.Key == 16 && e.Kind == ui.KeyPressed && !e.Repeat {
			m.keyboard.shiftDown, m.keyboard.shiftUsed = true, false
			return true
		}
		if e.Key == 16 && e.Kind == ui.KeyReleased && m.keyboard.shiftDown {
			m.keyboard.shiftDown = false
			if !m.keyboard.shiftUsed {
				now := time.Now()
				if !m.keyboard.lastShift.IsZero() && now.Sub(m.keyboard.lastShift) < 500*time.Millisecond {
					m.keyboard.lastShift = time.Time{}
					m.openQuickInput(false)
					m.quick.everywhere = true
				} else {
					m.keyboard.lastShift = now
				}
			}
			return true
		}
		if e.Kind == ui.KeyPressed || e.Kind == ui.Character || e.Kind == ui.PointerPressed {
			m.keyboard.shiftUsed = true
			m.keyboard.lastShift = time.Time{}
		}
	}
	if e.Kind != ui.KeyPressed {
		return false
	}
	command := e.Modifiers&(ui.ModifierControl|ui.ModifierCommand) != 0
	if !m.fileActions.chordUntil.IsZero() {
		pending := time.Now().Before(m.fileActions.chordUntil)
		m.fileActions.chordUntil = time.Time{}
		if pending {
			switch e.Key {
			case 'S':
				m.menu.suppressCharacter = 'S'
				m.runWorkbenchCommand("saveAll")
				return true
			case 'O':
				if command {
					m.runWorkbenchCommand("openFolder")
					return true
				}
			case 'I':
				if command {
					m.runWorkbenchCommand("hover")
					return true
				}
			case 27:
				return true
			}
		}
	}
	if command && e.Key == 'K' && m.keymapProfile() == "vscode" && e.Modifiers&ui.ModifierAlt == 0 {
		m.fileActions.chordUntil = time.Now().Add(5 * time.Second)
		return true
	}
	modifiers := e.Modifiers & (ui.ModifierControl | ui.ModifierShift | ui.ModifierAlt | ui.ModifierCommand)
	if modifiers&ui.ModifierCommand != 0 {
		modifiers = modifiers&^ui.ModifierCommand | ui.ModifierControl
	}
	key := e.Key
	if key == ',' {
		key = 188
	}
	if key == '\\' {
		key = 220
	}
	if key == '`' {
		key = 192
	}
	for _, b := range keyboardBindings(m.keymapProfile()) {
		if key == b.key && modifiers == b.modifiers {
			if b.command == "closeEditor" && m.terminalFocused {
				return false
			}
			m.runWorkbenchCommand(b.command)
			return true
		}
	}
	if m.keymapProfile() == "jetbrains" && command {
		// Reserved JetBrains editing/navigation chords must not fall through
		// to VS Code Close Editor/New File or accidental plain text insertion.
		switch key {
		case 'W', 'N', 'P', 'K', 'J', 'H', 'D':
			return !m.terminalFocused
		}
	}
	return false
}
