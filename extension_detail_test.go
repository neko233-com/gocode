package main

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	ui "github.com/neko233-com/godesktop"
	"github.com/rivo/uniseg"
)

func extensionTestWidth(text string, size float32) float32 {
	width := float32(0)
	graphemes := uniseg.NewGraphemes(text)
	for graphemes.Next() {
		cluster := graphemes.Str()
		if cluster == " " {
			width += size * .3
		} else {
			width += size * .6
		}
	}
	return width
}

func TestExtensionDetailWrapPreservesPrefixUnicodeAndMeasuredWidth(t *testing.T) {
	text := "prefix e\u0301 👩‍👩‍👧‍👦 中文 content without losing the final word"
	for _, width := range []float32{30, 90, 240} {
		lines, truncated := wrapExtensionText(text, width, 13, 100, extensionTestWidth)
		if truncated || strings.Join(lines, "") != text {
			t.Fatalf("width %v loses metadata: %q, truncated=%v", width, lines, truncated)
		}
		for _, line := range lines {
			if !utf8.ValidString(line) || extensionTestWidth(line, 13) > width {
				t.Fatalf("line overflows %v: %q", width, line)
			}
			if strings.HasPrefix(line, "\u0301") || strings.HasPrefix(line, "\u200d") || strings.HasSuffix(line, "\u200d") {
				t.Fatalf("split grapheme: %q", line)
			}
		}
	}
	lines, truncated := wrapExtensionText("first\r\n\r\nlast\rnext", 500, 13, 20, extensionTestWidth)
	if truncated || strings.Join(lines, "|") != "first||last|next" {
		t.Fatalf("paragraphs: %q %v", lines, truncated)
	}
	for _, text := range []string{strings.Repeat("X", extensionDetailTextBytes+50), strings.Repeat("one\n", 600), "\xffprefix" + strings.Repeat("😀", extensionDetailTextBytes)} {
		lines, truncated = wrapExtensionText(text, 90, 13, 5, extensionTestWidth)
		if !truncated || len(lines) > 5 {
			t.Fatalf("unbounded wrap: rows=%d truncated=%v", len(lines), truncated)
		}
		for _, line := range lines {
			if !utf8.ValidString(line) || extensionTestWidth(line, 13) > 90 {
				t.Fatalf("invalid bounded line: %q", line)
			}
		}
	}
	lines, truncated = wrapExtensionText("😀", 1, 13, 10, extensionTestWidth)
	if !truncated || len(lines) != 1 || lines[0] != "" {
		t.Fatalf("unrenderable grapheme should report omission: %q %v", lines, truncated)
	}
	bounded, truncated := boundedExtensionText("\xffprefix"+strings.Repeat("\x00", extensionDetailTextBytes), extensionDetailTextBytes)
	if !truncated || !strings.HasPrefix(bounded, "�prefix") || !utf8.ValidString(bounded) || len(bounded) > extensionDetailTextBytes {
		t.Fatalf("normalization must retain bounded prefix, bytes=%d", len(bounded))
	}
}

