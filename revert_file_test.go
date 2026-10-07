package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gocode/internal/filewatch"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

// These fixtures exercise the real bounded disk worker without a native window.
// The UI owns all model edits and executes each worker receipt on this goroutine.
func revertModel(t *testing.T, count int) *model {
	t.Helper()
	m := &model{workspace: t.TempDir(), active: count - 1, editing: true}
	for i := 0; i < count; i++ {
		path := filepath.Join(m.workspace, fmt.Sprintf("file-%03d 世界.go", i))
		content := fmt.Sprintf("original %d\r\n😀\r\n", i)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		buffer, err := textbuffer.New(content)
		if err != nil {
			t.Fatal(err)
		}
		m.docs = append(m.docs, &document{path: path, buffer: buffer, diskHash: sha256.Sum256([]byte(content)), diskKnown: true})
	}
	return m
}

func revertReceipt(t *testing.T, mailbox chan func()) func() {
	t.Helper()
	select {
	case receipt := <-mailbox:
		return receipt
	case <-time.After(20 * time.Second):
		// At the cap, the unchanged paths can consume 128 existing 100 ms
		// bounded read ticks before the explicit target is chosen by the worker.
		t.Fatal("bounded disk worker did not deliver the revert receipt")
		return nil
	}
}

func TestRevertFile129thEditableUsesBoundedWorkerAndRestoresWatchSlots(t *testing.T) {
	m := revertModel(t, filewatch.MaxFiles+1)
	target := m.current()
	mailbox := watchActor(t, m)
	if target.watchID != 0 || m.docs[filewatch.MaxFiles-1].watchID == 0 {
		t.Fatal("ordinary watches no longer respect the 128-path cap")
	}
	if err := m.applyDocumentEdits(target.path, target.buffer.Version(), []textbuffer.Edit{{Range: textbuffer.Range{}, Text: "local "}}); err != nil {
		t.Fatal(err)
	}
	local := target.buffer.Text()
	version := target.buffer.Version()
	const disk = "current disk\r\n世界 😀\r\n"
	replaceOnDisk(t, target.path, disk)
	m.beginReload(target)
	if m.reloadPrompt != target || m.reloadBusy || target.buffer.Text() != local {
		t.Fatal("dirty revert skipped discard confirmation")
	}
	m.requestDiskReload(target)
	if target.watchID == 0 || target.reloadID == 0 || !m.reloadBusy {
		t.Fatal("129th explicit target was not admitted")
	}
	other := m.docs[0]
	m.requestDiskReload(other)
	if other.reloadID != 0 || !m.reloadBusy || target.reloadID == 0 || !strings.Contains(m.message, "Another reload") {
		t.Fatal("multiple explicit bodies were admitted or the first request was lost")
	}
	revertReceipt(t, mailbox)()
	if m.reloadBusy || m.reloadPrompt != nil || target.reloadID != 0 || target.buffer.Text() != disk || target.dirty() || target.buffer.Version() != version+1 {
		t.Fatal("129th editable file did not finish its real disk reload")
	}
	// The temporarily evicted 128th slot must resume ordinary watching after
	// completion; raising the cap or permanently replacing a slot is not a fix.
	ordinary := m.docs[filewatch.MaxFiles-1]
	replaceOnDisk(t, ordinary.path, "restored ordinary watch\n")
	revertReceipt(t, mailbox)()
	if ordinary.buffer.Text() != "restored ordinary watch\n" || ordinary.dirty() {
		t.Fatal("normal 128th watch slot was not restored")
	}
}

func TestRevertFileOldOrdinaryReceiptCannotClearExplicitRead(t *testing.T) {
	m := revertModel(t, 1)
	d := m.current()
	mailbox := watchActor(t, m)
	replaceOnDisk(t, d.path, "queued old disk\n")
	old := revertReceipt(t, mailbox)
	before := d.buffer.Text()
	version := d.buffer.Version()
	replaceOnDisk(t, d.path, "latest current disk\r\n😀\r\n")
	m.beginReload(d) // A clean file still has an explicit asynchronous request.
	requested := d.reloadID
	old()
	if !m.reloadBusy || d.reloadID != requested || d.buffer.Text() != before || d.buffer.Version() != version {
		t.Fatal("queued ordinary receipt adopted obsolete text or cancelled explicit read")
	}
	revertReceipt(t, mailbox)()
	if m.reloadBusy || d.reloadID != 0 || d.buffer.Text() != "latest current disk\r\n😀\r\n" || d.buffer.Version() != version+1 {
		t.Fatal("explicit read did not use the current disk revision")
	}
}

func TestRevertFileCancelledWorkerReceiptPreservesDirtyBuffer(t *testing.T) {
	m := revertModel(t, 1)
	d := m.current()
	if err := m.applyDocumentEdits(d.path, d.buffer.Version(), []textbuffer.Edit{{Range: textbuffer.Range{}, Text: "keep local "}}); err != nil {
		t.Fatal(err)
	}
	mailbox := watchActor(t, m)
	m.beginReload(d)
	m.requestDiskReload(d)
	held := revertReceipt(t, mailbox)
	local, version := d.buffer.Text(), d.buffer.Version()
	m.cancelReload()
	held()
	if m.reloadBusy || m.reloadPrompt != nil || d.reloadID != 0 || d.buffer.Text() != local || d.buffer.Version() != version || !d.dirty() {
		t.Fatal("cancelled read discarded dirty text or left busy")
	}
	// Prove normal watching survives cancellation rather than simply dropping
	// or stopping the worker. Its conflict still preserves the local revision.
	replaceOnDisk(t, d.path, "external after cancellation\n")
	revertReceipt(t, mailbox)()
	if d.diskConflict == nil || d.buffer.Text() != local || d.buffer.Version() != version || m.reloadBusy {
		t.Fatal("ordinary dirty conflict changed after cancelling revert")
	}
}

