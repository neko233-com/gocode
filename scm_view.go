package main

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"unicode/utf8"

	ui "github.com/neko233-com/godesktop"
)

func (m *model) showSCM() {
	m.activity = "source-control"
	m.palette = false
	m.navigation = false
	m.scm.inputFocused = false
	if m.scm.refresh != nil {
		m.scm.refresh()
	}
}
func (m *model) dismissSCMDiff() {
	m.scm.diff, m.scm.diffRows = nil, nil
	m.scm.inputFocused = false
}
func (m *model) scmInput(cx *ui.Context, e ui.InputEvent) bool {
	command := e.Modifiers&(ui.ModifierControl|ui.ModifierCommand) != 0
	if e.Kind == ui.KeyPressed && command && e.Modifiers&ui.ModifierShift != 0 && e.Key == 'G' {
		m.showSCM()
		return true
	}
	if m.palette || m.navigation {
		return false
	}
	inside := func(key string, x, y float32) bool {
		if cx == nil {
			return false
		}
		b, ok := cx.ElementBounds(key)
		return ok && x >= b.X && x < b.X+b.Width && y >= b.Y && y < b.Y+b.Height
	}
	if e.Kind == ui.Scroll {
		if m.activity == "source-control" && inside("scm-resources", e.PointerX, e.PointerY) {
			m.scm.scroll = max(0, m.scm.scroll-e.Y*24)
			return true
		}
		if m.scm.diff != nil && inside("scm-diff", e.PointerX, e.PointerY) {
			m.scm.diffScroll = max(0, m.scm.diffScroll-e.Y*20)
			return true
		}
	}
	if e.Kind == ui.PointerPressed {
		m.scm.inputFocused = m.activity == "source-control" && inside("scm-message", e.X, e.Y)
		if m.scm.diff != nil && inside("scm-diff", e.X, e.Y) {
			m.editing, m.terminalFocused, m.chatFocused = false, false, false
			return true
		}
	}
	if m.scm.inputFocused && m.activity == "source-control" {
		if e.Kind == ui.Character {
			if !command && e.Key >= 32 && utf8.ValidRune(rune(e.Key)) && len(m.scm.message)+utf8.RuneLen(rune(e.Key)) <= 64<<10 {
				m.scm.message += string(rune(e.Key))
			}
			return true
		}
		if e.Kind == ui.KeyPressed {
			switch e.Key {
			case 8:
				_, size := utf8.DecodeLastRuneInString(m.scm.message)
				if size > 0 {
					m.scm.message = m.scm.message[:len(m.scm.message)-size]
				}
			case 13:
				if command {
					m.scmAction("commit", nil, false)
				} else if len(m.scm.message) < 64<<10 {
					m.scm.message += "\n"
				}
			case 27:
				m.scm.inputFocused = false
			case 'V':
				if command && m.readClipboard != nil {
					value, err := m.readClipboard()
					if err == nil && utf8.ValidString(value) && len(m.scm.message)+len(value) <= 64<<10 {
						m.scm.message += value
					}
				}
			}
			return true
		}
	}
	if m.scm.diff != nil && !m.terminalFocused && !m.chatFocused && e.Kind == ui.Character {
		return true
	}
	if m.scm.diff != nil && !m.terminalFocused && !m.chatFocused && e.Kind == ui.KeyPressed {
		if e.Key == 27 || command && e.Key == 'W' {
			m.dismissSCMDiff()
			m.focusTab(m.current())
			return true
		}
		if !command {
			switch e.Key {
			case 38:
				m.scm.diffScroll = max(0, m.scm.diffScroll-20)
			case 40:
				m.scm.diffScroll += 20
			case 33:
				m.scm.diffScroll = max(0, m.scm.diffScroll-400)
			case 34:
				m.scm.diffScroll += 400
			case 36:
				m.scm.diffScroll = 0
			case 35:
				m.scm.diffScroll = float32(len(m.scm.diffRows) * 20)
			default:
				return false
			}
			return true
		}
	}
	if m.scm.diff != nil && !m.terminalFocused && !m.chatFocused && e.Kind == ui.KeyPressed && command && (e.Key == 'S' || e.Key == 'Z' || e.Key == 'Y') {
		return true
	}
	return false
}