func TestExtensionDetailPlansReachLongDescriptionAndLastContribution(t *testing.T) {
	e := extensionInfo{"Long metadata", "publisher.native", "START " + strings.Repeat("description with 中文 and 👩‍👩‍👧‍👦 ", 150) + " END", "1.2.3"}
	wide := makeExtensionDetailPlan(e, "Enabled", "Details", nil, 700, extensionTestWidth)
	narrow := makeExtensionDetailPlan(e, "Enabled", "Details", nil, 280, extensionTestWidth)
	if narrow.height <= wide.height || narrow.height < 480 {
		t.Fatalf("actual viewport width must change wrap/extent: wide=%v narrow=%v", wide.height, narrow.height)
	}
	var description strings.Builder
	for _, row := range narrow.rows[1:] {
		if row.muted {
			break
		}
		description.WriteString(row.text)
	}
	if !strings.HasPrefix(description.String(), "START ") || !strings.HasSuffix(description.String(), " END") {
		t.Fatal("description displays a prefix-truncated chat tail instead of complete bounded metadata")
	}
	commands := make([]extensionCommand, 2000)
	for i := range commands {
		commands[i] = extensionCommand{fmt.Sprintf("native.command.%04d", i), "Native command"}
	}
	plan := makeExtensionDetailPlan(e, "Enabled", "Feature Contributions", commands, 540, extensionTestWidth)
	state := extensionDetailState{scroll: plan.height}
	state.setExtent(540, 160, plan.height)
	start, end, before, after := plan.visibleRows(state.scroll, 160)
	if start == 0 || end != len(plan.rows) || plan.rows[end-1].command != 1999 {
		t.Fatalf("last contribution unreachable: %d..%d/%d", start, end, len(plan.rows))
	}
	if end-start > extensionDetailVisibleRows || before <= 0 || after != extensionDetailPadding {
		t.Fatalf("bad bounded viewport spacers: %d, %v, %v", end-start, before, after)
	}
	sum := before + after
	for _, row := range plan.rows[start:end] {
		sum += row.height
	}
	if sum != plan.height {
		t.Fatalf("virtualization changed scroll extent %v -> %v", plan.height, sum)
	}
	key := plan.rows[end-1].key
	state.scrollBy(-80)
	start, end, _, _ = plan.visibleRows(state.scroll, 160)
	if plan.rows[end-1].key != key {
		t.Fatal("scroll changed final command hit identity")
	}
	seen := map[string]bool{}
	duplicates := makeExtensionDetailPlan(e, "Enabled", "Feature Contributions", []extensionCommand{{"duplicate", "a"}, {"duplicate", "b"}}, 540, extensionTestWidth)
	for _, row := range duplicates.rows {
		if row.command >= 0 {
			if seen[row.key] {
				t.Fatal("duplicate manifest IDs alias native hit targets")
			}
			seen[row.key] = true
		}
	}
	commands = append(commands, make([]extensionCommand, extensionDetailCommands)...)
	bounded := makeExtensionDetailPlan(e, "Enabled", "Feature Contributions", commands, 540, extensionTestWidth)
	if len(bounded.rows) != extensionDetailCommands+2 || !strings.Contains(bounded.rows[len(bounded.rows)-1].text, "Command Palette") {
		t.Fatal("command plan is not explicitly bounded")
	}
}

