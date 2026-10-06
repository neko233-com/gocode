package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

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
	return ui.Button(text, click).Key(key).FontSize(13).PaddingXY(8, 0).Radius(0).Background(ui.Color{}).Foreground(ui.RGB(foreground))
}
func icon(name, key string, click func(*ui.Context)) *ui.Element {
	return ui.Icon(name).Width(30).Height(30).Key(key).Foreground(ui.RGB(foreground)).OnClick(click)
}
func codeFont() string {
	if runtime.GOOS == "darwin" {
		return "Menlo"
	}
	return "Consolas"
}

func (m *model) view(cx *ui.Context) *ui.Element {
	_, height := cx.WindowSize()
	bar := m.titlebar(cx)
	activities := []*ui.Element{}
	for _, name := range []string{"files", "search", "source-control", "debug", "extensions"} {
		color := uint32(muted)
		indicator := ui.Color{}
		if m.activity == name {
			color = foreground
			indicator = ui.RGB(foreground)
		}
		activities = append(activities, ui.Row(ui.Column().Width(2).Background(indicator), ui.Icon(name).Width(46).Height(48).Foreground(ui.RGB(color))).Height(48).Key("activity-"+name).OnClick(func(*ui.Context) { m.activity = name; m.palette = false; m.query = "" }))
	}
	activities = append(activities, spacer(), ui.Icon("account").Height(48).Foreground(ui.RGB(muted)), ui.Icon("settings").Height(48).Foreground(ui.RGB(muted)).Key("settings").OnClick(func(*ui.Context) { m.palette = true; m.query = "" }))
	activity := ui.Column(activities...).Width(48).Background(ui.RGB(outer))
	tabs := []*ui.Element{}
	for i, d := range m.docs {
		bg := uint32(outer)
		top := ui.Color{}
		if i == m.active {
			bg = editor
			top = ui.RGB(accent)
		}
		name := filepath.Base(d.path)
		symbol := "×"
		if d.dirty() {
			symbol = "●"
		}
		tab := ui.Column(ui.Column().Height(1).Background(top), ui.Row(label("{} ").Foreground(ui.RGB(0x519aba)).Width(28), label(name).Flex(1), button(symbol, fmt.Sprintf("close-%d", i), func(*ui.Context) { m.closeTab(i) }).Width(26)).Padding(5).Height(34)).Width(float32(70 + len([]rune(name))*7)).Background(ui.RGB(bg)).Key(fmt.Sprintf("tab-%d", i)).OnClick(func(*ui.Context) { m.active = i; m.documentEvent("focus", d, textbuffer.ChangeEvent{}) })
		tabs = append(tabs, tab, ui.Column().Width(1).Background(ui.RGB(border)))
	}
	tabs = append(tabs, spacer(), icon("split", "split", func(*ui.Context) { m.message = "Split editors are not implemented in this preview" }))
	crumb := "Welcome"
	if d := m.current(); d != nil {
		rel, err := filepath.Rel(m.workspace, d.path)
		if err != nil {
			rel = d.path
		}
		crumb = strings.ReplaceAll(filepath.ToSlash(rel), "/", "  ›  ")
	}
	parts := []*ui.Element{ui.Row(tabs...).Height(35).Background(ui.RGB(outer)), ui.Row(label(crumb).PaddingXY(10, 0), spacer()).Height(24)}
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
	if m.palette {
		items := []*ui.Element{label("> " + m.query + "▏").Height(32).Padding(8).Background(ui.RGB(0x313131))}
		items = append(items, button("File: Save active document", "palette-save", func(*ui.Context) {
			if err := m.save(); err != nil {
				m.message = err.Error()
			}
			m.palette = false
		}).Height(28), button("View: Toggle panel", "palette-panel", func(*ui.Context) { m.showPanel = !m.showPanel; m.palette = false }).Height(28))
		items = append(items, button("Copilot: Open Chat (Ctrl/Cmd+I)", "palette-copilot-chat", func(*ui.Context) {
			m.panel = "COPILOT"
			m.showPanel = true
			m.chatFocused = true
			m.palette = false
			m.editing = false
		}).Height(28), button("Copilot: Sign in for completion", "palette-copilot-sign-in", func(*ui.Context) {
			if m.signInCopilot != nil {
				m.signInCopilot()
			}
			m.palette = false
		}).Height(28))
		items = append(items, button("Copilot: Sign in for chat", "palette-copilot-chat-sign-in", func(*ui.Context) {
			if m.signInChat != nil {
				m.signInChat()
			}
			m.palette = false
		}).Height(28))
		for _, action := range []struct{ title, method string }{{"Format Document (Shift+Alt+F)", "textDocument/formatting"}, {"Go to Definition (F12)", "textDocument/definition"}, {"Show Hover (Ctrl/Cmd+K)", "textDocument/hover"}} {
			items = append(items, button(action.title, "palette-"+action.method, func(*ui.Context) {
				if m.requestLSP != nil {
					m.requestLSP(m.current(), action.method)
				}
				m.palette = false
			}).Height(28))
		}
		for _, command := range m.commands {
			if strings.Contains(strings.ToLower(command.Title), strings.ToLower(m.query)) {
				items = append(items, button(command.Title, "palette-"+command.ID, func(*ui.Context) { m.execute(command.ID); m.palette = false }).Height(28))
			}
		}
		items = append(items, button("Close command palette  (Esc)", "palette-close", func(*ui.Context) { m.palette = false }).Height(28))
		parts = append(parts, ui.Column(items...).Padding(6).Background(ui.RGB(0x252526)))
	}
	if m.message != "" {
		parts = append(parts, ui.Row(label(m.message).Padding(8).Flex(1), icon("close", "dismiss", func(*ui.Context) { m.message = "" })).Height(32).Background(ui.RGB(0x252526)))
	}
	panelHeight := float32(0)
	if m.showPanel {
		panelHeight = 181
		if m.panel == "COPILOT" {
			panelHeight = 250
		}
	}
	visible := max(1, int((height-36-35-24-22-panelHeight)/20))
	if m.message != "" {
		visible = max(1, visible-2)
	}
	if m.palette {
		visible = max(1, visible-10)
	}
	visible = max(1, visible-len(m.completions)*24/20)
	parts = append(parts, m.codeView(visible).Flex(1))
	if m.showPanel {
		width, _ := cx.WindowSize()
		parts = append(parts, m.panelView(max(120, width-318)))
	}
	center := ui.Column(parts...).Flex(1).Background(ui.RGB(editor))
	body := ui.Row(activity, ui.Column().Width(1).Background(ui.RGB(border)), m.sidebar().Width(240), ui.Column().Width(1).Background(ui.RGB(border)), center).Flex(1)
	return ui.Column(bar, body, m.statusbar()).Background(ui.RGB(outer))
}