func (m *model) scmSidebar(cx *ui.Context) *ui.Element {
	control := func(text, key string, fn func(*ui.Context), enabled bool) *ui.Element {
		if !enabled || m.scm.busy {
			fn = nil
		}
		return button(text, key, fn).Height(26)
	}
	message := m.scm.message
	if message == "" {
		message = "Message (Ctrl/Cmd+Enter to commit)"
	}
	if m.scm.inputFocused {
		message += "▏"
	}
	status := m.scm.status
	if status == "" {
		status = "Detecting repository…"
	}
	if m.scm.repository == nil && !m.scm.busy {
		return ui.Column(label(status).Padding(12).Height(58), control("Initialize repository", "scm-init", func(*ui.Context) { m.scmAction("init", nil, false) }, strings.Contains(status, "not a Git repository"))).Flex(1)
	}
	staged := 0
	changed := 0
	for _, entry := range m.scm.snapshot.Entries {
		if entry.Staged() {
			staged++
		}
		if entry.Changed() {
			changed++
		}
	}
	controls := ui.Column(ui.Row(label(filepath.Base(m.scm.snapshot.Root)).Flex(1), control("↻", "scm-refresh", func(*ui.Context) {
		if m.scm.refresh != nil {
			m.scm.refresh()
		}
	}, true)).Height(26), ui.Viewport(label(strings.ReplaceAll(message, "\n", "↵")).PaddingXY(6, 0)).Height(50).Background(ui.RGB(0x313131)).Key("scm-message").OnClick(func(*ui.Context) {
		m.scm.inputFocused = true
		m.editing = false
		m.terminalFocused = false
		m.chatFocused = false
	}), control("✓ Commit staged changes", "scm-commit", func(*ui.Context) { m.scmAction("commit", nil, false) }, staged > 0 && strings.TrimSpace(m.scm.message) != ""), label(status).FontSize(11).Foreground(ui.RGB(muted)).Height(32)).PaddingXY(12, 6).Gap(4)
	type row struct {
		title  bool
		path   string
		staged bool
		symbol string
	}
	resources := []row{}
	for _, group := range []struct {
		staged bool
		title  string
		count  int
	}{{true, "STAGED CHANGES", staged}, {false, "CHANGES", changed}} {
		resources = append(resources, row{title: true, path: fmt.Sprintf("%s  %d", group.title, group.count), staged: group.staged})
		for _, entry := range m.scm.snapshot.Entries {
			if group.staged && !entry.Staged() || !group.staged && !entry.Changed() {
				continue
			}
			symbol := string(entry.Worktree)
			if group.staged {
				symbol = string(entry.Index)
			}
			if entry.Conflict {
				symbol = "!"
			}
			resources = append(resources, row{path: entry.Path, staged: group.staged, symbol: symbol})
		}
	}
	_, height := cx.WindowSize()
	visible := max(24, height-250)
	m.scm.scroll = min(max(0, float32(len(resources)*24)-visible), max(0, m.scm.scroll))
	first := int(m.scm.scroll / 24)
	last := min(len(resources), first+int(math.Ceil(float64(visible/24)))+2)
	children := []*ui.Element{ui.Column().Height(float32(first * 24))}
	for index := first; index < last; index++ {
		item := resources[index]
		key := fmt.Sprintf("scm-resource-%t-%s", item.staged, item.path)
		if item.title {
			paths := []string{}
			for _, entry := range m.scm.snapshot.Entries {
				if item.staged && entry.Staged() || !item.staged && entry.Changed() {
					paths = append(paths, entry.Path)
				}
			}
			operation, text := "stage", "+"
			if item.staged {
				operation, text = "unstage", "−"
			}
			children = append(children, ui.Row(label(item.path).FontSize(11).Flex(1), control(text, key, func(*ui.Context) { m.scmAction(operation, paths, false) }, len(paths) > 0)).Height(24).PaddingXY(8, 0))
			continue
		}
		operation, text := "stage", "+"
		if item.staged {
			operation, text = "unstage", "−"
		}
		children = append(children, ui.Row(ui.Viewport(ui.Row(label(filepath.Base(item.path)).Flex(1), label(item.symbol).Foreground(ui.RGB(0xe2c08d)))).Flex(1).Key(key).OnClick(func(*ui.Context) { m.scmAction("diff", []string{item.path}, item.staged) }), control(text, key+"-index", func(*ui.Context) { m.scmAction(operation, []string{item.path}, false) }, item.symbol != "!")).Height(24).PaddingXY(14, 0))
	}
	children = append(children, ui.Column().Height(float32((len(resources)-last)*24)))
	return ui.Column(controls, ui.Viewport(ui.Column(children...).Height(float32(len(resources)*24))).ScrollOffset(0, m.scm.scroll).Key("scm-resources").Flex(1)).Flex(1)
}