func TestExtensionDetailRouteIsIndependentAndConfinedToNativeViewport(t *testing.T) {
	m := &model{activity: "files", editing: true}
	m.focusExtensionDetails("native.fixture", "Details")
	s := &m.extensionsView.detailUI
	s.setExtent(500, 200, 2000)
	regions := map[string]ui.Bounds{
		"extension-detail-editor":   {X: 300, Y: 40, Width: 500, Height: 500},
		"extension-detail-viewport": {X: 300, Y: 260, Width: 500, Height: 200},
	}
	lookup := func(key string) (ui.Bounds, bool) { b, ok := regions[key]; return b, ok }
	for _, point := range []ui.Bounds{{X: 250, Y: 300}, {X: 400, Y: 100}, {X: 800, Y: 300}, {X: 400, Y: 460}} {
		if m.extensionDetailRoute(ui.InputEvent{Kind: ui.Scroll, Y: -3, PointerX: point.X, PointerY: point.Y}, lookup) || s.scroll != 0 {
			t.Fatalf("wheel escaped detail content at %v", point)
		}
	}
	if !m.extensionDetailRoute(ui.InputEvent{Kind: ui.Scroll, Y: -3, PointerX: 400, PointerY: 300}, lookup) || s.scroll != 72 {
		t.Fatal("detail wheel stopped when another activity was selected")
	}
	if m.extensionsView.scroll != 0 {
		t.Fatal("detail wheel mutated extension sidebar")
	}
	s.focused = false
	if m.extensionDetailRoute(ui.InputEvent{Kind: ui.KeyPressed, Key: 34}, lookup) || s.scroll != 72 {
		t.Fatal("unfocused detail stole keyboard scrolling")
	}
	m.extensionDetailRoute(ui.InputEvent{Kind: ui.PointerPressed, X: 400, Y: 300}, lookup)
	if !s.focused || m.editing || m.extensionsView.focused {
		t.Fatal("detail pointer did not move logical focus")
	}
	if !m.extensionDetailRoute(ui.InputEvent{Kind: ui.KeyPressed, Key: 35}, lookup) || s.scroll != 1800 {
		t.Fatalf("End does not reach actual content: %v", s.scroll)
	}
	regions["extension-detail-viewport"] = ui.Bounds{X: 300, Y: 260, Width: 600, Height: 1900}
	if !m.extensionDetailRoute(ui.InputEvent{Kind: ui.Scroll, Y: 0, PointerX: 400, PointerY: 300}, lookup) || s.scroll != 100 {
		t.Fatalf("native resize did not clamp old offset: %v", s.scroll)
	}
	for _, modifiers := range []int{ui.ModifierControl, ui.ModifierCommand, ui.ModifierAlt} {
		if m.extensionDetailRoute(ui.InputEvent{Kind: ui.KeyPressed, Key: 35, Modifiers: modifiers}, lookup) {
			t.Fatal("detail intercepted a workbench/keymap chord")
		}
	}
	if m.extensionDetailRoute(ui.InputEvent{Kind: ui.KeyPressed, Key: 13}, lookup) || m.extensionDetailRoute(ui.InputEvent{Kind: ui.KeyPressed, Key: 9}, lookup) {
		t.Fatal("detail intercepted native button activation/focus order")
	}
	m.extensionDetailRoute(ui.InputEvent{Kind: ui.PointerPressed, X: 100, Y: 100}, lookup)
	if s.focused {
		t.Fatal("outside pointer retained detail keyboard focus")
	}
	m.extensionsView.detail = ""
	if m.extensionDetailRoute(ui.InputEvent{Kind: ui.KeyPressed, Key: 35}, lookup) || s.id != "" || s.scroll != 0 {
		t.Fatal("closed editor retains routing state")
	}
}

