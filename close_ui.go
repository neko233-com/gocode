package main

import (
	"path/filepath"

	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

type closeSave struct {
	document *document
	snapshot textbuffer.Snapshot
	expected *[32]byte
}

func (m *model) requestWindowClose(*ui.Context) bool {
	for _, d := range m.docs {
		if d.dirty() {
			m.beginClose(nil)
			return false
		}
	}
	if m.closeBusy || m.saveBusy || len(m.saveJobs) != 0 {
		m.beginClose(nil)
		m.closeError = "A save is still in progress"
		return false
	}
	return true
}
func (m *model) beginClose(target *document) {
	m.history.prompt = nil
	if m.closeBusy {
		return
	}
	if !m.closePrompt {
		m.closeEditing, m.closeChatFocused, m.closeUpdateFocused = m.editing, m.chatFocused, m.updateFocused
		m.closeTerminalFocused = m.terminalFocused
	}
	m.closePrompt = true
	m.closeTarget = target
	m.closeError = ""
	m.editing = false
	m.chatFocused = false
	m.updateFocused = false
	m.pointerSelecting = false
	m.terminalFocused, m.terminalSelecting, m.panelResizing = false, false, false
}
func (m *model) cancelClose() {
	if m.closeBusy {
		return
	}
	m.closePrompt = false
	m.closeTarget = nil
	m.closeError = ""
	m.editing, m.chatFocused, m.updateFocused = m.closeEditing, m.closeChatFocused, m.closeUpdateFocused
	m.terminalFocused = m.closeTerminalFocused
}
func (m *model) closePlans() []closeSave {
	plans := []closeSave{}
	for _, d := range m.docs {
		if d.buffer != nil && d.dirty() && (m.closeTarget == nil || m.closeTarget == d) {
			var expected *[32]byte
			if d.diskKnown {
				value := d.diskHash
				expected = &value
			}
			plans = append(plans, closeSave{d, d.buffer.Snapshot(), expected})
		}
	}
	return plans
}
func (m *model) finishClose(cx *ui.Context) {
	target := m.closeTarget
	m.closePrompt = false
	m.closeTarget = nil
	if target == nil {
		cx.Quit()
		return
	}
	for index, d := range m.docs {
		if d == target {
			m.removeTab(index)
			return
		}
	}
}
func (m *model) closeOverlay(cx *ui.Context, base *ui.Element) *ui.Element {
	plans := m.closePlans()
	items := []*ui.Element{label("Do you want to save your changes?").FontSize(17).Height(30)}
	for _, plan := range plans[:min(len(plans), 4)] {
		items = append(items, label(filepath.Base(plan.document.path)).Height(22))
	}
	if len(plans) > 4 {
		items = append(items, label("More documents also have unsaved changes").Height(22))
	}
	for _, line := range wrapChatText(m.closeError, 390) {
		items = append(items, label(line).Height(20).FontSize(12))
	}
	var save, discard, cancel func(*ui.Context)
	if !m.closeBusy {
		save = func(*ui.Context) {
			if m.saveForClose != nil {
				m.saveForClose()
			}
		}
		discard = func(*ui.Context) {
			if m.discardForClose != nil {
				m.discardForClose()
			}
		}
		cancel = func(*ui.Context) { m.cancelClose() }
	}
	items = append(items, ui.Row(button("Save", "close-save", save).Width(100).Height(30).Background(ui.RGB(accent)), spacer(), button("Don't Save", "close-discard", discard).Width(120).Height(30), button("Cancel", "close-cancel", cancel).Width(90).Height(30)).Gap(8).Height(40))
	popup := ui.Column(items...).Padding(22).Width(460).Background(ui.RGB(0x252526)).Key("close-dialog")
	shade := ui.Column().Background(ui.RGBA(0, .65)).Key("close-backdrop").OnClick(func(*ui.Context) {})
	height := 114 + min(len(plans), 4)*22 + len(wrapChatText(m.closeError, 390))*20
	if len(plans) > 4 {
		height += 22
	}
	center := ui.Column(spacer(), ui.Row(spacer(), popup, spacer()).Height(float32(height)), spacer())
	return ui.Stack(base, shade, center)
}
