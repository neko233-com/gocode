package main

import (
	"fmt"
	ui "github.com/neko233-com/godesktop"
)

func (m *model) historyOverlay(base *ui.Element) *ui.Element {
	p := m.history.prompt
	if p == nil {
		return base
	}
	// Captured prompt identity prevents a held click from accepting a new group.
	choose := func(all bool) func(*ui.Context) {
		return func(*ui.Context) {
			if m.history.prompt == p {
				m.confirmHistory(all)
			}
		}
	}
	popup := ui.Column(
		label("Undo changes across all files?").FontSize(17).Height(30),
		label(p.group.label).Height(24),
		ui.Row(
			button(fmt.Sprintf("Undo in %d Files", len(p.group.members)), "history-all", choose(true)).Width(145).Height(30).Background(ui.RGB(accent)),
			button("Undo this File", "history-one", choose(false)).Width(130).Height(30),
			button("Cancel", "history-cancel", func(*ui.Context) {
				if m.history.prompt == p {
					m.history.prompt = nil
				}
			}).Width(80).Height(30),
		).Gap(8).Height(40),
	).Padding(22).Width(440).Background(ui.RGB(0x252526)).Key("history-dialog")
	shade := ui.Column().Background(ui.RGBA(0, .65)).Key("history-backdrop").OnClick(func(*ui.Context) {})
	center := ui.Column(spacer(), ui.Row(spacer(), popup, spacer()).Height(138), spacer())
	return ui.Stack(base, shade, center)
}
