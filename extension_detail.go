package main

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	ui "github.com/neko233-com/godesktop"
	"github.com/rivo/uniseg"
)

const (
	extensionDetailTextBytes     = 64 << 10
	extensionDetailTextRows      = 512
	extensionDetailCommands      = 4096
	extensionDetailPadding       = float32(20)
	extensionDetailLine          = float32(24)
	extensionDetailCommandHeight = float32(66)
	extensionDetailVisibleRows   = 1024 // covers very tall native viewports with a hard tree budget
)

// The extension editor owns its viewport and keyboard focus independently of
// the extension search/list sidebar. All fields belong to the native UI thread.
type extensionDetailState struct {
	id, tab                       string
	scroll, contentHeight         float32
	viewportWidth, viewportHeight float32
	focused                       bool
	plan                          extensionDetailPlan
	planInfo                      extensionInfo
	planStatus                    string
	planCommands                  []extensionCommand
	planTotalCommands             int
}

type extensionDetailRow struct {
	text         string
	key          string
	size, height float32
	muted        bool
	command      int // -1 for noninteractive text, otherwise the manifest command index
}

type extensionDetailPlan struct {
	rows   []extensionDetailRow
	height float32
}

type extensionTextMeasure func(string, float32) float32

func nativeExtensionTextWidth(text string, size float32) float32 {
	w, _ := ui.MeasureText(text, size, "")
	return w
}

func (s *extensionDetailState) reset() { *s = extensionDetailState{} }

func (s *extensionDetailState) sync(id, tab string) {
	if s.id != id || s.tab != tab {
		s.reset()
		s.id, s.tab = id, tab
	}
}

func (s *extensionDetailState) setExtent(width, height, content float32) {
	s.viewportWidth = max(1, finiteDetailDimension(width))
	s.viewportHeight = max(0, finiteDetailDimension(height))
	s.contentHeight = max(0, finiteDetailDimension(content))
	s.scroll = max(0, min(s.scroll, s.maxScroll()))
}

func finiteDetailDimension(value float32) float32 {
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		return 0
	}
	return min(value, 1e6)
}

func (s *extensionDetailState) maxScroll() float32 { return max(0, s.contentHeight-s.viewportHeight) }

func (s *extensionDetailState) scrollBy(delta float32) {
	s.scroll = max(0, min(s.scroll+finiteDetailDimension(delta), s.maxScroll()))
}

func (m *model) focusExtensionDetails(id, tab string) {
	if tab == "" {
		tab = "Details"
	}
	m.extensionsView.detail, m.extensionsView.tab = id, tab
	m.extensionsView.detailUI.sync(id, tab)
	m.extensionsView.detailUI.focused = true
	m.extensionsView.focused = false
	m.editing, m.terminalFocused, m.chatFocused = false, false, false
}

func (m *model) selectExtensionDetailTab(tab string) {
	if tab == "Details" || tab == "Feature Contributions" {
		m.focusExtensionDetails(m.extensionsView.detail, tab)
	}
}

func (m *model) selectedExtensionDetail() (extensionInfo, bool, bool) {
	for _, e := range m.installed {
		if strings.EqualFold(e.ID, m.extensionsView.detail) {
			return e, true, true
		}
	}
	for _, result := range m.extensionsView.catalog.results {
		if strings.EqualFold(result.id(), m.extensionsView.detail) {
			return result.info(), false, true
		}
	}
	return extensionInfo{}, false, false
}

func (m *model) extensionDetailStatus(e extensionInfo, installed bool) string {
	state := "Enabled"
	if !installed {
		state = "Available in " + m.extensionStoreName()
	}
	if containsExtension(m.extensionsView.settings.Disabled, e.ID) {
		state = "Disabled"
	}
	if containsExtension(m.extensionsView.settings.Uninstall, e.ID) {
		state = "Uninstalled · reload required"
	}
	return state
}

func (m *model) extensionDetailActions(e extensionInfo, installed bool) (string, func(*ui.Context), func(*ui.Context)) {
	action, title := "disable", "Disable"
	if containsExtension(m.extensionsView.settings.Disabled, e.ID) {
		action, title = "enable", "Enable"
	}
	if !installed {
		action, title = "catalogInstall", "Install"
	}
	if m.extensionsView.busy || m.extensionsView.manage == nil {
		return title, nil, nil
	}
	toggle := func(*ui.Context) { m.extensionsView.manage(action, e.ID) }
	var uninstall func(*ui.Context)
	if installed && !strings.EqualFold(e.ID, bundledID) {
		uninstall = func(*ui.Context) { m.extensionsView.manage("uninstall", e.ID) }
	}
	return title, toggle, uninstall
}

