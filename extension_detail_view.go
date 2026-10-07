package main

import (
	"strings"

	ui "github.com/neko233-com/godesktop"
)

// Stable VS Code extensionEditor.ts/css at 2a59476c9bfcb90b3ddc372c36762471b7dfad1c
// fixes the header and 36-DIP navbar, independently scrolling the body content.
// This is a native Viewport, not the upstream webview used for README rendering.
func (m *model) renderExtensionDetail(cx *ui.Context, e extensionInfo, installed bool) *ui.Element {
	width, availableHeight := m.extensionDetailDimensions(cx)
	title, toggle, uninstall := m.extensionDetailActions(e, installed)
	header, headerHeight := m.extensionDetailHeader(e, width, title, toggle, uninstall)
	items := []*ui.Element{
		ui.Row(label(fitExtensionLine("Extensions: "+e.Name, max(1, width-54), 13, nativeExtensionTextWidth)).PaddingXY(12, 0).Flex(1), icon("close", "extension-detail-close", func(*ui.Context) { m.closeExtensionDetails() })).Height(35).Background(ui.RGB(outer)),
		header,
	}
	if m.extensionsView.reload {
		items = append(items, ui.Row(label(fitExtensionLine("Reload to apply extension changes.", max(1, width-168), 13, nativeExtensionTextWidth)).Flex(1), button("Reload Window", "extension-detail-reload", func(*ui.Context) { m.requestRelaunch(m.workspace) }).Background(ui.RGB(accent)).Height(28)).PaddingXY(20, 4).Height(36))
		availableHeight -= 36
	}
	tabs := []*ui.Element{}
	for _, tab := range []string{"Details", "Feature Contributions"} {
		fg := uint32(muted)
		underline := ui.Color{}
		if m.extensionsView.tab == tab {
			fg = foreground
			underline = ui.RGB(accent)
		}
		// Keep full tab identities while fitting labels in a narrow editor.
		tabWidth := min(float32(180), max(1, (width-40)/2))
		tabs = append(tabs, ui.Column(button(fitExtensionLine(strings.ToUpper(tab), max(1, tabWidth-16), 11, nativeExtensionTextWidth), "extension-tab-"+tab, func(*ui.Context) { m.selectExtensionDetailTab(tab) }).FontSize(11).Foreground(ui.RGB(fg)).Width(tabWidth).Height(35), ui.Column().Height(1).Background(underline)).Width(tabWidth).Height(36))
	}
	items = append(items, ui.Row(tabs...).PaddingXY(20, 0).Height(36), rule())
	s := &m.extensionsView.detailUI
	commands := m.extensionsView.contributions[e.ID]
	s.updatePlan(e, m.extensionDetailStatus(e, installed), commands, width, nativeExtensionTextWidth)
	viewportHeight := max(0, availableHeight-35-headerHeight-37)
	s.setExtent(width, viewportHeight, s.plan.height)
	// Use the whole window as a bounded overscan ceiling. A native resize may
	// have refreshed geometry before this first new layout; taller viewports
	// still paint their entire visible range without a second input event.
	content := m.extensionDetailContent(e, commands, width, max(viewportHeight, availableHeight))
	viewport := ui.Viewport(content).ScrollOffset(0, s.scroll).Flex(1).Key("extension-detail-viewport")
	items = append(items, ui.Stack(viewport, m.extensionDetailScrollIndicator(width, viewportHeight)).Flex(1))
	return ui.Column(items...).Flex(1).Background(ui.RGB(editor)).Key("extension-detail-editor")
}

func (m *model) extensionDetailDimensions(cx *ui.Context) (float32, float32) {
	if cx == nil {
		return 640, 480
	}
	w, h := cx.WindowSize()
	width := w - 51 // activity rail, border and editor card insets
	if !m.hideSidebar {
		width -= 241
	}
	height := h - 60 // app titlebar/statusbar and editor card insets
	if m.showPanel {
		switch m.panel {
		case "TERMINAL":
			height -= m.terminalPanelHeight(h)
		case "COPILOT":
			height -= 250
		default:
			height -= 181
		}
	}
	if m.openBusy || len(m.openJobs) > 0 {
		height -= 32
	}
	if d := m.current(); d != nil && d.diskConflict != nil {
		height -= 38
	}
	if m.navigation {
		height -= 32
	}
	height -= float32(len(m.completions) * 24)
	if m.message != "" && !strings.HasPrefix(m.message, "Saved ") {
		height -= 32
	}
	return max(1, width), max(0, height)
}

