package main

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"unicode/utf8"

	workspaceSearch "github.com/neko233-com/gocode/internal/search"
	ui "github.com/neko233-com/godesktop"
)

func (m *model) showSearch(cx *ui.Context) {
	m.activity = "search"
	m.palette, m.navigation = false, false
	m.search.focus = 0
	m.search.caret = utf8.RuneCountInString(m.search.query.Text)
	m.terminalFocused, m.chatFocused, m.updateFocused, m.editing = false, false, false, false
	if !m.search.initialized {
		m.search.initialized = true
		m.search.query.UseIgnore = true
		m.search.selected = -1
	}
	if cx != nil {
		cx.Invalidate()
	}
}
func (m *model) searchField(index int) *string {
	switch index {
	case 1:
		return &m.search.query.Include
	case 2:
		return &m.search.query.Exclude
	case 3:
		return &m.search.replacement
	default:
		return &m.search.query.Text
	}
}
func (m *model) searchInput(cx *ui.Context, e ui.InputEvent) bool {
	if e.Kind == ui.InputCancelled {
		m.search.replacePressed = false
		m.search.replaceCapture = nil
	}
	command := e.Modifiers&(ui.ModifierControl|ui.ModifierCommand) != 0
	if m.keymapProfile() == "vscode" && e.Kind == ui.KeyPressed && command && e.Modifiers&ui.ModifierShift != 0 && e.Key == 'H' {
		m.showSearch(cx)
		m.search.replaceShown, m.search.focus = true, 3
		m.search.caret = utf8.RuneCountInString(m.search.replacement)
		return true
	}
	if e.Kind == ui.KeyPressed && command && e.Modifiers&ui.ModifierShift != 0 && e.Key == 'F' {
		m.showSearch(cx)
		return true
	}
	if m.activity != "search" || m.palette || m.navigation {
		return false
	}
	inside := func(key string, x, y float32) bool {
		if cx == nil {
			return false
		}
		b, ok := cx.ElementBounds(key)
		return ok && x >= b.X && x < b.X+b.Width && y >= b.Y && y < b.Y+b.Height
	}
	if e.Kind == ui.Scroll && inside("search-results", e.PointerX, e.PointerY) {
		m.search.scroll = max(0, m.search.scroll-e.Y*22)
		return true
	}
	if e.Kind == ui.PointerReleased && m.search.replacePressed && !inside("search-replace-all", e.X, e.Y) {
		m.search.replacePressed = false
		m.search.replaceCapture = nil
	}
	if e.Kind == ui.PointerPressed {
		m.search.replacePressed = inside("search-replace-all", e.X, e.Y)
		m.search.replaceCapture = nil
		if m.search.replacePressed {
			m.search.replaceCapture = m.search.replacePlan
			m.search.replacePressGeneration = m.search.replaceGeneration
		}
		for i, key := range []string{"search-query", "search-include", "search-exclude", "search-replacement"} {
			if inside(key, e.X, e.Y) {
				m.search.focus = i
				m.search.caret = utf8.RuneCountInString(*m.searchField(i))
				m.search.selectAll = false
				m.editing = false
				m.terminalFocused, m.chatFocused = false, false
				return true
			}
		}
		m.search.focus = -2
		m.search.selectAll = false
		if inside("search-results", e.X, e.Y) {
			m.search.focus = -1
		}
		return false
	}
	if e.Kind == ui.KeyPressed && e.Key == 115 && len(m.search.report.Matches) > 0 {
		step := 1
		if e.Modifiers&ui.ModifierShift != 0 {
			step = -1
		}
		index := (max(0, m.search.selected) + step + len(m.search.report.Matches)) % len(m.search.report.Matches)
		if m.search.selected < 0 {
			index = 0
			if step < 0 {
				index = len(m.search.report.Matches) - 1
			}
		}
		m.revealSearchResult(cx, index)
		m.activateSearchResult(cx, index)
		return true
	}
	if m.search.focus == -2 {
		return false
	}
	if e.Kind == ui.KeyPressed && (e.Key == 38 || e.Key == 40) && len(m.search.report.Matches) > 0 {
		step := 1
		if e.Key == 38 {
			step = -1
		}
		index := m.search.selected + step
		if m.search.selected < 0 {
			index = 0
		}
		index = max(0, min(len(m.search.report.Matches)-1, index))
		m.search.focus = -1
		m.revealSearchResult(cx, index)
		return true
	}
	if m.search.focus == -1 {
		if e.Kind == ui.KeyPressed && e.Key == 13 {
			m.activateSearchResult(cx, m.search.selected)
			return true
		}
		if e.Kind == ui.KeyPressed && e.Key == 27 {
			m.search.focus = -2
			m.editing = true
			return true
		}
		return false
	}
	field := m.searchField(m.search.focus)
	fieldChanged := func() {
		if m.search.focus == 3 {
			m.replacementChanged()
		} else {
			m.searchChanged(cx)
		}
	}
	runes := []rune(*field)
	m.search.caret = max(0, min(len(runes), m.search.caret))
	change := func(text string) {
		if m.search.selectAll {
			runes = nil
			m.search.caret = 0
			m.search.selectAll = false
		}
		added := []rune(text)
		value := string(runes[:m.search.caret]) + text + string(runes[m.search.caret:])
		if len(value) > workspaceSearch.MaxQueryBytes {
			return
		}
		*field = value
		m.search.caret += len(added)
		fieldChanged()
	}
	if e.Kind == ui.Character && e.Key >= 32 && !command {
		change(string(rune(e.Key)))
		return true
	}
	if e.Kind != ui.KeyPressed {
		return false
	}
	if command {
		switch e.Key {
		case 'A':
			m.search.selectAll = true
			return true
		case 'V':
			if m.readClipboard != nil {
				value, err := m.readClipboard()
				if err != nil {
					m.message = err.Error()
				} else {
					change(value)
				}
			}
			return true
		case 'C', 'X':
			if m.search.selectAll && m.writeClipboard != nil {
				if err := m.writeClipboard(*field); err != nil {
					m.message = err.Error()
				} else if e.Key == 'X' {
					change("")
				}
			}
			return true
		}
	}
	if e.Modifiers&ui.ModifierAlt != 0 {
		switch e.Key {
		case 'C':
			m.search.query.CaseSensitive = !m.search.query.CaseSensitive
		case 'W':
			m.search.query.WholeWord = !m.search.query.WholeWord
		case 'R':
			m.search.query.Regex = !m.search.query.Regex
		default:
			return false
		}
		m.searchChanged(cx)
		return true
	}
	switch e.Key {
	case 13:
		if m.search.focus == 3 {
			m.previewReplacement(cx)
			return true
		}
		if m.search.submit != nil {
			m.search.submit()
		}
		return true
	case 27:
		m.stopSearch()
		m.search.focus = -2
		m.editing = true
		return true
	case 9:
		order := []int{0, 1, 2}
		if m.search.replaceShown {
			order = []int{0, 3, 1, 2}
		}
		index := 0
		for i, value := range order {
			if value == m.search.focus {
				index = i
				break
			}
		}
		step := 1
		if e.Modifiers&ui.ModifierShift != 0 {
			step = -1
		}
		m.search.focus = order[(index+step+len(order))%len(order)]
		m.search.caret = utf8.RuneCountInString(*m.searchField(m.search.focus))
		m.search.selectAll = false
		return true
	case 37:
		m.search.caret = max(0, m.search.caret-1)
		m.search.selectAll = false
		return true
	case 39:
		m.search.caret = min(len(runes), m.search.caret+1)
		m.search.selectAll = false
		return true
	case 36:
		m.search.caret = 0
		m.search.selectAll = false
		return true
	case 35:
		m.search.caret = len(runes)
		m.search.selectAll = false
		return true
	case 8, 46:
		if m.search.selectAll {
			change("")
			return true
		}
		index := m.search.caret
		if e.Key == 8 {
			index--
		}
		if index >= 0 && index < len(runes) {
			*field = string(runes[:index]) + string(runes[index+1:])
			if e.Key == 8 {
				m.search.caret--
			}
			fieldChanged()
		}
		return true
	}
	return false
}

