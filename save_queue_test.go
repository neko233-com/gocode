package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	textbuffer "github.com/neko233-com/godesktop/editor"
)

// A real disk worker posts into this deterministic UI mailbox. Holding an ack
// reproduces edits/queued saves during I/O without timing-dependent file hooks.
func saveActor(t *testing.T, m *model) chan func() {
	t.Helper()
	mailbox := make(chan func(), 64)
	stop := m.startSaveActor(context.Background(), func(fn func()) bool { mailbox <- fn; return true })
	t.Cleanup(stop)
	return mailbox
}
func saveAck(t *testing.T, mailbox chan func()) func() {
	t.Helper()
	select {
	case fn := <-mailbox:
		return fn
	case <-time.After(5 * time.Second):
		t.Fatal("disk worker did not acknowledge")
		return nil
	}
}
func TestSaveQueueFrozenSnapshotsAndRebasedDiskExpectation(t *testing.T) {
	m := testModel(t)
	d := m.current()
	mailbox := saveActor(t, m)
	text(m, "first")
	first := d.buffer.Text()
	var firstErr, secondErr error
	firstDone, secondDone := false, false
	m.requestSave(context.Background(), []*document{d}, func(err error) { firstErr, firstDone = err, true })
	ack := saveAck(t, mailbox)
	text(m, "newer")
	newer := d.buffer.Text()
	m.requestSave(context.Background(), []*document{d}, func(err error) { secondErr, secondDone = err, true })
	if data, _ := os.ReadFile(d.path); string(data) != first || !d.dirty() || firstDone || secondDone {
		t.Fatal("worker changed UI state or queued save raced ahead")
	}
	ack()
	if !errors.Is(firstErr, errSaveChanged) || !d.dirty() || !firstDone {
		t.Fatal("old snapshot acknowledged newer edits", firstErr)
	}
	saveAck(t, mailbox)()
	if !secondDone || secondErr != nil || d.dirty() || m.saveBusy || len(m.saveJobs) != 0 || d.saveID != 1 {
		t.Fatal("successive save did not rebase/acknowledge", secondErr)
	}
	if data, _ := os.ReadFile(d.path); string(data) != newer {
		t.Fatal("latest snapshot not persisted")
	}
}
func TestSaveQueueCancellationBoundsAndCloseBarrier(t *testing.T) {
	m := testModel(t)
	d := m.current()
	mailbox := saveActor(t, m)
	text(m, "save")
	m.requestSave(context.Background(), []*document{d}, nil)
	ack := saveAck(t, mailbox)
	ctx, cancel := context.WithCancel(context.Background())
	var cancelled error
	m.requestSave(ctx, []*document{d}, func(err error) { cancelled = err })
	cancel()
	for range maxSaveJobs - 1 {
		m.requestSave(context.Background(), nil, nil)
	}
	var full error
	m.requestSave(context.Background(), []*document{d}, func(err error) { full = err })
	if full == nil || !strings.Contains(full.Error(), "queue is full") || len(m.saveJobs) != maxSaveJobs {
		t.Fatal("save queue not bounded", full)
	}
	if m.requestWindowClose(nil) || !m.closePrompt {
		t.Fatal("in-flight close did not defer")
	}
	ack()
	for m.saveBusy {
		saveAck(t, mailbox)()
	}
	if !errors.Is(cancelled, context.Canceled) || d.dirty() || len(m.saveJobs) != 0 {
		t.Fatal("cancel/barrier did not drain", cancelled)
	}
	m.cancelClose()
	if !m.requestWindowClose(nil) {
		t.Fatal("completed save still blocks close")
	}
}
func TestSaveQueueExternalChangesAndClosedDocuments(t *testing.T) {
	for _, mode := range []string{"external", "closed"} {
		t.Run(mode, func(t *testing.T) {
			m := testModel(t)
			d := m.current()
			mailbox := saveActor(t, m)
			text(m, "first")
			m.requestSave(context.Background(), []*document{d}, nil)
			ack := saveAck(t, mailbox)
			text(m, "later")
			var result error
			m.requestSave(context.Background(), []*document{d}, func(err error) { result = err })
			if mode == "external" {
				if err := os.WriteFile(d.path, []byte("external editor"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				m.removeTab(m.active)
			}
			ack()
			if m.saveBusy {
				saveAck(t, mailbox)()
			}
			if result == nil || !d.dirty() {
				t.Fatal("conflicting/closed save accepted", result)
			}
			if mode == "external" {
				if data, _ := os.ReadFile(d.path); string(data) != "external editor" {
					t.Fatal("external source overwritten")
				}
			}
		})
	}
}

func TestSaveQueueFrozenPathGuardsReceiptAndQueuedDiskExpectation(t *testing.T) {
	m := testModel(t)
	t.Cleanup(m.closeDocuments)
	d := m.current()
	oldPath := d.path
	mailbox := saveActor(t, m)
	var saved int
	m.onDocument = func(kind string, target *document, _ textbuffer.ChangeEvent) {
		if kind == "save" && target == d {
			saved++
		}
	}
	text(m, "old URI first 世界😀")
	first := d.buffer.Text()
	var firstErr, queuedErr error
	firstDone, queuedDone := false, false
	m.requestSave(context.Background(), []*document{d}, func(err error) { firstErr, firstDone = err, true })
	ack := saveAck(t, mailbox)
	text(m, "latest unsaved new URI\r\n")
	wanted := d.buffer.Text()
	m.requestSave(context.Background(), []*document{d}, func(err error) { queuedErr, queuedDone = err, true })
	newPath := filepath.Join(m.workspace, "renamed-owned-document.go")
	newDisk := "different committed target\r\n// 世界😀\r\n"
	if err := os.WriteFile(newPath, []byte(newDisk), 0600); err != nil {
		t.Fatal(err)
	}
	newHash := sha256.Sum256([]byte(newDisk))
	// Apply the UI's path/hash adoption while a real old-path receipt and one
	// frozen old-path job remain pending. This checks the writer's own identity
	// defenses independently of Save As admission exclusion.
	d.path, d.diskHash, d.diskKnown = newPath, newHash, true
	ack()
	if m.saveBusy {
		saveAck(t, mailbox)()
	}
	if !firstDone || !queuedDone || !errors.Is(firstErr, errSaveChanged) || !errors.Is(queuedErr, errSaveChanged) || m.saveBusy || len(m.saveJobs) != 0 || !d.dirty() || d.saveID != 0 || saved != 0 || d.diskHash != newHash || !d.diskKnown || d.diskConflict != nil {
		t.Fatal("old-path receipt or queued job adopted new-path disk state", firstErr, queuedErr, saved)
	}
	if autoSaveDisk(t, oldPath) != first || autoSaveDisk(t, newPath) != newDisk {
		t.Fatal("queued old-path writer ran after identity adoption")
	}
	var latestErr error
	m.requestSave(context.Background(), []*document{d}, func(err error) { latestErr = err })
	saveAck(t, mailbox)()
	if latestErr != nil || d.dirty() || d.saveID != 1 || saved != 1 || autoSaveDisk(t, newPath) != wanted || autoSaveDisk(t, oldPath) != first {
		t.Fatal("new-path writer did not retain its own hash expectation and exact save event", latestErr)
	}
}