func (m *model) titlebar(cx *ui.Context) *ui.Element {
	left := []*ui.Element{ui.Icon("debug").Width(36).Foreground(ui.RGB(0x23a9f2))}
	if runtime.GOOS == "darwin" {
		left = []*ui.Element{ui.Column().Width(80)}
	}
	for _, menu := range []string{"File", "Edit", "Selection", "View", "Go", "Run", "…"} {
		left = append(left, button(menu, "menu-"+menu, func(*ui.Context) { m.palette = true; m.query = "" }).Padding(6))
	}
	search := ui.Row(ui.Icon("search").Width(24).Height(24).Foreground(ui.RGB(muted)), label(filepath.Base(m.workspace)).Flex(1)).Padding(4).Width(400).Height(28).Background(ui.RGB(0x242424)).Radius(5).Key("command-center").OnClick(func(*ui.Context) { m.palette = !m.palette; m.query = "" })
	controls := ui.Row(icon("minus", "window-minimize", func(*ui.Context) { cx.Minimize() }).Width(46), icon("maximize", "window-maximize", func(*ui.Context) { cx.ToggleMaximize() }).Width(46), icon("close", "window-close", func(*ui.Context) { cx.Quit() }).Width(46))
	if runtime.GOOS == "darwin" {
		controls = ui.Row().Width(30)
	}
	return ui.Column(ui.Row(ui.Row(left...), spacer().Draggable(), search, spacer().Draggable(), controls).Height(35), rule()).Height(36).Background(ui.RGB(outer))
}
func (m *model) sidebar() *ui.Element {
	title := map[string]string{"files": "EXPLORER", "search": "SEARCH", "source-control": "SOURCE CONTROL", "debug": "RUN AND DEBUG", "extensions": "EXTENSIONS"}[m.activity]
	children := []*ui.Element{ui.Row(label(title).FontSize(11).PaddingXY(18, 0).Flex(1), label("…").Width(28)).Height(35)}
	switch m.activity {
	case "files":
		children = append(children, ui.Row(ui.Icon("chevron-down").Width(22).Height(22), label(strings.ToUpper(filepath.Base(m.workspace))).FontSize(11)).Height(24))
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
		children = append(children, label("Search files by name").PaddingXY(12, 0).Height(30), label(m.query+"▏").PaddingXY(12, 0).Height(30).Background(ui.RGB(0x313131)))
		if m.query != "" {
			for _, path := range m.files {
				if strings.Contains(strings.ToLower(path), strings.ToLower(m.query)) {
					children = append(children, button(path, "search-"+path, func(*ui.Context) {
						m.open(filepath.Join(m.workspace, filepath.FromSlash(path)))
						m.activity = "files"
						m.query = ""
					}).Height(26))
				}
			}
		}
	case "extensions":
		children = append(children, label("INSTALLED").PaddingXY(12, 0).Height(28))
		for _, e := range m.installed {
			children = append(children, ui.Row(ui.Icon("extensions").Width(42).Height(48), ui.Column(label(e.Name).FontSize(14), label(e.Description).Foreground(ui.RGB(muted)), label(e.ID+"  "+e.Version).FontSize(11).Foreground(ui.RGB(muted))).Gap(3).Flex(1)).Height(72).Padding(6))
		}
		children = append(children, rule(), label("EXTENSION COMMANDS").PaddingXY(12, 0).Height(28))
		for _, c := range m.commands {
			children = append(children, button(c.Title, "extension-"+c.ID, func(*ui.Context) { m.execute(c.ID) }).Height(32))
		}
	case "source-control":
		children = append(children, label("Source control is planned.").PaddingXY(12, 0).Height(32), label("Open files remain editable.").PaddingXY(12, 0).Height(32))
	case "debug":
		children = append(children, label("No debug adapter configured.").PaddingXY(12, 0).Height(32), button("Open command palette", "debug-palette", func(*ui.Context) { m.palette = true }).Height(32))
	}
	children = append(children, spacer())
	if m.activity == "files" {
		children = append(children, rule(), label("›  OUTLINE").PaddingXY(8, 0).Height(24), rule(), label("›  TIMELINE").PaddingXY(8, 0).Height(24))
	}
	return ui.Column(children...).Background(ui.RGB(outer))
}
func (m *model) codeView(visible int) *ui.Element {
	d := m.current()
	if d == nil {
		return ui.Column(spacer(), label("gocode").FontSize(44).Foreground(ui.RGB(0x555555)), label("Open a file in Explorer to start editing."), spacer()).Padding(40)
	}
	if d.large != nil {
		return m.largeCodeView(d, visible)
	}
	if m.editing && d.line >= d.scroll+visible {
		d.scroll = max(0, d.line-visible+1)
	}
	rows := []*ui.Element{}
	selection := d.buffer.Selection().Range()
	for i := d.scroll; i < d.buffer.LineCount() && i < d.scroll+visible; i++ {
		line := d.buffer.Line(i)
		runes := []rune(line)
		if len(runes) > 400 {
			line = string(runes[:400])
		}
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
		if m.editing && i == d.line {
			x, _ := ui.MeasureText(strings.ReplaceAll(string([]rune(line)[:min(d.column, len([]rune(line)))]), "\t", "    "), 14, codeFont())
			layers = append(layers, ui.Row(ui.Column().Width(x), ui.Column().Width(1).Height(20).Background(ui.RGB(0xaeafad)), spacer()))
			if ghost := ghostText(d, m.suggestion); ghost != "" {
				layers = append(layers, ui.Row(ui.Column().Width(x), ui.Text(ghost).FontFamily(codeFont()).FontSize(14).Foreground(ui.RGB(0x777777)), spacer()))
			}
		}
		rows = append(rows, ui.Row(ui.Text(fmt.Sprintf("%4d", i+1)).FontFamily(codeFont()).FontSize(14).Foreground(ui.RGB(0x858585)).Width(52), ui.Column().Width(16), ui.Stack(layers...).Flex(1)).Height(20).Background(bg).Key(fmt.Sprintf("code-line-%d", i)).OnClick(func(*ui.Context) {
			m.moveCursor(d, i, hitColumn(d.buffer.Line(i), max(0, m.pointerX-358)), m.pointerShift)
			m.editing = true
		}))
	}
	rows = append(rows, spacer())
	minimap := []*ui.Element{ui.Column().Height(4)}
	for i := 0; i < min(100, d.buffer.LineCount()); i++ {
		line := d.buffer.Line(i)
		width := float32(min(70, len([]rune(strings.TrimSpace(line)))))
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
	header = append(header, spacer(), icon("chevron-down", "panel-toggle", func(*ui.Context) { m.showPanel = false }), icon("close", "panel-close", func(*ui.Context) { m.showPanel = false }))
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
	} else if m.panel == "TERMINAL" {
		lines = append(lines, label("A shell terminal is not connected in this preview.").Height(20).Foreground(ui.RGB(muted)), label(filepath.Base(m.workspace)+" >").FontFamily(codeFont()).Height(20))
	} else if m.panel == "OUTPUT" {
		for _, line := range m.output[max(0, len(m.output)-5):] {
			lines = append(lines, label(strings.TrimSuffix(line, "\n")).FontFamily(codeFont()).Height(20))
		}
	} else if m.panel == "PROBLEMS" {
		problems := m.problems()
		for i, p := range problems[:min(5, len(problems))] {
			lines = append(lines, button(fmt.Sprintf("%s  %s:%d:%d", p.Message, filepath.Base(p.Path), p.Range.Start.Line+1, p.Range.Start.Character+1), fmt.Sprintf("problem-%d", i), func(*ui.Context) {
				m.open(p.Path)
				if d := m.current(); d != nil && d.buffer != nil {
					column, _ := d.buffer.RuneColumn(p.Range.Start)
					m.moveCursor(d, p.Range.Start.Line, column, false)
					m.editing = true
				}
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
	return ui.Column(rule(), ui.Row(label(" >< ").Width(34).Background(ui.RGB(accent)), label(fmt.Sprintf("  × %d   ⚠ %d", errors, warnings)).Width(180).Key("status-problems").OnClick(func(*ui.Context) { m.panel = "PROBLEMS"; m.showPanel = true }), label(strings.TrimSpace(m.status+"  "+m.lspStatus)).Flex(1), label(position).Width(105), label("Spaces: 4").Width(80), label("UTF-8").Width(55), label(lineEnding).Width(42), label(language).Width(60), icon("terminal", "status-panel", func(*ui.Context) { m.showPanel = !m.showPanel }).Width(28)).Height(21)).Height(22).Background(ui.RGB(outer))
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