func (m *model) revealSearchResult(cx *ui.Context, index int) {
	m.search.selected = index
	if cx == nil || index < 0 || index >= len(m.search.resultRows) {
		return
	}
	b, ok := cx.ElementBounds("search-results")
	if !ok {
		return
	}
	start := float32(m.search.resultRows[index] * 22)
	if start < m.search.scroll {
		m.search.scroll = start
	}
	if start+22 > m.search.scroll+b.Height {
		m.search.scroll = start + 22 - b.Height
	}
}

func (m *model) searchSidebar(cx *ui.Context) *ui.Element {
	field := func(index int, key, placeholder string) *ui.Element {
		value := *m.searchField(index)
		text := value
		color := uint32(foreground)
		if value == "" {
			text = placeholder
			color = muted
		}
		if m.search.focus == index {
			r := []rune(value)
			caret := max(0, min(len(r), m.search.caret))
			text = string(r[:caret]) + "▏" + string(r[caret:])
			color = foreground
		}
		bg := uint32(0x313131)
		if m.search.focus == index && m.search.selectAll {
			bg = 0x264f78
		}
		offset := float32(0)
		if m.search.focus == index && value != "" {
			r := []rune(value)
			caret := max(0, min(len(r), m.search.caret))
			w, _ := ui.MeasureText(string(r[:caret]), 13, "")
			width := float32(144)
			if index != 0 {
				width = 216
			}
			offset = max(0, w-width+20)
		}
		return ui.Viewport(label(strings.ReplaceAll(text, "\n", "↵")).Foreground(ui.RGB(color)).PaddingXY(6, 0)).ScrollOffset(offset, 0).Height(26).Background(ui.RGB(bg)).Key(key).OnClick(func(*ui.Context) { m.search.focus = index; m.search.caret = utf8.RuneCountInString(value) })
	}
	toggle := func(text, key string, on bool, change func()) *ui.Element {
		bg := uint32(0x252526)
		if on {
			bg = 0x264f78
		}
		return label(text).FontSize(12).Width(24).Height(26).Background(ui.RGB(bg)).Key(key).OnClick(func(c *ui.Context) { change(); m.searchChanged(c) })
	}
	chevron := "chevron-right"
	if m.search.replaceShown {
		chevron = "chevron-down"
	}
	showReplace := ui.Icon(chevron).Width(16).Height(26).Key("search-replace-toggle").OnClick(func(*ui.Context) {
		m.search.replaceShown = !m.search.replaceShown
		m.search.focus = 0
		m.search.caret = utf8.RuneCountInString(m.search.query.Text)
	})
	query := ui.Row(showReplace, field(0, "search-query", "Search").Flex(1), toggle("Aa", "search-case", m.search.query.CaseSensitive, func() { m.search.query.CaseSensitive = !m.search.query.CaseSensitive }), toggle("ab", "search-word", m.search.query.WholeWord, func() { m.search.query.WholeWord = !m.search.query.WholeWord }), toggle(".*", "search-regex", m.search.query.Regex, func() { m.search.query.Regex = !m.search.query.Regex })).Height(26)
	replaceControls := ui.Column().Height(0)
	if m.search.replaceShown {
		replaceControls = ui.Column(field(3, "search-replacement", "Replace"), ui.Row(button("Preview", "search-replace-preview", func(c *ui.Context) { m.previewReplacement(c) }).Flex(1), button("Replace all", "search-replace-all", func(c *ui.Context) {
			m.replacementButton(c)
		}).Flex(1)).Height(24)).Gap(3).Height(53)
	}
	ignoreLabel := "✓ Use ignore files"
	if !m.search.query.UseIgnore {
		ignoreLabel = "Use ignore files"
	}
	status := m.search.status
	if m.search.replaceShown && m.search.replaceStatus != "" {
		status = m.search.replaceStatus
	}
	if status == "" {
		status = "Search workspace contents"
	}
	controls := ui.Column(query, replaceControls, ui.Row(button("Search", "search-run", func(*ui.Context) {
		if m.search.submit != nil {
			m.search.submit()
		}
	}).Flex(1), button("Cancel", "search-cancel", func(*ui.Context) { m.stopSearch() })).Height(24), label("files to include").FontSize(11).Foreground(ui.RGB(muted)).Height(16), field(1, "search-include", "e.g. **/*.{go,md}"), label("files to exclude").FontSize(11).Foreground(ui.RGB(muted)).Height(16), field(2, "search-exclude", "e.g. **/generated/**"), button(ignoreLabel, "search-ignore", func(c *ui.Context) { m.search.query.UseIgnore = !m.search.query.UseIgnore; m.searchChanged(c) }).Height(24), label(status).FontSize(11).Foreground(ui.RGB(muted)).Height(34)).Gap(3).PaddingXY(12, 6)
	_, height := cx.WindowSize()
	visible := max(22, height-318)
	if m.search.replaceShown {
		visible = max(22, visible-56)
	}
	onResult := func(c *ui.Context, index int) {
		if m.search.replacePlan != nil {
			m.search.selected = index
			m.search.focus = -1
		} else {
			m.activateSearchResult(c, index)
		}
	}
	maxScroll := max(0, float32(len(m.search.rows)*22)-visible)
	m.search.scroll = min(maxScroll, max(0, m.search.scroll))
	first := max(0, int(m.search.scroll/22))
	last := min(len(m.search.rows), first+int(math.Ceil(float64(visible/22)))+2)
	rows := []*ui.Element{ui.Column().Height(float32(first * 22))}
	for i := first; i < last; i++ {
		row := m.search.rows[i]
		match := m.search.report.Matches[row.result]
		index := row.result
		if row.file {
			rows = append(rows, ui.Row(ui.Icon("chevron-down").Width(16).Height(22), label(filepath.Base(match.Path)).Flex(1)).PaddingXY(12, 0).Height(22).Key(fmt.Sprintf("search-file-%d-%s", m.search.generation, match.Path)).OnClick(func(c *ui.Context) { onResult(c, index) }))
			continue
		}
		bg := uint32(0x181818)
		if m.search.selected == index {
			bg = 0x37373d
		}
		before := match.Before
		if r := []rune(before); len(r) > 8 {
			before = "…" + string(r[len(r)-8:])
		}
		content := ui.Row(label(fmt.Sprintf("%d  ", match.Range.Start.Line+1)).Foreground(ui.RGB(muted)), label(before), label(match.Text).Background(ui.RGB(0x613214)), label(match.After))
		if plan := m.search.replacePlan; plan != nil && index < len(plan.Values) {
			value := strings.ReplaceAll(strings.ReplaceAll(plan.Values[index], "\n", "↵"), "\t", "⇥")
			runes := []rune(value)
			if len(runes) > 128 {
				value = string(runes[:128]) + "…"
			}
			content = ui.Row(label(fmt.Sprintf("%d  ", match.Range.Start.Line+1)).Foreground(ui.RGB(muted)), label(match.Text).Background(ui.RGB(0x522222)), label(" → ").Foreground(ui.RGB(muted)), label(value).Background(ui.RGB(0x244b2a)))
		}
		rows = append(rows, ui.Viewport(content).PaddingXY(24, 0).Height(22).Background(ui.RGB(bg)).Key(fmt.Sprintf("search-result-%d-%d", m.search.generation, index)).OnClick(func(c *ui.Context) { onResult(c, index) }))
	}
	rows = append(rows, ui.Column().Height(float32((len(m.search.rows)-last)*22)))
	return ui.Column(controls, ui.Viewport(ui.Column(rows...).Height(float32(len(m.search.rows)*22))).ScrollOffset(0, m.search.scroll).Key("search-results").Flex(1)).Flex(1)
}