func (m *model) extensionDetailHeader(e extensionInfo, width float32, title string, toggle, uninstall func(*ui.Context)) (*ui.Element, float32) {
	padding, iconSize, gap := float32(20), float32(128), float32(12)
	if width < 430 {
		iconSize = 64
	}
	if width < 220 {
		iconSize, gap = 0, 0
		padding = min(12, max(0, (width-1)/4))
	}
	detailsWidth := max(1, width-2*padding-iconSize-gap)
	description, more := wrapExtensionText(e.Description, detailsWidth, 14, 3, nativeExtensionTextWidth)
	if more && len(description) > 0 {
		description[len(description)-1] = fitExtensionLine(strings.TrimSuffix(description[len(description)-1], "…")+"…", detailsWidth, 14, nativeExtensionTextWidth)
	}
	details := []*ui.Element{
		label(fitExtensionLine(e.Name, detailsWidth, 26, nativeExtensionTextWidth)).FontSize(26).Height(32),
		label(fitExtensionLine(e.ID+"  ·  v"+e.Version, detailsWidth, 13, nativeExtensionTextWidth)).Foreground(ui.RGB(muted)).Height(22),
		ui.Column().Height(8),
	}
	for _, line := range description {
		details = append(details, label(line).FontSize(14).Height(20))
	}
	details = append(details, ui.Column().Height(10))
	actions := []*ui.Element{button(title, "extension-toggle", toggle).Height(28).Background(ui.RGB(accent)), button("Uninstall", "extension-uninstall", uninstall).Height(28).Background(ui.RGB(0x313131))}
	actionHeight := float32(28)
	if detailsWidth < 164 {
		for _, action := range actions {
			action.Width(detailsWidth)
		}
		details = append(details, ui.Column(actions...).Gap(4).Height(60))
		actionHeight = 60
	} else {
		details = append(details, ui.Row(actions...).Gap(8).Height(28))
	}
	detailsHeight := float32(32+22+8+len(description)*20+10) + actionHeight
	height := max(iconSize, detailsHeight) + 32
	row := []*ui.Element{}
	if iconSize > 0 {
		row = append(row, ui.Icon("extensions").Width(iconSize).Height(iconSize).Foreground(ui.RGB(muted)))
	}
	row = append(row, ui.Column(details...).Width(detailsWidth).Height(detailsHeight))
	// Explicit top/bottom spacers preserve the upstream asymmetric 20/12 inset.
	return ui.Column(ui.Column().Height(20), ui.Row(row...).Gap(gap).PaddingXY(padding, 0).Height(height-32), ui.Column().Height(12)).Height(height).Key("extension-detail-header"), height
}

func (m *model) extensionDetailContent(e extensionInfo, commands []extensionCommand, width, visibleHeight float32) *ui.Element {
	s := &m.extensionsView.detailUI
	start, end, before, after := s.plan.visibleRows(s.scroll, visibleHeight)
	textWidth := max(1, width-2*extensionDetailPadding-12)
	children := []*ui.Element{ui.Column().Height(before)}
	for _, row := range s.plan.rows[start:end] {
		if row.command >= 0 {
			children = append(children, m.extensionDetailCommand(e.ID, commands[row.command], row.key, textWidth))
			continue
		}
		text := label(fitExtensionLine(row.text, textWidth, row.size, nativeExtensionTextWidth)).FontSize(row.size).Height(row.height).Width(textWidth)
		if row.muted {
			text.Foreground(ui.RGB(muted))
		}
		children = append(children, text)
	}
	children = append(children, ui.Column().Height(after))
	return ui.Column(children...).PaddingXY(extensionDetailPadding, 0).Width(width).Height(s.plan.height).Key("extension-detail-content")
}

func (m *model) extensionDetailCommand(extensionID string, command extensionCommand, key string, width float32) *ui.Element {
	run := m.extensionDetailCommandAction(extensionID, command.ID)
	text := command.Title
	if text == "" {
		text = command.ID
	}
	lines, more := wrapExtensionText(text, max(1, width-16), 13, 2, nativeExtensionTextWidth)
	if more && len(lines) > 0 {
		lines[len(lines)-1] = fitExtensionLine(lines[len(lines)-1]+"…", max(1, width-16), 13, nativeExtensionTextWidth)
	}
	labels := []*ui.Element{}
	color := uint32(foreground)
	if run == nil {
		color = 0x6e6e6e
	}
	for _, line := range lines {
		labels = append(labels, label(line).Foreground(ui.RGB(color)).Height(20))
	}
	return ui.Column(ui.Column(labels...).PaddingXY(8, 0).Width(width).Height(42).HoverBackground(ui.RGB(0x333333)).Radius(4).FocusRing(false).Key(key).OnClick(run), label(fitExtensionLine(command.ID, width, 11, nativeExtensionTextWidth)).FontSize(11).Foreground(ui.RGB(muted)).Height(20), ui.Column().Height(4)).Height(extensionDetailCommandHeight).Width(width)
}

func (m *model) extensionDetailCommandAction(extensionID, commandID string) func(*ui.Context) {
	if !m.extensionsView.running[strings.ToLower(extensionID)] || m.execute == nil {
		return nil
	}
	return func(*ui.Context) { m.execute(commandID) }
}

func (m *model) extensionDetailScrollIndicator(width, height float32) *ui.Element {
	s := &m.extensionsView.detailUI
	if height <= 0 || s.maxScroll() <= 0 {
		return nil
	}
	thumb := min(height, max(float32(24), height*height/s.contentHeight))
	top := s.scroll / s.maxScroll() * (height - thumb)
	return ui.Row(spacer(), ui.Column(ui.Column().Height(top), ui.Column().Width(6).Height(thumb).Radius(3).Background(ui.RGBA(0x797979, .45)).Key("extension-detail-scroll-thumb"), spacer()).Width(8)).Height(height)
}
