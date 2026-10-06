package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gocode/internal/filewatch"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

func watchActor(t *testing.T, m *model) chan func() {
	t.Helper()
	mailbox := make(chan func(), 1)
	stop := m.startDocumentWatch(context.Background(), func(fn func()) bool { mailbox <- fn; return true })
	t.Cleanup(stop)
	return mailbox
}
func watchAck(t *testing.T, mailbox chan func()) func() {
	t.Helper()
	select {
	case fn := <-mailbox:
		return fn
	case <-time.After(6 * time.Second):
		t.Fatal("file watch did not acknowledge")
		return nil
	}
}
func replaceOnDisk(t *testing.T, path, text string) {
	t.Helper()
	temp := filepath.Join(filepath.Dir(path), ".external-save")
	if err := os.WriteFile(temp, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	if err := filewatch.Replace(context.Background(), temp, path, nil); err != nil {
		t.Fatal(err)
	}
}
func TestExternalReloadActualWorkerVersionsUndoAndDirtyConflict(t *testing.T) {
	m := testModel(t)
	d := m.current()
	mailbox := watchActor(t, m)
	before := d.buffer.Snapshot()
	var changes []textbuffer.ChangeEvent
	m.onDocument = func(kind string, doc *document, event textbuffer.ChangeEvent) {
		if kind == "change" {
			changes = append(changes, event)
		}
	}
	replaceOnDisk(t, d.path, "disk\r\n😀\r\n")
	watchAck(t, mailbox)()
	if d.buffer.Version() != before.Version+1 || d.dirty() || d.buffer.Text() != "disk\r\n😀\r\n" || len(changes) != 1 || changes[0].Version != d.buffer.Version() {
		t.Fatal("clean reload did not sync monotonic version")
	}
	change, _ := d.buffer.Undo()
	m.changed(d, change, nil)
	if d.buffer.Text() != before.Text() || !d.dirty() {
		t.Fatal("reload is not undoable")
	}
	local := d.buffer.Text()
	version := d.buffer.Version()
	replaceOnDisk(t, d.path, "external newer\n")
	watchAck(t, mailbox)()
	if d.buffer.Text() != local || d.buffer.Version() != version || d.diskConflict == nil || !d.dirty() {
		t.Fatal("dirty buffer overwritten")
	}
	m.beginReload(d)
	m.cancelReload()
	if m.reloadPrompt != nil || d.buffer.Text() != local || !d.dirty() {
		t.Fatal("cancel discarded edits")
	}
	m.beginReload(d)
	m.requestDiskReload(d)
	watchAck(t, mailbox)()
	if m.reloadPrompt != nil || d.dirty() || d.buffer.Text() != "external newer\n" || d.diskConflict != nil || d.buffer.Version() != version+1 {
		t.Fatal("explicit worker reload failed")
	}
}

func TestStaleWatchReadAfterTypingAndReopenDoesNotReplaceBuffer(t *testing.T) {
	m := testModel(t)
	d := m.current()
	mailbox := watchActor(t, m)
	replaceOnDisk(t, d.path, "external")
	held := watchAck(t, mailbox)
	text(m, "local")
	local := d.buffer.Text()
	held()
	watchAck(t, mailbox)()
	if d.buffer.Text() != local || !d.dirty() || d.diskConflict == nil {
		t.Fatal("read race discarded newer edit")
	}
	// A stale reply must not act on a new document opened at the same path.
	m.removeTab(m.active)
	m.open(d.path)
	newDoc := m.current()
	stale, _ := filewatch.Read(context.Background(), filewatch.Entry{ID: d.watchID, Path: d.path, Version: d.buffer.Version(), Hash: d.diskHash, Known: true})
	stale.Text = "stale"
	m.applyDiskResult(stale)
	if newDoc == d || newDoc.buffer.Text() != "external" {
		t.Fatal("closed identity applied to reopened doc")
	}
}

func TestSaveAcknowledgementReconcilesOwnRenameAndExternalConflict(t *testing.T) {
	m := testModel(t)
	d := m.current()
	watchbox := watchActor(t, m)
	savebox := saveActor(t, m)
	text(m, "local")
	m.saveActive()
	saved := saveAck(t, savebox)
	replaceOnDisk(t, d.path, "external after our write")
	saved()
	watchAck(t, watchbox)()
	if d.dirty() || d.buffer.Text() != "external after our write" || d.diskConflict != nil {
		t.Fatal("post-save external disk state not reconciled")
	}
	text(m, "keep")
	unsaved := d.buffer.Text()
	replaceOnDisk(t, d.path, "conflicting disk")
	watchAck(t, watchbox)()
	replaceOnDisk(t, d.path, "newer disk before overwrite")
	m.overwriteDisk(d)
	saveAck(t, savebox)()
	if data, _ := os.ReadFile(d.path); string(data) != "newer disk before overwrite" || !d.dirty() || d.buffer.Text() != unsaved || !strings.Contains(m.message, "changed on disk") {
		t.Fatal("overwrite ignored newer disk version", m.message)
	}
}

func TestConfirmedReloadRejectsNewVSIXEditAndMissingFilePreservesText(t *testing.T) {
	m := testModel(t)
	d := m.current()
	mailbox := watchActor(t, m)
	text(m, "local")
	replaceOnDisk(t, d.path, "external")
	watchAck(t, mailbox)()
	m.beginReload(d)
	m.requestDiskReload(d)
	held := watchAck(t, mailbox)
	err := m.applyDocumentEdits(d.path, d.buffer.Version(), []textbuffer.Edit{{Range: textbuffer.Range{}, Text: "new VSIX edit"}})
	if err != nil {
		t.Fatal(err)
	}
	local := d.buffer.Text()
	version := d.buffer.Version()
	held()
	if d.buffer.Text() != local || !d.dirty() || m.reloadBusy || !strings.Contains(m.reloadError, "New unsaved edits") {
		t.Fatal("pending reload discarded accepted VSIX edit")
	}
	m.cancelReload()
	watchAck(t, mailbox)()
	if err := os.Remove(d.path); err != nil {
		t.Fatal(err)
	}
	watchAck(t, mailbox)()
	if d.diskConflict == nil || d.diskConflict.kind != "missing" || d.buffer.Text() != local || d.buffer.Version() != version {
		t.Fatal("deletion discarded buffer")
	}
}
