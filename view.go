package main

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

const (
	outer      = 0x181818
	editor     = 0x1f1f1f
	border     = 0x2b2b2b
	foreground = 0xcccccc
	muted      = 0x9d9d9d
	accent     = 0x0078d4
)

func label(s string) *ui.Element { return ui.Text(s).FontSize(13).Foreground(ui.RGB(foreground)) }
func spacer() *ui.Element        { return ui.Column().Flex(1) }
func rule() *ui.Element          { return ui.Column().Height(1).Background(ui.RGB(border)) }
func button(text, key string, click func(*ui.Context)) *ui.Element {
	fg := uint32(foreground)
	if click == nil {
		fg = 0x6e6e6e
	}
	return ui.Button(text, click).Key(key).FontSize(13).PaddingXY(8, 0).Radius(4).HoverBackground(ui.RGB(0x333333)).Background(ui.Color{}).Foreground(ui.RGB(fg)).FocusRing(false)
}
func icon(name, key string, click func(*ui.Context)) *ui.Element {
	return ui.Icon(name).Width(30).Height(30).Radius(4).HoverBackground(ui.RGB(0x333333)).Key(key).Foreground(ui.RGB(foreground)).OnClick(click)
}
func codeFont() string {
	if runtime.GOOS == "darwin" {
		return "Menlo"
	}
	return "Consolas"
}