func TestExtensionDetailFocusResetKeymapsAndCommandActions(t *testing.T) {
	for _, profile := range []string{"vscode", "jetbrains"} {
		t.Run(profile, func(t *testing.T) {
			m := testModel(t)
			m.keyboard.profile = profile
			m.focusExtensionDetails("native.fixture", "Details")
			d := m.current()
			before := d.buffer.Snapshot().Text()
			text(m, "must never type into source")
			if d.buffer.Snapshot().Text() != before || d.buffer.Dirty() {
				t.Fatal("extension detail typed into underlying document")
			}
			m.extensionsView.detailUI.scroll = 80
			m.selectExtensionDetailTab("Feature Contributions")
			if m.extensionsView.detailUI.scroll != 0 || !m.extensionsView.detailUI.focused {
				t.Fatal("tab selection did not reset independent scroll")
			}
			m.extensionsView.detailUI.scroll = 90
			m.focusExtensionDetails("native.fixture", "Feature Contributions")
			if m.extensionsView.detailUI.scroll != 90 {
				t.Fatal("same selected extension/tab unnecessarily reset scroll")
			}
			m.focusExtensionDetails("native.other", "Feature Contributions")
			if m.extensionsView.detailUI.scroll != 0 {
				t.Fatal("new extension retained old scroll")
			}
			closeKey := int('W')
			if profile == "jetbrains" {
				closeKey = 115
			}
			m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: closeKey, Modifiers: ui.ModifierControl})
			if m.extensionsView.detail != "" || m.extensionsView.detailUI.id != "" || m.extensionsView.detailUI.focused {
				t.Fatal("built-in close editor shortcut failed to clear detail state")
			}
			m.focusExtensionDetails("native.fixture", "Details")
			m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'F', Modifiers: ui.ModifierControl | ui.ModifierShift})
			text(m, "query")
			if m.search.query.Text != "query" || m.extensionsView.detailUI.focused {
				t.Fatal("search shortcut retained stale detail focus and swallowed typing")
			}
			m.focusExtensionDetails("native.fixture", "Details")
			m.extensionsView.detailUI.scroll = 70
			m.focusTab(d)
			if m.extensionsView.detail != "" || m.extensionsView.detailUI.id != "" || m.extensionsView.detailUI.scroll != 0 {
				t.Fatal("real document focus retained closed detail scroll")
			}
		})
	}
	m := &model{}
	e := extensionInfo{"Fixture", "Native.Fixture", "Description", "1"}
	var calls []string
	m.extensionsView.manage = func(action, id string) { calls = append(calls, action+":"+id) }
	title, toggle, uninstall := m.extensionDetailActions(e, true)
	if title != "Disable" || toggle == nil || uninstall == nil {
		t.Fatal("installed actions unavailable")
	}
	toggle(nil)
	uninstall(nil)
	m.extensionsView.settings.Disabled = []string{"native.fixture"}
	title, toggle, _ = m.extensionDetailActions(e, true)
	if title != "Enable" {
		t.Fatal(title)
	}
	toggle(nil)
	title, toggle, uninstall = m.extensionDetailActions(e, false)
	if title != "Install" || uninstall != nil {
		t.Fatal("catalog actions wrong")
	}
	toggle(nil)
	if strings.Join(calls, ",") != "disable:Native.Fixture,uninstall:Native.Fixture,enable:Native.Fixture,catalogInstall:Native.Fixture" {
		t.Fatalf("actual action routing changed: %v", calls)
	}
	m.extensionsView.busy = true
	_, toggle, uninstall = m.extensionDetailActions(e, true)
	if toggle != nil || uninstall != nil {
		t.Fatal("busy actions remained clickable")
	}
	m.extensionsView.busy = false
	e.ID = bundledID
	_, _, uninstall = m.extensionDetailActions(e, true)
	if uninstall != nil {
		t.Fatal("bundled extension became uninstallable")
	}
	var executed string
	m.execute = func(id string) { executed = id }
	m.extensionsView.running = map[string]bool{"native.fixture": true}
	run := m.extensionDetailCommandAction("Native.Fixture", "fixture.realCommand")
	if run == nil {
		t.Fatal("running extension command disabled")
	}
	run(nil)
	if executed != "fixture.realCommand" {
		t.Fatal("native contribution click routed the wrong command")
	}
	m.extensionsView.running["native.fixture"] = false
	if m.extensionDetailCommandAction("Native.Fixture", "fixture.realCommand") != nil {
		t.Fatal("stopped extension command remained runnable")
	}
}

func TestExtensionDetailNativeMeasuredWidths(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if nativeExtensionTextWidth("W", 13) <= 0 {
		t.Skip("native text shaping requires an owned Run; no native GUI is started by this headless test")
	}
	text := "Native proportional Wiii 中文 e\u0301 👩‍👩‍👧‍👦 العربية τέλος "
	for _, width := range []float32{70, 180, 420} {
		lines, more := wrapExtensionText(strings.Repeat(text, 3), width, 13, 80, nativeExtensionTextWidth)
		if more {
			t.Fatalf("native wrap unexpectedly truncated at %v", width)
		}
		for _, line := range lines {
			if nativeExtensionTextWidth(line, 13) > width+.01 {
				t.Fatalf("native shaped line overflow %v: %q", width, line)
			}
		}
		fitted := fitExtensionLine(text, width, 14, nativeExtensionTextWidth)
		if nativeExtensionTextWidth(fitted, 14) > width+.01 || !utf8.ValidString(fitted) {
			t.Fatal("native fitted header overflow")
		}
	}
}