func (m *model) scmDiffView(cx *ui.Context, visible int) *ui.Element {
	diff := m.scm.diff
	mode := "Working tree"
	if diff.Staged {
		mode = "Index"
	}
	header := ui.Row(label(filepath.Base(diff.Path)+"  ·  "+mode+" changes").Flex(1), button("Open file", "scm-diff-open", func(*ui.Context) {
		path := filepath.Join(m.scm.snapshot.Root, filepath.FromSlash(diff.Path))
		m.dismissSCMDiff()
		m.open(path)
	}), button("Close", "scm-diff-close", func(*ui.Context) { m.dismissSCMDiff(); m.focusTab(m.current()) })).Height(35).PaddingXY(12, 0)
	if diff.Binary {
		return ui.Column(header, label(fmt.Sprintf("Binary file  ·  %d → %d bytes", diff.OldBytes, diff.NewBytes)).Padding(24), spacer()).Flex(1)
	}
	m.scm.diffScroll = min(max(0, float32(len(m.scm.diffRows)*20-visible*20)), max(0, m.scm.diffScroll))
	first := int(m.scm.diffScroll / 20)
	last := min(len(m.scm.diffRows), first+visible+2)
	rows := []*ui.Element{ui.Column().Height(float32(first * 20))}
	cell := func(text string, number int, changed bool, left bool) *ui.Element {
		bg := ui.Color{}
		if changed {
			if left {
				bg = ui.RGB(0x3e2020)
			} else {
				bg = ui.RGB(0x203a26)
			}
		}
		value := ""
		if number > 0 {
			value = fmt.Sprint(number)
		}
		parts := []*ui.Element{}
		for _, fragment := range highlight(string(codeLinePreview(text, 400))) {
			parts = append(parts, ui.Text(fragment.text).FontFamily(codeFont()).FontSize(14).Foreground(ui.RGB(fragment.color)))
		}
		return ui.Row(label(value).Width(48).Foreground(ui.RGB(muted)), ui.Viewport(ui.Row(parts...)).Flex(1)).Height(20).Background(bg).Flex(1)
	}
	for index := first; index < last; index++ {
		row := m.scm.diffRows[index]
		rows = append(rows, ui.Row(cell(row.left, row.oldLine, row.removed, true), ui.Column().Width(1).Background(ui.RGB(border)), cell(row.right, row.newLine, row.added, false)).Height(20).Key(fmt.Sprintf("scm-diff-row-%d", index)))
	}
	rows = append(rows, ui.Column().Height(float32((len(m.scm.diffRows)-last)*20)))
	return ui.Column(header, ui.Row(label("Original").Flex(1), label(mode).Flex(1)).Height(24).PaddingXY(12, 0), ui.Viewport(ui.Column(rows...).Height(float32(len(m.scm.diffRows)*20))).ScrollOffset(0, m.scm.diffScroll).Key("scm-diff").Flex(1)).Flex(1)
}