func TestRevertFileNewEditBeforeExplicitDiskReadDoesNotRebaseDiscard(t *testing.T) {
	m := revertModel(t, 1)
	d := m.current()
	mailbox := watchActor(t, m)
	replaceOnDisk(t, d.path, "external before revert\n")
	ordinary := revertReceipt(t, mailbox)
	m.beginReload(d)
	if !m.reloadBusy {
		t.Fatal("clean revert did not start an explicit read")
	}
	// The real worker remains blocked awaiting the older ordinary receipt.
	// This change publishes a new watch snapshot before any explicit read,
	// and must not silently become its newly authorized discard version.
	if err := m.applyDocumentEdits(d.path, d.buffer.Version(), []textbuffer.Edit{{Range: textbuffer.Range{}, Text: "VSIX after request "}}); err != nil {
		t.Fatal(err)
	}
	local, version := d.buffer.Text(), d.buffer.Version()
	ordinary()
	revertReceipt(t, mailbox)()
	if m.reloadBusy || d.reloadID != 0 || !strings.Contains(m.reloadError, "New unsaved edits") || d.buffer.Text() != local || d.buffer.Version() != version || !d.dirty() {
		t.Fatal("watch publication rebased explicit discard to a newer edit")
	}
}

func TestRevertFileNewEditDeletedAndOversizedDiskFinishWithoutDiscard(t *testing.T) {
	for _, scenario := range []string{"new-edit", "deleted", "oversized"} {
		t.Run(scenario, func(t *testing.T) {
			m := revertModel(t, 1)
			d := m.current()
			if err := m.applyDocumentEdits(d.path, d.buffer.Version(), []textbuffer.Edit{{Range: textbuffer.Range{}, Text: "unsaved "}}); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "deleted":
				if err := os.Remove(d.path); err != nil {
					t.Fatal(err)
				}
			case "oversized":
				if err := os.Truncate(d.path, filewatch.MaxBytes+1); err != nil {
					t.Fatal(err)
				}
			}
			mailbox := watchActor(t, m)
			m.beginReload(d)
			m.requestDiskReload(d)
			held := revertReceipt(t, mailbox)
			if scenario == "new-edit" {
				if err := m.applyDocumentEdits(d.path, d.buffer.Version(), []textbuffer.Edit{{Range: textbuffer.Range{}, Text: "new VSIX edit "}}); err != nil {
					t.Fatal(err)
				}
			}
			local, version := d.buffer.Text(), d.buffer.Version()
			held()
			// An initial ordinary receipt may have been queued just before the
			// explicit snapshot; that receipt must not finish the current request.
			if m.reloadBusy {
				revertReceipt(t, mailbox)()
			}
			if m.reloadBusy || d.reloadID != 0 || d.buffer.Text() != local || d.buffer.Version() != version || !d.dirty() {
				t.Fatal("failed revert discarded local revision or left busy")
			}
			want := map[string]string{"new-edit": "New unsaved edits", "deleted": "Deleted on disk", "oversized": "8 MiB"}[scenario]
			if !strings.Contains(m.reloadError, want) {
				t.Fatalf("failure was not explained: %q", m.reloadError)
			}
			m.cancelReload()
		})
	}
}

func TestRevertFileClosedTargetAndWorkerShutdownReleaseBusy(t *testing.T) {
	for _, closeDocument := range []bool{true, false} {
		t.Run(fmt.Sprint(closeDocument), func(t *testing.T) {
			m := revertModel(t, 1)
			d := m.current()
			mailbox := make(chan func(), 1)
			stop := m.startDocumentWatch(context.Background(), func(fn func()) bool { mailbox <- fn; return true })
			t.Cleanup(stop)
			m.beginReload(d)
			held := revertReceipt(t, mailbox)
			before, version := d.buffer.Text(), d.buffer.Version()
			if closeDocument {
				m.removeTab(m.active)
			} else {
				stop()
			}
			held()
			if m.reloadBusy || d.reloadID != 0 || d.buffer.Text() != before || d.buffer.Version() != version || !strings.Contains(m.reloadError, "cancelled") {
				t.Fatal("closed target/stopped worker left an active discard request")
			}
		})
	}
}

func TestRevertFileRejectsIneligibleTargetsAndBusyOperations(t *testing.T) {
	for _, scenario := range []string{"untitled", "large", "closed", "no-watcher", "saving", "save-queued", "save-as", "closing"} {
		t.Run(scenario, func(t *testing.T) {
			m := revertModel(t, 1)
			d := m.current()
			published := 0
			m.publishWatches = func() { published++ }
			switch scenario {
			case "untitled":
				d.untitled = true
			case "large":
				d.large = &largeDocument{}
			case "closed":
				m.docs = nil
			case "no-watcher":
				m.publishWatches = nil
			case "saving":
				m.saveBusy = true
			case "save-queued":
				m.saveJobs = []saveJob{{}}
			case "save-as":
				m.fileActions.busy = true
			case "closing":
				m.closeBusy = true
			}
			before, version := d.buffer.Text(), d.buffer.Version()
			m.beginReload(d)
			m.requestDiskReload(d)
			if d.reloadID != 0 || m.reloadBusy || m.reloadPrompt != nil || published != 0 || m.reloadError == "" || m.message == "" || d.buffer.Text() != before || d.buffer.Version() != version {
				t.Fatal("ineligible revert was accepted or failed without explanation")
			}
		})
	}
}