func TestExtensionDetailPlanCacheResizeAndNonfiniteBounds(t *testing.T) {
	s := extensionDetailState{}
	s.sync("native.fixture", "Details")
	shapes := 0
	measure := func(text string, size float32) float32 { shapes++; return extensionTestWidth(text, size) }
	e := extensionInfo{"Fixture", "native.fixture", strings.Repeat("description ", 100), "1"}
	s.updatePlan(e, "Enabled", nil, 500, measure)
	first := shapes
	height := s.plan.height
	s.updatePlan(e, "Enabled", nil, 500, measure)
	if shapes != first {
		t.Fatal("unchanged metadata was reshaped every native frame")
	}
	s.updatePlan(e, "Enabled", nil, 200, measure)
	if shapes <= first || s.plan.height <= height {
		t.Fatal("resize retained stale text layout")
	}
	s.scroll = s.plan.height
	s.setExtent(200, 100, s.plan.height)
	s.setExtent(float32(math.Inf(1)), float32(math.NaN()), float32(math.Inf(-1)))
	if s.viewportWidth != 1 || s.viewportHeight != 0 || s.contentHeight != 0 || s.scroll != 0 {
		t.Fatalf("nonfinite geometry leaked: %#v", s)
	}
}

func TestExtensionDetailWrapBoundsNativeShapingWork(t *testing.T) {
	shapedBytes := 0
	measure := func(text string, size float32) float32 { shapedBytes += len(text); return float32(len(text)) }
	lines, more := wrapExtensionText(strings.Repeat("x", extensionDetailTextBytes), 16, 13, extensionDetailTextRows, measure)
	if !more || len(lines) != extensionDetailTextRows {
		t.Fatal("display row bound changed")
	}
	if shapedBytes > extensionDetailTextBytes*8 {
		t.Fatalf("short rows repeatedly shaped the entire metadata: %d bytes", shapedBytes)
	}
}

func TestExtensionDetailImmediateTabNavigationBeforeNextLayout(t *testing.T) {
	m := &model{installed: []extensionInfo{{"Fixture", "native.fixture", "Description", "1"}}}
	commands := make([]extensionCommand, 2000)
	for i := range commands {
		commands[i] = extensionCommand{fmt.Sprintf("fixture.%d", i), "Command"}
	}
	m.extensionsView.contributions = map[string][]extensionCommand{"native.fixture": commands}
	lookup := func(key string) (ui.Bounds, bool) {
		return ui.Bounds{X: 300, Y: 240, Width: 620, Height: 300}, key == "extension-detail-viewport"
	}
	m.focusExtensionDetails("native.fixture", "Details")
	m.selectExtensionDetailTab("Feature Contributions")
	if m.extensionsView.detailUI.contentHeight != 0 {
		t.Fatal("regression must start before any new detail layout")
	}
	if !m.extensionDetailRoute(ui.InputEvent{Kind: ui.KeyPressed, Key: 35}, lookup) || m.extensionsView.detailUI.scroll <= 1000 {
		t.Fatal("End immediately after native tab selection was lost before Draw")
	}
	start, end, _, _ := m.extensionsView.detailUI.plan.visibleRows(m.extensionsView.detailUI.scroll, 300)
	if start == 0 || m.extensionsView.detailUI.plan.rows[end-1].command != 1999 {
		t.Fatal("immediate navigation cannot reach final contribution")
	}
	m.selectExtensionDetailTab("Details")
	m.selectExtensionDetailTab("Feature Contributions")
	if !m.extensionDetailRoute(ui.InputEvent{Kind: ui.Scroll, PointerX: 400, PointerY: 300, Y: -3}, lookup) || m.extensionsView.detailUI.scroll != 72 {
		t.Fatal("wheel immediately after tab selection was lost before Draw")
	}
}
