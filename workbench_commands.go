package main

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

// Menu labels, ordering and bindings follow VS Code's MIT licensed menu
// contributions at 1.141.0; see agent docs/windows-workbench.md for provenance.
type workbenchCommand struct {
	ID, Title, Binding string
	Run                func()
	Enabled            bool
	Checked            bool
}
type menuEntry struct {
	ID, Title, Binding string
	Submenu            string
	Enabled, Checked   bool
}

func (m *model) workbenchCommands() []workbenchCommand {
	d := m.current()
	if m.extensionsView.detail != "" || m.scm.diff != nil {
		d = nil
	}
	editable := d != nil && d.buffer != nil && !m.saveBusy && !m.fileActions.busy
	dirty := false
	for _, doc := range m.docs {
		dirty = dirty || doc.dirty()
	}
	c := func(id, title, binding string, enabled bool, run func()) workbenchCommand {
		return workbenchCommand{ID: id, Title: title, Binding: m.keymapBinding(id, binding), Enabled: enabled, Run: run}
	}
	commands := []workbenchCommand{
		c("autoSave", "Auto Save", "", true, m.toggleAutoSave),
		c("revertFile", "Revert File", "", d != nil && d.buffer != nil && !d.untitled && m.publishWatches != nil && !m.fileActions.busy && !m.saveBusy && len(m.saveJobs) == 0 && !m.reloadBusy, func() { m.beginReload(d) }),
		c("keymapVSCode", "Use VS Code Keymap", "", true, func() { m.selectKeymap("vscode") }),
		c("keymapJetBrains", "Use JetBrains Keymap", "", true, func() { m.selectKeymap("jetbrains") }),
		c("keyboard", "Keyboard Shortcuts", "", true, m.openSettings),
		c("recentFiles", "Recent Files", "", true, func() { m.openQuickInput(false); m.quick.recent = true }),
		c("newFile", "New Text File", "Ctrl+N", !m.closeBusy, m.newTextFile),
		c("newWindow", "New Window", "Ctrl+Shift+N", m.relaunch != nil, func() { m.relaunch(m.workspace, false) }),
		c("openFile", "Open File...", "Ctrl+O", m.fileActions.choose != nil, func() { m.chooseFileAction("open", nil, nil) }),
		c("openFolder", "Open Folder...", "Ctrl+K Ctrl+O", m.fileActions.choose != nil && m.relaunch != nil, func() { m.chooseFileAction("folder", nil, nil) }),
		c("save", "Save", "Ctrl+S", editable, m.saveActive),
		c("saveAs", "Save As...", "Ctrl+Shift+S", editable && m.fileActions.choose != nil, func() { m.saveAsDocument(d, nil) }),
		c("saveAll", "Save All", "Ctrl+K S", dirty && !m.saveBusy && !m.fileActions.busy, func() {
			var docs []*document
			for _, doc := range m.docs {
				if doc.dirty() {
					docs = append(docs, doc)
				}
			}
			if m.requestSave != nil {
				m.requestSave(context.Background(), docs, func(err error) {
					if err != nil {
						m.message = err.Error()
					}
				})
			}
		}),
		c("closeEditor", "Close Editor", "Ctrl+W", d != nil || m.extensionsView.detail != "" || m.scm.diff != nil, func() {
			if m.extensionsView.detail != "" {
				m.closeExtensionDetails()
			} else if m.scm.diff != nil {
				m.dismissSCMDiff()
				m.focusTab(m.current())
			} else {
				m.closeTab(m.active)
			}
		}),
		c("closeWindow", "Close Window", "Alt+F4", m.native != nil, func() { m.native.RequestClose() }),
		c("undo", "Undo", "Ctrl+Z", editable, func() { m.requestHistory(d, false) }),
		c("redo", "Redo", "Ctrl+Y", editable, func() { m.requestHistory(d, true) }),
		c("cut", "Cut", "Ctrl+X", editable, func() { m.editorCommand('X') }),
		c("copy", "Copy", "Ctrl+C", editable, func() { m.editorCommand('C') }),
		c("paste", "Paste", "Ctrl+V", editable, func() { m.editorCommand('V') }),
		c("selectAll", "Select All", "Ctrl+A", editable, func() { m.editorCommand('A') }),
		c("find", "Find in Files", "Ctrl+Shift+F", true, func() { m.showSearch(m.native) }),
		c("replace", "Replace in Files", "Ctrl+Shift+H", true, func() {
			m.showSearch(m.native)
			m.search.replaceShown = true
			m.search.focus = 3
			m.search.caret = utf8.RuneCountInString(m.search.replacement)
		}),
		c("commands", "Command Palette...", "Ctrl+Shift+P", true, func() { m.openQuickInput(true) }),
		c("quickOpen", "Go to File...", "Ctrl+P", true, func() { m.openQuickInput(false) }),
		c("explorer", "Explorer", "Ctrl+Shift+E", true, func() { m.hideSidebar = false; m.activity = "files" }),
		c("search", "Search", "Ctrl+Shift+F", true, func() { m.hideSidebar = false; m.showSearch(m.native) }),
		c("scm", "Source Control", "Ctrl+Shift+G", true, func() { m.hideSidebar = false; m.showSCM() }),
		c("debug", "Run and Debug", "Ctrl+Shift+D", true, func() { m.hideSidebar = false; m.activity = "debug" }),
		c("extensions", "Extensions", "Ctrl+Shift+X", true, m.showExtensions),
		c("sidebar", "Primary Side Bar", "Ctrl+B", true, func() { m.hideSidebar = !m.hideSidebar }),
		c("panel", "Panel", "Ctrl+J", true, m.togglePanel),
		c("split", "Split Right", "Ctrl+\\", d != nil, func() { m.splitEditor(false) }),
		c("splitDown", "Split Down", "", d != nil, func() { m.splitEditor(true) }),
		c("goLine", "Go to Line/Column...", "Ctrl+G", d != nil, func() { m.navigation = true; m.query = ""; m.terminalFocused = false }),
		c("definition", "Go to Definition", "F12", d != nil && m.requestLSP != nil, func() { m.requestLSP(d, "textDocument/definition") }),
		c("hover", "Show Hover", "Ctrl+K Ctrl+I", d != nil && m.requestLSP != nil, func() { m.requestLSP(d, "textDocument/hover") }),
		c("format", "Format Document", "Shift+Alt+F", editable && m.requestLSP != nil, func() { m.requestLSP(d, "textDocument/formatting") }),
		c("newTerminal", "New Terminal", "Ctrl+Shift+`", m.newTerminal != nil, func() { m.newTerminal(); m.focusTerminal() }),
		c("terminal", "Terminal", "Ctrl+`", m.newTerminal != nil, func() {
			if m.currentTerminal() == nil {
				m.newTerminal()
			}
			m.focusTerminal()
		}),
		c("killTerminal", "Kill Terminal", "", m.currentTerminal() != nil && m.killTerminal != nil, func() { m.killTerminal(m.currentTerminal()) }),
		c("settings", "Settings", "Ctrl+,", true, m.openSettings),
		c("installVSIX", "Install from VSIX...", "", m.fileActions.choose != nil && m.extensionsView.manage != nil, func() { m.chooseFileAction("vsix", nil, nil) }),
		c("reload", "Reload Window", "", m.relaunch != nil, func() { m.requestRelaunch(m.workspace) }),
		c("checkUpdates", "Check for Updates...", "", m.requestUpdate != nil, func() { m.activity = "settings"; m.requestUpdate() }),
		c("copilotChat", "Open Copilot Chat", "Ctrl+I", true, func() { m.panel = "COPILOT"; m.showPanel = true; m.chatFocused = true; m.editing = false }),
		c("copilotSignIn", "Copilot: Sign In", "", m.signInCopilot != nil, func() { m.signInCopilot() }),
		c("copilotChatSignIn", "Copilot: Sign In for Chat", "", m.signInChat != nil, func() { m.signInChat() }),
		c("restartLanguages", "Restart Language Servers", "", m.restartLanguages != nil, func() { m.restartLanguages() }),
	}
	for i := range commands {
		switch commands[i].ID {
		case "sidebar":
			commands[i].Checked = !m.hideSidebar
		case "panel":
			commands[i].Checked = m.showPanel
		case "autoSave":
			commands[i].Checked = m.autoSaveConfig().Mode != "off"
		}
	}
	for _, command := range m.commands {
		id := command.ID
		commands = append(commands, c("extension:"+id, command.Title, "", m.execute != nil, func() { m.execute(id) }))
	}
	return commands
}
func (m *model) editorCommand(key int) {
	m.terminalFocused, m.editing = false, true
	m.input(m.native, ui.InputEvent{Kind: ui.KeyPressed, Key: key, Modifiers: ui.ModifierControl})
}
func (m *model) runWorkbenchCommand(id string) bool {
	for _, c := range m.workbenchCommands() {
		if c.ID == id {
			if c.Enabled && c.Run != nil {
				c.Run()
			}
			return true
		}
	}
	return false
}
func (m *model) menuEntries(name string) []menuEntry {
	if name == "More" {
		var entries []menuEntry
		for _, n := range m.menu.overflow {
			entries = append(entries, menuEntry{Title: n, Submenu: n, Enabled: true})
		}
		return entries
	}
	ids := map[string][]string{
		"File":          {"newFile", "newWindow", "-", "openFile", "openFolder", "@Open Recent", "-", "save", "saveAs", "saveAll", "-", "autoSave", "@Preferences", "-", "revertFile", "closeEditor", "?Close Folder", "closeWindow", "-", "closeWindow:Exit"},
		"Edit":          {"undo", "redo", "-", "cut", "copy", "paste", "-", "?Find", "?Replace", "-", "find", "replace"},
		"Selection":     {"selectAll", "-", "?Expand Selection", "?Shrink Selection", "-", "?Copy Line Up", "?Copy Line Down", "?Move Line Up", "?Move Line Down", "?Duplicate Selection", "-", "?Add Cursor Above", "?Add Cursor Below", "?Add Cursors to Line Ends", "?Add Next Occurrence", "?Add Previous Occurrence", "?Select All Occurrences", "-", "?Switch to Ctrl+Click for Multi-Cursor"},
		"View":          {"commands", "-", "explorer", "search", "scm", "debug", "extensions", "-", "@Appearance", "@Editor Layout", "-", "?Problems", "?Output", "?Debug Console", "terminal"},
		"Go":            {"?Back", "?Forward", "-", "quickOpen", "?Go to Symbol in Workspace...", "?Go to Symbol in Editor...", "-", "definition", "?Go to Type Definition", "?Go to Implementations", "?Go to References", "-", "goLine"},
		"Run":           {"?Start Debugging", "?Run Without Debugging", "?Stop Debugging", "?Restart Debugging", "-", "?Open Configurations", "?Add Configuration...", "-", "?Step Over", "?Step Into", "?Step Out", "?Continue", "-", "?Toggle Breakpoint"},
		"Terminal":      {"newTerminal", "?Split Terminal", "-", "?Run Task...", "?Run Build Task...", "?Run Active File", "?Run Selected Text", "-", "?Configure Tasks...", "?Configure Default Build Task...", "-", "killTerminal"},
		"Help":          {"?Welcome", "?Show All Commands", "?Documentation", "?Editor Playground", "-", "?Release Notes", "?View License", "-", "checkUpdates", "about:About"},
		"Preferences":   {"settings", "keyboard", "?Color Theme", "?File Icon Theme", "extensions"},
		"Appearance":    {"?Full Screen", "?Zen Mode", "sidebar", "panel", "?Status Bar", "?Zoom In", "?Zoom Out", "?Reset Zoom"},
		"Editor Layout": {"split", "splitDown", "?Single", "?Two Columns", "?Three Columns", "?Grid (2x2)"},
	}
	if name == "Open Recent" {
		var result []menuEntry
		for _, d := range m.docs {
			if !d.untitled {
				result = append(result, menuEntry{ID: "recent:" + d.path, Title: filepath.Base(d.path), Enabled: true})
			}
		}
		if len(result) == 0 {
			return []menuEntry{{Title: "No Recent Files"}}
		}
		return result
	}
	commands := map[string]workbenchCommand{}
	for _, c := range m.workbenchCommands() {
		commands[c.ID] = c
	}
	var result []menuEntry
	for _, id := range ids[name] {
		switch {
		case id == "-":
			result = append(result, menuEntry{})
		case strings.HasPrefix(id, "@"):
			result = append(result, menuEntry{Title: id[1:], Submenu: id[1:], Enabled: true})
		case strings.HasPrefix(id, "?"):
			result = append(result, menuEntry{Title: id[1:]})
		case id == "about:About":
			result = append(result, menuEntry{ID: "about", Title: "About", Enabled: true})
		default:
			key, title, _ := strings.Cut(id, ":")
			c := commands[key]
			if title == "" {
				title = c.Title
			}
			result = append(result, menuEntry{ID: key, Title: title, Binding: c.Binding, Enabled: c.Enabled, Checked: c.Checked})
		}
	}
	return result
}
func (m *model) newTextFile() {
	m.untitledSequence++
	buffer, _ := textbuffer.New("")
	d := &document{path: filepath.Join(m.workspace, "Untitled-"+strconv.FormatUint(m.untitledSequence, 10)), buffer: buffer, untitled: true}
	m.docs = append(m.docs, d)
	m.addGroupDocument(m.findGroup(m.groups.active), d)
	m.focusTab(d)
	m.extensionsView.detail = ""
}