func (m *model) view(cx *ui.Context) *ui.Element {
	m.observeAutoSaveFocus(time.Now())
	defer func() {
		if m.publishEditors != nil {
			m.publishEditors(0)
		}
	}()
	_, height := cx.WindowSize()
	bar := m.titlebar(cx)
	activities := []*ui.Element{}
	for _, name := range []string{"files", "search", "source-control", "debug", "extensions"} {
		color := uint32(muted)
		indicator := ui.Color{}
		if m.activity == name {
			color = foreground
			indicator = ui.RGB(0x333333)
		}
		activities = append(activities, ui.Column(ui.Icon(name).Width(44).Height(44).Foreground(ui.RGB(color))).Padding(2).Height(48).Radius(4).Background(indicator).HoverBackground(ui.RGB(0x2b2b2b)).Key("activity-"+name).OnClick(func(c *ui.Context) {
			if name == "search" {
				m.showSearch(c)
			} else if name == "source-control" {
				m.showSCM()
			} else if name == "extensions" {
				m.showExtensions()
			} else {
				m.hideSidebar = false
				m.activity = name
				m.palette = false
				m.query = ""
			}
		}))
	}
	activities = append(activities, spacer(), ui.Icon("account").Height(48).Foreground(ui.RGB(muted)), ui.Icon("settings").Height(48).Foreground(ui.RGB(muted)).Key("settings").OnClick(func(*ui.Context) { m.activity = "settings"; m.palette = false; m.updateFocused = false }))
	activity := ui.Column(activities...).Width(48).Background(ui.RGB(outer))
	crumb := "Welcome"
	if d := m.current(); d != nil {
		rel, err := filepath.Rel(m.workspace, d.path)
		if err != nil {
			rel = d.path
		}
		crumb = strings.ReplaceAll(filepath.ToSlash(rel), "/", "  ›  ")
	}
	parts := []*ui.Element{}
	if m.groups.root == nil && m.extensionsView.detail == "" {
		parts = append(parts, m.tabsView(cx), ui.Row(label(crumb).PaddingXY(10, 0), spacer()).Height(24))
	}
	if m.openBusy || len(m.openJobs) > 0 {
		parts = append(parts, ui.Row(label("Opening "+filepath.Base(m.openingPath)+"…").Flex(1), button("Cancel", "cancel-open", func(*ui.Context) {
			if m.cancelPendingOpens != nil {
				m.cancelPendingOpens()
			}
		})).PaddingXY(8, 0).Height(32).Background(ui.RGB(0x252526)))
	}
	if d := m.current(); d != nil && d.diskConflict != nil {
		parts = append(parts, m.diskBanner(d))
	}
	if m.navigation {
		parts = append(parts, label("Go to line or :byte offset: "+m.query+"▏").Height(32).Padding(8).Background(ui.RGB(0x313131)))
	}
	if len(m.completions) > 0 {
		items := []*ui.Element{}
		for i, suggestion := range m.completions {
			items = append(items, button(suggestion.item.title(), fmt.Sprintf("completion-%d", i), func(*ui.Context) { m.chooseCompletion(suggestion) }).Height(24))
		}
		parts = append(parts, ui.Column(items...).Background(ui.RGB(0x252526)))
	}
	if m.message != "" && !strings.HasPrefix(m.message, "Saved ") {
		parts = append(parts, ui.Row(label(m.message).Padding(8).Flex(1), icon("close", "dismiss", func(*ui.Context) { m.message = "" })).Height(32).Background(ui.RGB(0x252526)))
	}
	panelHeight := float32(0)
	if m.showPanel {
		panelHeight = 181
		if m.panel == "TERMINAL" {
			panelHeight = m.terminalPanelHeight(height)
		}
		if m.panel == "COPILOT" {
			panelHeight = 250
		}
	}
	visible := max(1, int((height-36-35-24-22-panelHeight)/20))
	if m.openBusy || len(m.openJobs) > 0 {
		visible = max(1, visible-2)
	}
	if m.message != "" && !strings.HasPrefix(m.message, "Saved ") {
		visible = max(1, visible-2)
	}
	visible = max(1, visible-len(m.completions)*24/20)
	if d := m.current(); d != nil && d.diskConflict != nil {
		visible = max(1, visible-2)
	}
	if m.extensionsView.detail != "" {
		parts = append(parts, m.extensionDetailView(cx))
	} else if m.scm.diff != nil {
		parts = append(parts, m.scmDiffView(cx, visible).Flex(1))
	} else if m.groups.root == nil {
		parts = append(parts, m.codeView(visible).Flex(1).Key("editor-content"))
	} else {
		width, _ := cx.WindowSize()
		parts = append(parts, m.editorGroupsView(cx, max(1, width-290), max(1, float32(visible*20)+59)).Flex(1))
	}
	editorCard := ui.Column(ui.Column(parts...).Flex(1).ClipRounded(7).Background(ui.RGB(editor))).Padding(1).ClipRounded(8).Background(ui.RGB(border)).Flex(1).Key("workbench-editor-card")
	parts = []*ui.Element{editorCard}
	if m.showPanel {
		width, _ := cx.WindowSize()
		parts = append(parts, m.panelView(max(120, width-290)))
	}
	center := ui.Column(parts...).Flex(1).Background(ui.RGB(outer))
	bodyItems := []*ui.Element{activity, ui.Column().Width(1).Background(ui.RGB(border))}
	if !m.hideSidebar {
		bodyItems = append(bodyItems, m.sidebar(cx).Width(240), ui.Column().Width(1).Background(ui.RGB(border)))
	}
	bodyItems = append(bodyItems, center)
	body := ui.Row(bodyItems...).Flex(1)
	base := ui.Column(bar, body, m.statusbar()).Background(ui.RGB(outer))
	if m.closePrompt {
		return m.closeOverlay(cx, base)
	}
	if m.reloadPrompt != nil {
		return m.reloadOverlay(base)
	}
	if m.history.prompt != nil {
		return m.historyOverlay(base)
	}
	if m.menu.name != "" {
		return m.menuOverlay(cx, base)
	}
	if m.palette {
		return m.quickOverlay(cx, base)
	}
	return base
}

