package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/neko233-com/gocode/internal/filewatch"
	ui "github.com/neko233-com/godesktop"
)

type diskConflict struct {
	kind, message string
	hash          [32]byte
}

func (m *model) startDocumentWatch(parent context.Context, dispatch func(func()) bool) func() {
	watcher := filewatch.Start(parent, dispatch, m.applyDiskResult)
	m.publishWatches = func() {
		entries := make([]filewatch.Entry, 0, min(len(m.docs), filewatch.MaxFiles))
		for _, d := range m.docs {
			if d.buffer == nil || d.untitled {
				continue
			}
			if len(entries) == filewatch.MaxFiles {
				m.message = "Automatic file watching is limited to 128 open editable documents"
				break
			}
			if d.watchID == 0 {
				m.watchSequence++
				d.watchID = m.watchSequence
			}
			entries = append(entries, filewatch.Entry{ID: d.watchID, Path: d.path, Version: d.buffer.Version(), ReloadID: d.reloadID, Hash: d.diskHash, Known: d.diskKnown})
		}
		_ = watcher.Update(filewatch.State{Entries: entries, Paused: m.saveBusy || len(m.saveJobs) > 0})
	}
	m.publishWatches()
	return watcher.Close
}
func (m *model) applyDiskResult(result filewatch.Result) bool {
	var d *document
	for _, candidate := range m.docs {
		if candidate.watchID == result.Entry.ID && candidate.path == result.Entry.Path {
			d = candidate
			break
		}
	}
	if d == nil || d.buffer == nil {
		return true
	}
	if m.saveBusy || len(m.saveJobs) > 0 {
		return false
	}
	if d.diskKnown != result.Entry.Known || d.diskHash != result.Entry.Hash {
		return false
	}
	explicit := result.Entry.ReloadID != 0 && d.reloadID == result.Entry.ReloadID
	if d.buffer.Version() != result.Entry.Version {
		if explicit {
			d.reloadID = 0
			m.reloadBusy = false
			m.reloadError = "New unsaved edits arrived; review and retry"
			m.publishWatches()
		}
		return false
	}
	if result.Kind == "text" && result.Hash == d.diskHash && d.diskKnown && !explicit {
		d.diskConflict = nil
		return true
	}
	if result.Kind != "text" || d.dirty() && !explicit {
		message := "Changed on disk. Unsaved edits are preserved."
		if result.Kind == "missing" {
			message = "Deleted on disk. Editor contents are preserved."
		}
		if result.Err != nil {
			message = result.Err.Error()
		}
		d.diskConflict = &diskConflict{result.Kind, message, result.Hash}
		if explicit {
			d.reloadID = 0
			m.reloadBusy = false
			m.reloadError = message
			m.publishWatches()
		}
		return true
	}
	change, err := d.buffer.Reload(result.Text)
	if err != nil {
		m.message = err.Error()
		return true
	}
	d.diskHash, d.diskKnown, d.diskConflict, d.reloadID = result.Hash, true, nil, 0
	d.followSelection()
	d.scroll = max(0, min(d.scroll, d.buffer.LineCount()-1))
	m.documentEvent("change", d, change)
	m.message = "Reloaded " + filepath.Base(d.path) + " from disk"
	if explicit {
		m.reloadPrompt = nil
		m.reloadBusy = false
		m.reloadError = ""
		m.editing = true
	}
	return true
}
func (m *model) beginReload(d *document) {
	if d == nil || d.buffer == nil || m.publishWatches == nil || m.saveBusy || len(m.saveJobs) > 0 {
		return
	}
	if !d.dirty() {
		m.requestDiskReload(d)
		return
	}
	m.reloadPrompt, m.reloadBusy, m.reloadError = d, false, ""
	m.pointerSelecting, m.terminalFocused = false, false
}
func (m *model) requestDiskReload(d *document) {
	if d == nil || !m.ownsDocument(d) || m.publishWatches == nil || m.saveBusy || len(m.saveJobs) > 0 {
		return
	}
	m.reloadSequence++
	d.reloadID = m.reloadSequence
	m.reloadBusy = m.reloadPrompt != nil
	m.reloadError = "Reading current disk contents…"
	m.publishWatches()
}
func (m *model) cancelReload() {
	if m.reloadBusy {
		return
	}
	m.reloadPrompt, m.reloadError = nil, ""
}
func (m *model) overwriteDisk(d *document) {
	if d == nil || d.diskConflict == nil || d.diskConflict.kind != "text" || m.requestSave == nil || m.saveBusy || len(m.saveJobs) > 0 {
		return
	}
	// This explicit action rebases only to the version the conflict UI observed.
	// The save worker still verifies it before writing and before atomic rename.
	d.diskHash, d.diskKnown = d.diskConflict.hash, true
	m.requestSave(context.Background(), []*document{d}, func(err error) {
		if err != nil {
			m.message = err.Error()
		}
	})
}
func (m *model) diskBanner(d *document) *ui.Element {
	conflict := d.diskConflict
	actions := []*ui.Element{label(conflict.message).Flex(1)}
	if conflict.kind == "text" {
		actions = append(actions, button("Reload from Disk", "disk-reload", func(*ui.Context) { m.beginReload(d) }).Height(30), button("Overwrite Disk", "disk-overwrite", func(*ui.Context) { m.overwriteDisk(d) }).Height(30))
	}
	return ui.Row(actions...).PaddingXY(8, 0).Height(38).Background(ui.RGB(0x332b00))
}
func (m *model) reloadOverlay(base *ui.Element) *ui.Element {
	d := m.reloadPrompt
	items := []*ui.Element{label("Reload from disk?").FontSize(17).Height(30), label(fmt.Sprintf("Unsaved changes in %s will be discarded.", filepath.Base(d.path))).Height(24)}
	for _, line := range wrapChatText(m.reloadError, 390) {
		items = append(items, label(line).FontSize(12).Height(20))
	}
	var reload, cancel func(*ui.Context)
	if !m.reloadBusy {
		reload = func(*ui.Context) { m.requestDiskReload(d) }
		cancel = func(*ui.Context) { m.cancelReload() }
	}
	items = append(items, ui.Row(spacer(), button("Reload", "reload-confirm", reload).Width(100).Height(30).Background(ui.RGB(accent)), button("Cancel", "reload-cancel", cancel).Width(90).Height(30)).Gap(8).Height(40))
	popup := ui.Column(items...).Padding(22).Width(460).Background(ui.RGB(0x252526))
	shade := ui.Column().Background(ui.RGBA(0, .65)).OnClick(func(*ui.Context) {})
	center := ui.Column(spacer(), ui.Row(spacer(), popup, spacer()).Height(float32(140+20*len(wrapChatText(m.reloadError, 390)))), spacer())
	return ui.Stack(base, shade, center)
}