// Prefix limiting happens before segmentation or shaping, including malformed
// input. Unlike the chat history helper, extension metadata retains its start.
func boundedExtensionText(text string, limit int) (string, bool) {
	truncated := len(text) > limit
	if truncated {
		text = text[:limit]
	}
	text = strings.ReplaceAll(strings.ToValidUTF8(text, "�"), "\x00", "�")
	if len(text) > limit {
		truncated = true
		text = text[:limit]
		for len(text) > 0 && !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
	}
	return text, truncated
}

func extensionGraphemes(text string) []string {
	iterator := uniseg.NewGraphemes(text)
	var clusters []string
	for iterator.Next() {
		clusters = append(clusters, iterator.Str())
	}
	return clusters
}

func fitExtensionLine(text string, width, size float32, measure extensionTextMeasure) string {
	text, _ = boundedExtensionText(text, extensionDetailTextBytes)
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r", " "), "\n", " ")
	if width <= 0 {
		return ""
	}
	if measure(text, size) <= width {
		return text
	}
	if measure("…", size) > width {
		return ""
	}
	clusters := extensionGraphemes(text)
	lo, hi := 0, len(clusters)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if measure(strings.Join(clusters[:mid], "")+"…", size) <= width {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return strings.Join(clusters[:lo], "") + "…"
}

// Find a fitting prefix with exponential probes first. Shaping the entire
// remaining 64-KiB paragraph for each short row would magnify native font work.
func fittingExtensionClusters(clusters []string, width, size float32, measure extensionTextMeasure) int {
	lo, hi := 0, min(1, len(clusters))
	for hi > 0 && measure(strings.Join(clusters[:hi], ""), size) <= width {
		lo = hi
		if hi == len(clusters) {
			return hi
		}
		hi = min(len(clusters), hi*2)
	}
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if measure(strings.Join(clusters[:mid], ""), size) <= width {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// Wrap at measured native width, retaining whole Unicode grapheme clusters.
// maxRows and the byte bound also cap shaping work for untrusted metadata.
func wrapExtensionText(text string, width, size float32, maxRows int, measure extensionTextMeasure) ([]string, bool) {
	text, truncated := boundedExtensionText(text, extensionDetailTextBytes)
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	maxRows = max(1, min(maxRows, extensionDetailTextRows))
	var lines []string
	paragraphs := strings.Split(text, "\n")
	for pi, paragraph := range paragraphs {
		clusters := extensionGraphemes(paragraph)
		if len(clusters) == 0 {
			lines = append(lines, "")
		}
		for len(clusters) > 0 {
			if len(lines) >= maxRows {
				return lines, true
			}
			lo := fittingExtensionClusters(clusters, width, size, measure)
			if lo == 0 {
				// A single glyph wider than the viewport cannot be split or drawn
				// outside the content. Report omission rather than overflow.
				return append(lines, fitExtensionLine(paragraph, width, size, measure)), true
			}
			end := lo
			if lo < len(clusters) {
				for i := lo - 1; i > 0; i-- {
					if strings.IndexFunc(clusters[i], unicode.IsSpace) >= 0 {
						end = i + 1
						break
					}
				}
			}
			lines = append(lines, strings.Join(clusters[:end], ""))
			clusters = clusters[end:]
		}
		if len(lines) >= maxRows && pi < len(paragraphs)-1 {
			return lines, true
		}
	}
	return lines, truncated
}

func makeExtensionDetailPlan(e extensionInfo, state, tab string, commands []extensionCommand, width float32, measure extensionTextMeasure) extensionDetailPlan {
	p := extensionDetailPlan{height: 2 * extensionDetailPadding}
	add := func(text string, size, height float32, muted bool) {
		p.rows = append(p.rows, extensionDetailRow{text: text, size: size, height: height, muted: muted, command: -1})
		p.height += height
	}
	textWidth := max(1, width-2*extensionDetailPadding-12) // reserve the scroll indicator
	if tab == "Feature Contributions" {
		add("Commands", 20, 36, false)
		seen := make(map[string]int)
		for i, command := range commands[:min(len(commands), extensionDetailCommands)] {
			key := "extension-detail-command-" + command.ID
			if seen[command.ID] > 0 {
				key += fmt.Sprintf("-duplicate-%d", seen[command.ID])
			}
			seen[command.ID]++
			p.rows = append(p.rows, extensionDetailRow{key: key, height: extensionDetailCommandHeight, command: i})
			p.height += extensionDetailCommandHeight
		}
		if len(commands) == 0 {
			add("This extension contributes no commands.", 13, 24, true)
		}
		if len(commands) > extensionDetailCommands {
			add("More commands are available through the Command Palette.", 13, 24, true)
		}
		return p
	}
	add(fitExtensionLine(e.Name, textWidth, 22, measure), 22, 38, false)
	lines, truncated := wrapExtensionText(e.Description, textWidth, 13, extensionDetailTextRows, measure)
	for _, line := range lines {
		add(line, 13, extensionDetailLine, false)
	}
	if truncated {
		add("Description truncated at the display limit.", 11, 24, true)
	}
	for _, entry := range []string{"Identifier: " + e.ID, "Version: " + e.Version, "Status: " + state} {
		lines, more := wrapExtensionText(entry, textWidth, 13, 8, measure)
		for _, line := range lines {
			add(line, 13, 28, true)
		}
		if more {
			add("…", 13, 28, true)
		}
	}
	return p
}

func (s *extensionDetailState) updatePlan(e extensionInfo, status string, commands []extensionCommand, width float32, measure extensionTextMeasure) {
	bounded := commands[:min(len(commands), extensionDetailCommands)]
	if len(s.plan.rows) > 0 && s.planInfo == e && s.planStatus == status && s.viewportWidth == width && s.planTotalCommands == len(commands) && slices.Equal(s.planCommands, bounded) {
		return
	}
	s.plan = makeExtensionDetailPlan(e, status, s.tab, commands, width, measure)
	s.planInfo, s.planStatus, s.planTotalCommands = e, status, len(commands)
	s.planCommands = slices.Clone(bounded)
	s.viewportWidth = width
}

// Return just the intersecting rows and one row of overscan, with exact spacers
// so the native viewport retains the full extent and stable keyed hit targets.
func (p extensionDetailPlan) visibleRows(scroll, height float32) (start, end int, before, after float32) {
	y := extensionDetailPadding
	start = len(p.rows)
	for i, row := range p.rows {
		if y+row.height >= scroll-extensionDetailLine {
			start = i
			break
		}
		y += row.height
	}
	before = y
	end = start
	for end < len(p.rows) && end-start < extensionDetailVisibleRows && y < scroll+height+extensionDetailCommandHeight {
		y += p.rows[end].height
		end++
	}
	return start, end, before, max(0, p.height-y)
}

// Selection/tab callbacks can be followed by a key or wheel message before the
// next Draw. Resolve the new content extent on that same UI turn; stale layout
// must not discard a user's immediate End/PageDown or reset newly moved scroll.
func (m *model) prepareExtensionDetailViewport(bounds ui.Bounds) {
	s := &m.extensionsView.detailUI
	contentHeight := s.contentHeight
	if e, installed, found := m.selectedExtensionDetail(); found {
		s.updatePlan(e, m.extensionDetailStatus(e, installed), m.extensionsView.contributions[e.ID], bounds.Width, nativeExtensionTextWidth)
		contentHeight = s.plan.height
	}
	s.setExtent(bounds.Width, bounds.Height, contentHeight)
}

func (m *model) extensionDetailRoute(e ui.InputEvent, bounds func(string) (ui.Bounds, bool)) bool {
	s := &m.extensionsView.detailUI
	if m.extensionsView.detail == "" {
		s.reset()
		return false
	}
	s.sync(m.extensionsView.detail, m.extensionsView.tab)
	if e.Kind == ui.PointerPressed {
		b, ok := bounds("extension-detail-editor")
		s.focused = ok && insideBounds(b, e.X, e.Y)
		if s.focused {
			m.extensionsView.focused = false
			m.editing, m.terminalFocused, m.chatFocused = false, false, false
		}
		return false // native controls retain their click/keyboard focus behavior
	}
	if e.Kind == ui.Scroll {
		b, ok := bounds("extension-detail-viewport")
		if !ok || !insideBounds(b, e.PointerX, e.PointerY) {
			return false
		}
		m.prepareExtensionDetailViewport(b)
		s.scrollBy(-e.Y * extensionDetailLine)
		return true
	}
	if m.editing || m.terminalFocused || m.chatFocused || m.navigation || m.updateFocused || m.extensionsView.focused || m.activity == "search" && m.search.focus >= 0 || m.activity == "source-control" && m.scm.inputFocused {
		s.focused = false
	}
	if !s.focused || e.Modifiers&(ui.ModifierControl|ui.ModifierCommand|ui.ModifierAlt) != 0 {
		return false
	}
	if e.Kind == ui.Character {
		return true
	} // never type into the underlying document
	if e.Kind != ui.KeyPressed {
		return false
	}
	if b, ok := bounds("extension-detail-viewport"); ok {
		m.prepareExtensionDetailViewport(b)
	}
	switch e.Key {
	case 38:
		s.scrollBy(-extensionDetailLine)
	case 40:
		s.scrollBy(extensionDetailLine)
	case 33:
		s.scrollBy(-max(extensionDetailLine, s.viewportHeight-extensionDetailLine))
	case 34:
		s.scrollBy(max(extensionDetailLine, s.viewportHeight-extensionDetailLine))
	case 36:
		s.scroll = 0
	case 35:
		s.scroll = s.maxScroll()
	case 27:
		m.closeExtensionDetails()
	default:
		return false
	}
	return true
}