func (m *model) titlebar(cx *ui.Context) *ui.Element {
	width, _ := cx.WindowSize()
	searchWidth := min(float32(400), max(160, width*.35))
	searchX := (width - searchWidth) / 2
	leftWidth := float32(35)
	if runtime.GOOS == "darwin" {
		leftWidth = 80
	}
	menuWidths := make([]float32, len(workbenchMenuNames))
	total := leftWidth
	for i, name := range workbenchMenuNames {
		w, _ := ui.MeasureText(name, 13, "")
		menuWidths[i] = w + 16
		total += menuWidths[i]
	}
	m.menu.visible = nil
	m.menu.overflow = nil
	for i, name := range workbenchMenuNames {
		if total > searchX-8 && leftWidth+menuWidths[i] > searchX-42 {
			m.menu.overflow = append(m.menu.overflow, workbenchMenuNames[i:]...)
			break
		}
		m.menu.visible = append(m.menu.visible, name)
		leftWidth += menuWidths[i]
	}
	if len(m.menu.overflow) > 0 {
		m.menu.visible = append(m.menu.visible, "More")
	}
	left := []*ui.Element{ui.Image(m.logo).Width(35).Height(35).Padding(9.5)}
	if runtime.GOOS == "darwin" {
		left = []*ui.Element{ui.Column().Width(80)}
	}
	for _, menu := range m.menu.visible {
		bg := ui.Color{}
		if m.menu.name == menu {
			bg = ui.RGB(0x333333)
		}
		title := menu
		if menu == "More" {
			title = "…"
		}
		left = append(left, button(title, "menu-"+menu, func(c *ui.Context) {
			if m.menu.name == menu {
				m.closeMenu()
			} else {
				m.openMenu(c, menu)
			}
		}).PaddingXY(8, 0).Height(35).Background(bg))
	}
	search := ui.Row(ui.Icon("search").Width(24).Height(20).Foreground(ui.RGB(muted)), label(filepath.Base(m.workspace)).Flex(1)).PaddingXY(4, 0).Width(searchWidth).Height(28).Background(ui.RGB(0x242424)).Radius(5).Key("command-center").OnClick(func(*ui.Context) { m.openQuickInput(false) })
	controls := ui.Row(icon("minus", "window-minimize", func(*ui.Context) { cx.Minimize() }).Width(46), icon("maximize", "window-maximize", func(*ui.Context) { cx.ToggleMaximize() }).Width(46), icon("close", "window-close", func(*ui.Context) { cx.RequestClose() }).Width(46))
	if runtime.GOOS == "darwin" {
		controls = ui.Row().Width(30)
	}
	base := ui.Row(ui.Row(left...), spacer().Draggable(), controls).Height(35)
	center := ui.Row(ui.Column().Width(searchX), ui.Column(ui.Column().Height(3.5), search).Height(35), spacer()).Height(35)
	return ui.Column(ui.Stack(base, center).Height(35), rule()).Height(36).Background(ui.RGB(outer))
}
func (m *model) sidebar(cx *ui.Context) *ui.Element {
	title := map[string]string{"files": "EXPLORER", "search": "SEARCH", "source-control": "SOURCE CONTROL", "debug": "RUN AND DEBUG", "extensions": "EXTENSIONS", "settings": "SETTINGS"}[m.activity]
	children := []*ui.Element{ui.Row(label(title).FontSize(11).PaddingXY(18, 0).Flex(1), label("…").Width(28)).Height(35)}
	switch m.activity {
	case "settings":
		children = append(children, ui.Viewport(ui.Column(m.keyboardSidebar(), m.autoSaveSidebar(), m.updatesSidebar())).ScrollOffset(0, m.settingsScroll).Flex(1).Key("settings-content"))
	case "files":
		children = append(children, ui.Row(ui.Icon("chevron-down").Width(22).Height(22), label(strings.ToUpper(filepath.Base(m.workspace))).FontSize(11)).Height(24))
		if m.workspaceBusy {
			children = append(children, ui.Row(label(m.workspaceStatus).Flex(1), button("Cancel", "cancel-workspace-scan", func(*ui.Context) {
				if m.cancelWorkspace != nil {
					m.cancelWorkspace()
				}
			})).Height(26))
		} else if m.workspaceStatus != "" {
			children = append(children, label(m.workspaceStatus).FontSize(11).Foreground(ui.RGB(muted)).PaddingXY(12, 0).Height(24))
		}
		lastDirectory := ""
		for _, path := range m.files {
			if len(children) > 40 {
				break
			}
			dir := filepath.ToSlash(filepath.Dir(path))
			indent := float32(12)
			if dir != "." {
				indent = 24
				if dir != lastDirectory {
					children = append(children, ui.Row(ui.Column().Width(12), ui.Icon("chevron-down").Width(18).Height(22), label(dir)).Height(22))
					lastDirectory = dir
				}
			}
			bg := ui.Color{}
			if d := m.current(); d != nil && filepath.Join(m.workspace, filepath.FromSlash(path)) == d.path {
				bg = ui.RGB(0x37373d)
			}
			fileIcon := "◇"
			color := uint32(0xc5c5c5)
			if strings.HasSuffix(path, ".go") {
				fileIcon = "{}"
				color = 0x519aba
			}
			if strings.HasSuffix(path, ".json") {
				fileIcon = "{}"
				color = 0xdcb67a
			}
			children = append(children, ui.Row(ui.Column().Width(indent), label(fileIcon).Width(24).Foreground(ui.RGB(color)), label(filepath.Base(path)).Flex(1)).Height(22).Background(bg).Key("file-"+path).OnClick(func(*ui.Context) { m.open(filepath.Join(m.workspace, filepath.FromSlash(path))) }))
		}
	case "search":
		children = append(children, m.searchSidebar(cx))
	case "extensions":
		children = append(children, m.extensionsSidebar(cx).Key("extensions-sidebar"))
	case "source-control":
		children = append(children, m.scmSidebar(cx))
	case "debug":
		children = append(children, label("No debug adapter configured.").PaddingXY(12, 0).Height(32), button("Open command palette", "debug-palette", func(*ui.Context) { m.palette = true }).Height(32))
	}
	if m.activity != "settings" && m.activity != "search" && m.activity != "source-control" && m.activity != "extensions" {
		children = append(children, spacer())
	}
	if m.activity == "files" {
		children = append(children, rule(), label("›  OUTLINE").PaddingXY(8, 0).Height(24), rule(), label("›  TIMELINE").PaddingXY(8, 0).Height(24))
	}
	return ui.Column(children...).Background(ui.RGB(outer))
}
func (m *model) codeView(visible int) *ui.Element {
	d := m.current()
	selection := textbuffer.Selection{}
	if d != nil && d.buffer != nil {
		selection = d.buffer.Selection()
	}
	width := float32(900)
	if m.native != nil {
		w, _ := m.native.WindowSize()
		width = max(1, w-290)
	}
	return m.codeViewFor(d, visible, width, selection, m.editing, func(key string) string { return key }, func(line int) {
		m.moveCursor(d, line, hitColumn(d.buffer.Line(line), max(0, m.pointerX-358)), m.pointerShift)
		m.editing = true
	}, func(line, offset int64, bytes bool) { m.navigateLarge(d, line, offset, bytes) }, func() { m.navigation = true; m.query = "" })
}
func (m *model) codeViewFor(d *document, visible int, width float32, selected textbuffer.Selection, editing bool, key func(string) string, click func(int), navigate func(int64, int64, bool), goTo func()) *ui.Element {
	if d == nil {
		return ui.Column(spacer(), label("gocode").FontSize(44).Foreground(ui.RGB(0x555555)), label("Open a file in Explorer to start editing."), spacer()).Padding(40)
	}
	if d.large != nil {
		return m.largeCodeViewFor(d, visible, width, key, navigate, goTo)
	}
	if editing && !d.holdScroll && d.line >= d.scroll+visible {
		d.scroll = max(0, d.line-visible+1)
	}
	rows := []*ui.Element{}
	selection := selected.Range()
	for i := d.scroll; i < d.buffer.LineCount() && i < d.scroll+visible; i++ {
		line := d.buffer.Line(i)
		runes := codeLinePreview(line, 400)
		line = string(runes)
		segments := []*ui.Element{}
		for _, f := range highlight(line) {
			segments = append(segments, ui.Text(f.text).FontFamily(codeFont()).FontSize(14).Foreground(ui.RGB(f.color)))
		}
		segments = append(segments, spacer())
		bg := ui.Color{}
		if i == d.line {
			bg = ui.RGB(0x282828)
		}
		layers := []*ui.Element{}
		if i >= selection.Start.Line && i <= selection.End.Line && selection.Start != selection.End {
			start, end := 0, len([]rune(line))
			if i == selection.Start.Line {
				start, _ = d.buffer.RuneColumn(selection.Start)
			}
			if i == selection.End.Line {
				end, _ = d.buffer.RuneColumn(selection.End)
			}
			x, _ := ui.MeasureText(strings.ReplaceAll(string([]rune(line)[:min(start, len([]rune(line)))]), "\t", "    "), 14, codeFont())
			z, _ := ui.MeasureText(strings.ReplaceAll(string([]rune(line)[:min(end, len([]rune(line)))]), "\t", "    "), 14, codeFont())
			layers = append(layers, ui.Row(ui.Column().Width(x), ui.Column().Width(max(2, z-x)).Height(20).Background(ui.RGB(0x264f78)), spacer()))
		}
		layers = append(layers, ui.Row(segments...))
		if editing && i == d.line {
			x, _ := ui.MeasureText(strings.ReplaceAll(string([]rune(line)[:min(d.column, len([]rune(line)))]), "\t", "    "), 14, codeFont())
			layers = append(layers, ui.Row(ui.Column().Width(x), ui.Column().Width(1).Height(20).Background(ui.RGB(0xaeafad)), spacer()))
			if ghost := ghostText(d, m.suggestion); ghost != "" {
				layers = append(layers, ui.Row(ui.Column().Width(x), ui.Text(ghost).FontFamily(codeFont()).FontSize(14).Foreground(ui.RGB(0x777777)), spacer()))
			}
		}
		rows = append(rows, ui.Row(ui.Text(fmt.Sprintf("%4d", i+1)).FontFamily(codeFont()).FontSize(14).Foreground(ui.RGB(0x858585)).Width(52), ui.Column().Width(16), ui.Stack(layers...).Flex(1)).Height(20).Background(bg).Key(key(fmt.Sprintf("code-line-%d", i))).OnClick(func(*ui.Context) { click(i) }))
	}
	rows = append(rows, spacer())
	minimap := []*ui.Element{ui.Column().Height(4)}
	for i := 0; i < min(100, d.buffer.LineCount()); i++ {
		line := d.buffer.Line(i)
		width := float32(runeCountUpTo(strings.TrimSpace(line), 70))
		color := uint32(0x484848)
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			color = 0x3e5037
		}
		minimap = append(minimap, ui.Column().Width(width).Height(2).Background(ui.RGB(color)), ui.Column().Height(2))
	}
	minimap = append(minimap, spacer())
	return ui.Row(ui.Column(rows...).Flex(1), ui.Column(minimap...).Width(84).Padding(6), ui.Column().Width(10).Background(ui.RGB(0x252526)))
}
func (m *model) panelView(width float32) *ui.Element {
	header := []*ui.Element{}
	for _, name := range []string{"PROBLEMS", "OUTPUT", "DEBUG CONSOLE", "TERMINAL", "PORTS", "COPILOT"} {
		color := uint32(muted)
		if m.panel == name {
			color = foreground
		}
		header = append(header, button(name, "panel-"+name, func(*ui.Context) { m.panel = name }).FontSize(11).Foreground(ui.RGB(color)))
	}
	header = append(header, spacer(), icon("chevron-down", "panel-toggle", func(*ui.Context) { m.hidePanel() }), icon("close", "panel-close", func(*ui.Context) { m.hidePanel() }))
	if m.panel == "TERMINAL" {
		windowHeight := float32(820)
		if m.native != nil {
			_, windowHeight = m.native.WindowSize()
		}
		height := m.terminalPanelHeight(windowHeight)
		return ui.Column(ui.Column().Height(4).Background(ui.RGB(border)).Key("panel-resize").OnClick(func(*ui.Context) {}), ui.Row(header...).Height(34), m.terminalBody(width, height).Flex(1)).Height(height).Background(ui.RGB(outer))
	}
	lines := []*ui.Element{}
	if m.panel == "COPILOT" {
		status := m.copilotStatus
		if m.chatBusy {
			status = "Copilot is responding…"
		} else if m.chatLoginBusy {
			status = "Finish Copilot chat sign-in in your browser"
		}
		lines = append(lines, label(status).FontSize(11).Height(20))
		answer := wrapChatText(m.chatAnswer, width)
		for _, line := range answer[max(0, len(answer)-6):] {
			lines = append(lines, label(line).Height(20))
		}
		lines = append(lines, spacer())
		if m.chatContextLabel != "" {
			lines = append(lines, ui.Row(label("Attached: "+m.chatContextLabel).Flex(1), button("Remove", "copilot-remove-context", func(*ui.Context) { m.chatContext = ""; m.chatContextLabel = "" })).Height(20))
		}
		lines = append(lines, ui.Row(label("Ask Copilot: "+m.chatPrompt+"▏").Flex(1).Key("copilot-input").OnClick(func(*ui.Context) { m.chatFocused = true; m.editing = false }), button("Send", "copilot-send", func(*ui.Context) {
			if m.askChat != nil && !m.chatBusy && m.chatPrompt != "" {
				prompt := m.chatPrompt
				m.chatPrompt = ""
				m.askChat(prompt)
			}
		}), button("Cancel", "copilot-cancel", func(*ui.Context) {
			if m.cancelChat != nil {
				m.cancelChat()
			}
		}), button("Selection", "copilot-attach-selection", func(*ui.Context) { m.attachChatContext(true) }), button("File", "copilot-attach-file", func(*ui.Context) { m.attachChatContext(false) }), button("Copy", "copilot-copy", func(*ui.Context) {
			if m.writeClipboard != nil {
				if err := m.writeClipboard(m.chatAnswer); err != nil {
					m.message = err.Error()
				}
			}
		}), button("Completion sign in", "copilot-sign-in", func(*ui.Context) {
			if m.signInCopilot != nil {
				m.signInCopilot()
			}
		}), button("Chat sign in", "copilot-chat-sign-in", func(*ui.Context) {
			if m.signInChat != nil {
				m.signInChat()
			}
		})).Height(28))
	} else if m.panel == "OUTPUT" {
		for _, line := range m.output[max(0, len(m.output)-5):] {
			lines = append(lines, label(strings.TrimSuffix(line, "\n")).FontFamily(codeFont()).Height(20))
		}
	} else if m.panel == "PROBLEMS" {
		problems := m.problems()
		for i, p := range problems[:min(5, len(problems))] {
			lines = append(lines, button(fmt.Sprintf("%s  %s:%d:%d", p.Message, filepath.Base(p.Path), p.Range.Start.Line+1, p.Range.Start.Character+1), fmt.Sprintf("problem-%d", i), func(*ui.Context) {
				m.openThen(context.Background(), p.Path, func(d *document, err error) {
					if err != nil || d == nil || d.buffer == nil {
						return
					}
					column, _ := d.buffer.RuneColumn(p.Range.Start)
					m.moveCursor(d, p.Range.Start.Line, column, false)
					m.editing = true
				})
			}).Height(20))
		}
		if len(problems) == 0 {
			lines = append(lines, label("No problems reported by extensions.").Foreground(ui.RGB(muted)).Height(20))
		}
	} else {
		lines = append(lines, label("No entries.").Foreground(ui.RGB(muted)).Height(20))
	}
	lines = append(lines, spacer())
	height := float32(181)
	if m.panel == "COPILOT" {
		height = 250
	}
	return ui.Column(rule(), ui.Row(header...).Height(34), ui.Column(lines...).Padding(12).Flex(1)).Height(height).Background(ui.RGB(editor))
}
func (m *model) statusbar() *ui.Element {
	errors, warnings := m.diagnosticCounts()
	position := "Ln 1, Col 1"
	language := "Plain Text"
	lineEnding := "LF"
	if d := m.current(); d != nil {
		position = fmt.Sprintf("Ln %d, Col %d", d.line+1, d.column+1)
		if d.buffer != nil && d.buffer.EOL() == "\r\n" {
			lineEnding = "CRLF"
		}
		if d.large != nil {
			language = "Large file"
			lineEnding = d.large.stats.EOL
		}
		if strings.HasSuffix(d.path, ".go") {
			language = "Go"
		}
	}
	return ui.Column(rule(), ui.Row(label(" >< ").Width(34).Background(ui.RGB(accent)), label(fmt.Sprintf("  × %d   ⚠ %d", errors, warnings)).Width(180).Key("status-problems").OnClick(func(*ui.Context) { m.panel = "PROBLEMS"; m.showPanel = true }), label(strings.TrimSpace(m.status+"  "+m.lspStatus)).Flex(1), label(position).Width(105), label("Spaces: 4").Width(80), label("UTF-8").Width(55), label(lineEnding).Width(42), label(language).Width(60), icon("terminal", "status-panel", func(*ui.Context) { m.togglePanel() }).Width(28)).Height(21)).Height(22).Background(ui.RGB(outer))
}

func hitColumn(line string, x float32) int {
	r := []rune(line)
	low, high := 0, len(r)
	for low < high {
		mid := (low + high) / 2
		a, _ := ui.MeasureText(strings.ReplaceAll(string(r[:mid]), "\t", "    "), 14, codeFont())
		z, _ := ui.MeasureText(strings.ReplaceAll(string(r[:mid+1]), "\t", "    "), 14, codeFont())
		if x < (a+z)/2 {
			high = mid
		} else {
			low = mid + 1
		}
	}
	return low
}
