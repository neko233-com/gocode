package main

import (
	"fmt"
	ui "github.com/neko233-com/godesktop"
)

func (m *model) editorGroupsView(cx *ui.Context, width, height float32) *ui.Element {
	m.captureActiveView()
	var render func(*editorSplit, float32, float32) *ui.Element
	render = func(n *editorSplit, w, h float32) *ui.Element {
		if n.group != nil {
			g := n.group
			g.visibleRows = max(1, int((h-59)/20))
			d := g.current
			parts := []*ui.Element{m.tabsViewFor(g, g.docs, g.tabs, d, w), ui.Row(label(m.groupBreadcrumb(d)).PaddingXY(10, 0), spacer()).Height(24)}
			key := func(k string) string { return groupKey(g, k) }
			var copyDocument *document
			if d != nil {
				copy := *d
				v := vOrCreate(g, d)
				copy.line, copy.column, copy.scroll = v.line, v.column, v.scroll
				copy.holdScroll = v.holdScroll
				copy.large = v.large
				copyDocument = &copy
			}
			selection := vSelection(g, d)
			focus := func() bool {
				if m.findGroup(g.id) != g || g.current != d || !m.ownsDocument(d) {
					return false
				}
				if m.groups.active != g.id {
					m.focusGroup(g.id)
				}
				return true
			}
			code := m.codeViewFor(copyDocument, g.visibleRows, w, selection, m.editing && m.groups.active == g.id, key, func(line int) {
				if !focus() {
					return
				}
				x := m.pointerX
				if b, ok := cx.ElementBounds(key(fmt.Sprintf("code-line-%d", line))); ok {
					x -= b.X + 68
				}
				m.moveCursor(d, line, hitColumn(d.buffer.Line(line), max(0, x)), m.pointerShift)
				m.editing = true
			}, func(line, offset int64, bytes bool) {
				if focus() {
					m.navigateLarge(d, line, offset, bytes)
					m.captureActiveView()
				}
			}, func() {
				if focus() {
					m.navigation = true
					m.query = ""
				}
			})
			if copyDocument != nil {
				g.views[d].scroll = copyDocument.scroll
				if m.groups.active == g.id {
					d.scroll = copyDocument.scroll
				}
			}
			parts = append(parts, ui.Viewport(code).Flex(1).Key(key("editor-content")))
			return ui.Column(parts...).Flex(1).Key(key("editor-group")).Background(ui.RGB(editor))
		}
		color := uint32(border)
		if m.groups.drag == n {
			color = accent
		}
		sash := ui.Column().Background(ui.RGB(color)).Key(fmt.Sprintf("editor-sash-%d", n.id)).OnClick(func(*ui.Context) {})
		if n.down {
			usable := max(0, h-4)
			first := usable * n.ratio
			return ui.Column(render(n.left, w, first).Flex(n.ratio), sash.Height(4), render(n.right, w, usable-first).Flex(1-n.ratio)).Flex(1).Key(fmt.Sprintf("editor-split-%d", n.id))
		}
		usable := max(0, w-4)
		first := usable * n.ratio
		return ui.Row(render(n.left, first, h).Flex(n.ratio), sash.Width(4), render(n.right, usable-first, h).Flex(1-n.ratio)).Flex(1).Key(fmt.Sprintf("editor-split-%d", n.id))
	}
	return render(m.groups.root, width, height)
}
