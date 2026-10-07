package main

import (
	"fmt"
	"strings"
	"unicode/utf8"

	ui "github.com/neko233-com/godesktop"
)

func (m *model) showExtensions() {
	m.activity = "extensions"
	m.hideSidebar = false
	m.palette = false
	m.extensionsView.focused = true
	m.editing, m.terminalFocused, m.chatFocused = false, false, false
}
func (m *model) closeExtensionDetails() {
	m.extensionsView.detail = ""
	m.extensionsView.focused = false
	if d := m.current(); d != nil {
		m.focusTab(d)
	}
}
func (m *model) filteredExtensions() []extensionInfo {
	query := strings.TrimSpace(m.extensionsView.query)
	enabledOnly, disabledOnly := strings.Contains(query, "@enabled"), strings.Contains(query, "@disabled")
	for _, token := range []string{"@installed", "@enabled", "@disabled"} {
		query = strings.ReplaceAll(query, token, "")
	}
	var result []extensionInfo
	for _, e := range m.installed {
		disabled := containsExtension(m.extensionsView.settings.Disabled, e.ID)
		if containsExtension(m.extensionsView.settings.Uninstall, e.ID) || enabledOnly && disabled || disabledOnly && !disabled {
			continue
		}
		if quickMatches(e.Name+" "+e.ID+" "+e.Description, query) {
			result = append(result, e)
		}
	}
	return result
}
func (m *model) extensionsSidebar(cx *ui.Context) *ui.Element {
	query := m.extensionsView.query
	fg := uint32(foreground)
	if query == "" {
		query = "Search Extensions in " + m.extensionStoreName()
		fg = muted
	}
	if m.extensionsView.focused {
		query += "▏"
	}
	inputBorder := uint32(border)
	if m.extensionsView.focused {
		inputBorder = accent
	}
	field := ui.Column(label(query).Foreground(ui.RGB(fg)).PaddingXY(6, 0).Height(26).Background(ui.RGB(0x313131)).ClipRounded(3)).Padding(1).ClipRounded(4).Background(ui.RGB(inputBorder)).Key("extensions-search").OnClick(func(*ui.Context) { m.extensionsView.focused = true; m.editing = false })
	items := m.filteredExtensions()
	section := "INSTALLED"
	if m.extensionsView.query != "" && !strings.Contains(m.extensionsView.query, "@") {
		section = "SEARCH RESULTS · " + strings.ToUpper(m.extensionStoreName())
		for _, result := range m.extensionsView.catalog.results {
			known := false
			for _, e := range items {
				if strings.EqualFold(e.ID, result.id()) {
					known = true
					break
				}
			}
			if !known {
				items = append(items, result.info())
			}
		}
	}
	children := []*ui.Element{ui.Column(field).PaddingXY(12, 6).Height(41), ui.Row(ui.Icon("chevron-down").Width(20).Height(22), label(section).FontSize(11).Flex(1), label(fmt.Sprint(len(items))).FontSize(11).PaddingXY(8, 0)).Height(22)}
	if m.extensionsView.catalog.busy {
		children = append(children, label("Searching "+m.extensionStoreName()+"…").FontSize(11).PaddingXY(12, 0).Height(22))
	}
	if m.extensionsView.catalog.error != "" {
		children = append(children, label("Search failed · retry by editing query").FontSize(11).Foreground(ui.RGB(0xf48771)).PaddingXY(12, 0).Height(22))
	}
	if m.extensionsView.reload {
		children = append(children, ui.Row(button("Restart Extensions", "extensions-reload", func(*ui.Context) { m.requestRelaunch(m.workspace) }).Height(28).Background(ui.RGB(accent))).PaddingXY(12, 4), label("Reload required to apply changes").FontSize(11).Foreground(ui.RGB(muted)).PaddingXY(12, 0).Height(22))
	}
	_, height := cx.WindowSize()
	visible := max(1, int((height-170)/72))
	m.extensionsView.scroll = max(0, min(m.extensionsView.scroll, max(0, len(items)-visible)))
	for i := m.extensionsView.scroll; i < len(items) && i < m.extensionsView.scroll+visible; i++ {
		e := items[i]
		bg := ui.Color{}
		if m.extensionsView.detail == e.ID {
			bg = ui.RGB(0x04395e)
		}
		publisher, _, _ := strings.Cut(e.ID, ".")
		state := ""
		installed := false
		for _, known := range m.installed {
			if strings.EqualFold(known.ID, e.ID) {
				installed = true
				break
			}
		}
		if containsExtension(m.extensionsView.settings.Disabled, e.ID) {
			state = "Disabled"
		}
		action := icon("settings", "extension-gear-"+e.ID, func(*ui.Context) { m.extensionsView.detail = e.ID; m.extensionsView.focused = false }).Width(18).Height(18)
		if !installed {
			var install func(*ui.Context)
			if !m.extensionsView.busy && m.extensionsView.manage != nil {
				install = func(*ui.Context) { m.extensionsView.manage("catalogInstall", e.ID) }
			}
			action = button("Install", "extension-install-"+e.ID, install).FontSize(11).PaddingXY(6, 0).Height(18).Background(ui.RGB(accent))
		}
		row := ui.Row(ui.Icon("extensions").Width(32).Height(40).Foreground(ui.RGB(muted)), ui.Column(ui.Row(label(e.Name).Flex(1), label(e.Version).FontSize(10).Foreground(ui.RGB(muted))).Gap(6), label(e.Description).FontSize(12).Foreground(ui.RGB(muted)), ui.Row(label(publisher).FontSize(11).Foreground(ui.RGB(muted)).Flex(1), label(state).FontSize(11).Foreground(ui.RGB(muted)), action)).Gap(3).Flex(1)).Gap(8).PaddingXY(12, 6).Height(72).Radius(4).HoverBackground(ui.RGB(0x2a2d2e)).Background(bg).FocusRing(false).Key("extension-card-" + e.ID).OnClick(func(*ui.Context) {
			m.extensionsView.detail = e.ID
			m.extensionsView.focused = false
			m.extensionsView.tab = "Details"
		})
		children = append(children, row)
	}
	if len(items) == 0 {
		children = append(children, label("No extensions found.").PaddingXY(12, 0).Height(28))
	}
	children = append(children, spacer(), rule(), button("Install from VSIX...", "extensions-install-vsix", func(*ui.Context) { m.chooseFileAction("vsix", nil, nil) }).Height(28))
	return ui.Column(children...).Flex(1)
}
func (m *model) extensionDetailView() *ui.Element {
	var selected *extensionInfo
	for i := range m.installed {
		if m.installed[i].ID == m.extensionsView.detail {
			selected = &m.installed[i]
			break
		}
	}
	installed := selected != nil
	if selected == nil {
		for _, result := range m.extensionsView.catalog.results {
			if result.id() == m.extensionsView.detail {
				info := result.info()
				selected = &info
				break
			}
		}
	}
	if selected == nil {
		m.extensionsView.detail = ""
		return ui.Column()
	}
	e := *selected
	disabled := containsExtension(m.extensionsView.settings.Disabled, e.ID)
	action, title := "disable", "Disable"
	if disabled {
		action, title = "enable", "Enable"
	}
	if !installed {
		action, title = "catalogInstall", "Install"
	}
	var toggle, uninstall func(*ui.Context)
	if !m.extensionsView.busy && m.extensionsView.manage != nil {
		toggle = func(*ui.Context) { m.extensionsView.manage(action, e.ID) }
		if installed && !strings.EqualFold(e.ID, bundledID) {
			uninstall = func(*ui.Context) { m.extensionsView.manage("uninstall", e.ID) }
		}
	}
	header := ui.Row(ui.Icon("extensions").Width(100).Height(100).Foreground(ui.RGB(muted)), ui.Column(label(e.Name).FontSize(26), ui.Row(label(e.ID).Foreground(ui.RGB(muted)), label("v"+e.Version).Foreground(ui.RGB(muted))).Gap(16), label(e.Description).FontSize(14), ui.Row(button(title, "extension-toggle", toggle).Height(28).Background(ui.RGB(accent)), button("Uninstall", "extension-uninstall", uninstall).Height(28).Background(ui.RGB(0x313131))).Gap(8)).Gap(10).Flex(1)).Gap(24).Padding(28).Height(200)
	items := []*ui.Element{ui.Row(label("Extensions: "+e.Name).PaddingXY(12, 0).Flex(1), icon("close", "extension-detail-close", func(*ui.Context) { m.closeExtensionDetails() })).Height(35).Background(ui.RGB(outer)), header}
	if m.extensionsView.reload {
		items = append(items, ui.Row(label("Reload the window to apply extension changes.").Flex(1), button("Reload Window", "extension-detail-reload", func(*ui.Context) { m.requestRelaunch(m.workspace) }).Background(ui.RGB(accent)).Height(28)).PaddingXY(28, 4).Height(36))
	}
	if m.extensionsView.tab == "" {
		m.extensionsView.tab = "Details"
	}
	tabs := []*ui.Element{}
	for _, tab := range []string{"Details", "Feature Contributions"} {
		fg := uint32(muted)
		if m.extensionsView.tab == tab {
			fg = foreground
		}
		tabs = append(tabs, button(tab, "extension-tab-"+tab, func(*ui.Context) { m.extensionsView.tab = tab }).Foreground(ui.RGB(fg)).Height(36))
	}
	items = append(items, ui.Row(tabs...).PaddingXY(28, 0).Height(36), rule())
	content := []*ui.Element{}
	if m.extensionsView.tab == "Feature Contributions" {
		content = append(content, label("Commands").FontSize(20).Height(36))
		for _, c := range m.extensionsView.contributions[e.ID] {
			command := c
			var run func(*ui.Context)
			if m.extensionsView.running[strings.ToLower(e.ID)] && m.execute != nil {
				run = func(*ui.Context) { m.execute(command.ID) }
			}
			content = append(content, button(command.Title, "extension-detail-command-"+command.ID, run).Height(28), label(command.ID).FontSize(11).Foreground(ui.RGB(muted)).Height(20))
		}
	} else {
		content = append(content, label(e.Name).FontSize(22).Height(38))
		for _, line := range wrapChatText(e.Description, 650) {
			content = append(content, label(line).Height(24))
		}
		state := "Enabled"
		if !installed {
			state = "Available in " + m.extensionStoreName()
		}
		if disabled {
			state = "Disabled"
		}
		if containsExtension(m.extensionsView.settings.Uninstall, e.ID) {
			state = "Uninstalled · reload required"
		}
		content = append(content, label("Identifier: "+e.ID).Height(28), label("Version: "+e.Version).Height(28), label("Status: "+state).Height(28), label("Runs in the isolated Node extension host").Foreground(ui.RGB(muted)).Height(28))
	}
	items = append(items, ui.Column(content...).Padding(28), spacer())
	return ui.Column(items...).Flex(1).Background(ui.RGB(editor))
}
func (m *model) extensionInput(cx *ui.Context, e ui.InputEvent) bool {
	if m.activity != "extensions" {
		return false
	}
	if e.Kind == ui.Scroll && cx != nil {
		if b, ok := cx.ElementBounds("extensions-sidebar"); ok && insideBounds(b, e.PointerX, e.PointerY) {
			m.extensionsView.scroll = max(0, m.extensionsView.scroll-int(e.Y))
			return true
		}
	}
	if e.Kind == ui.PointerPressed {
		if cx != nil {
			b, ok := cx.ElementBounds("extensions-search")
			m.extensionsView.focused = ok && insideBounds(b, e.X, e.Y)
		}
		return false
	}
	if !m.extensionsView.focused {
		return false
	}
	command := e.Modifiers&(ui.ModifierControl|ui.ModifierCommand) != 0
	if e.Kind == ui.Character && !command && e.Key >= 32 && utf8.ValidRune(rune(e.Key)) && len(m.extensionsView.query) < 1024 {
		m.extensionsView.query += string(rune(e.Key))
		m.extensionQueryChanged()
		return true
	}
	if e.Kind == ui.KeyPressed {
		switch e.Key {
		case 8:
			r := []rune(m.extensionsView.query)
			if len(r) > 0 {
				m.extensionsView.query = string(r[:len(r)-1])
			}
			m.extensionQueryChanged()
		case 27:
			m.extensionsView.focused = false
		case 'V':
			if command && m.readClipboard != nil {
				s, err := m.readClipboard()
				if err == nil && len(s) <= 1024 {
					m.extensionsView.query = s
					m.extensionQueryChanged()
				}
			}
		}
		return true
	}
	return false
}
