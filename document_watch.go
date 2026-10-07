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
	var reloadTarget *document
	var reloadEntry filewatch.Entry
	m.publishWatches = func() {
		// A requested revert must not wait forever behind the normal watch cap.
		// Keep one explicit target first, then spend the remaining bounded slots
		// on ordinary watches. The normal first 128 resume after its receipt.
		if reloadTarget != nil && reloadTarget.reloadID != 0 && (!m.ownsDocument(reloadTarget) || reloadTarget.buffer == nil || reloadTarget.untitled || reloadTarget.large != nil || reloadTarget.path != reloadEntry.Path) {
			reloadTarget.reloadID = 0
			m.reloadBusy = false
			m.reloadPrompt = nil
			m.reloadError = "The document was closed; reload was cancelled"
			m.message = m.reloadError
		}
		reloadTarget = nil
		for _, d := range m.docs {
			if d.reloadID != 0 {
				reloadTarget = d
				break
			}
		}
		if reloadTarget == nil {
			reloadEntry = filewatch.Entry{}
		}
		entries := make([]filewatch.Entry, 0, min(len(m.docs), filewatch.MaxFiles))
		appendEntry := func(d *document) {
			if d.watchID == 0 {
				m.watchSequence++
				d.watchID = m.watchSequence
			}
			entry := filewatch.Entry{ID: d.watchID, Path: d.path, Version: d.buffer.Version(), ReloadID: d.reloadID, Hash: d.diskHash, Known: d.diskKnown}
			if d == reloadTarget {
				if reloadEntry.ID != entry.ID || reloadEntry.ReloadID != entry.ReloadID {
					reloadEntry = entry
				}
				// Typing/VSIX changes publish normal subscriptions, but must not
				// rebase a confirmed discard to their newer buffer revision.
				entry = reloadEntry
			}
			entries = append(entries, entry)
		}
		if reloadTarget != nil {
			appendEntry(reloadTarget)
		}
		for _, d := range m.docs {
			if d.buffer == nil || d.untitled || d.large != nil || d == reloadTarget {
				continue
			}
			if len(entries) == filewatch.MaxFiles {
				if m.reloadError == "" {
					m.message = "Automatic file watching is limited to 128 open editable documents"
				}
				break
			}
			appendEntry(d)
		}
		if err := watcher.Update(filewatch.State{Entries: entries, Paused: m.saveBusy || len(m.saveJobs) > 0}); err != nil {
			if reloadTarget != nil {
				reloadTarget.reloadID = 0
				m.reloadBusy = false
				m.reloadError = err.Error()
			}
			m.message = err.Error()
		}
	}
	m.publishWatches()
	return func() {
		watcher.Close()
		if reloadTarget != nil && reloadTarget.reloadID != 0 {
			reloadTarget.reloadID = 0
			m.reloadBusy = false
			m.reloadError = "File watching stopped; reload was cancelled"
		}
		m.publishWatches = nil
	}
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
	// A queued ordinary watch or cancelled revert receipt is not the current
	// explicit read, even if its document version and disk hash still match.
	if result.Entry.ReloadID != d.reloadID {
		return false
	}
	explicit := result.Entry.ReloadID != 0
	if d.diskKnown != result.Entry.Known || d.diskHash != result.Entry.Hash {
		if explicit {
			m.finishDiskReload(d, "Disk baseline changed; review and retry")
		}
		return false
	}
	if d.buffer.Version() != result.Entry.Version {
		if explicit {
			m.finishDiskReload(d, "New unsaved edits arrived; review and retry")
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
			m.finishDiskReload(d, message)
		}
		return true
	}
	change, err := d.buffer.Reload(result.Text)
	if err != nil {
		m.message = err.Error()
		if explicit {
			m.finishDiskReload(d, err.Error())
		}
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
	}
	return true
}
func (m *model) beginReload(d *document) {
	if !m.canRequestDiskReload(d) {
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
	if !m.canRequestDiskReload(d) {
		return
	}
	m.reloadSequence++
	d.reloadID = m.reloadSequence
	m.reloadBusy = true
	m.reloadError = "Reading current disk contents…"
	m.message = m.reloadError
	m.publishWatches()
}
func (m *model) canRequestDiskReload(d *document) bool {
	message := ""
	switch {
	case d == nil || !m.ownsDocument(d):
		message = "No open document to reload"
	case d.untitled:
		message = "Untitled documents have no disk contents to reload"
	case d.buffer == nil || d.large != nil:
		message = "Large-file browsing is read-only; reload requires an editable document"
	case m.publishWatches == nil:
		message = "File watching is unavailable"
	case m.saveBusy || len(m.saveJobs) > 0 || m.fileActions.busy || m.closeBusy:
		message = "A file operation is busy; retry reload after it finishes"
	case m.reloadBusy || m.reloadPrompt != nil && m.reloadPrompt != d:
		message = "Another reload is pending; finish or cancel it first"
	default:
		for _, candidate := range m.docs {
			if candidate.reloadID != 0 {
				message = "Another reload is pending; finish or cancel it first"
				break
			}
		}
	}
	if message != "" {
		m.message = message
		// Keep the current request's status intact when rejecting a second one.
		if !m.reloadBusy {
			m.reloadError = message
		}
		return false
	}
	return true
}
func (m *model) finishDiskReload(d *document, message string) {
	d.reloadID = 0
	m.reloadBusy = false
	m.reloadError = message
	m.message = message
	if m.publishWatches != nil {
		m.publishWatches()
	}
}
func (m *model) cancelReload() {
	for _, d := range m.docs {
		d.reloadID = 0
	}
	m.reloadPrompt, m.reloadBusy, m.reloadError = nil, false, ""
	if m.publishWatches != nil {
		m.publishWatches()
	}
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
	var reload func(*ui.Context)
	cancel := func(*ui.Context) { m.cancelReload() }
	if !m.reloadBusy {
		reload = func(*ui.Context) { m.requestDiskReload(d) }
	}
	items = append(items, ui.Row(spacer(), button("Reload", "reload-confirm", reload).Width(100).Height(30).Background(ui.RGB(accent)), button("Cancel", "reload-cancel", cancel).Width(90).Height(30)).Gap(8).Height(40))
	popup := ui.Column(items...).Padding(22).Width(460).Radius(8).ClipRounded(8).Background(ui.RGB(0x252526)).Key("reload-dialog")
	shade := ui.Column().Background(ui.RGBA(0, .65)).OnClick(func(*ui.Context) {})
	center := ui.Column(spacer(), ui.Row(spacer(), popup, spacer()).Height(float32(140+20*len(wrapChatText(m.reloadError, 390)))), spacer())
	return ui.Stack(base, shade, center)
}
