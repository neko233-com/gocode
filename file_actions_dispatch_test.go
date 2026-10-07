package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	textbuffer "github.com/neko233-com/godesktop/editor"
)

type workerDispatchGate struct {
	mailbox  chan func()
	rejected chan struct{}
	allow    atomic.Bool
	attempts atomic.Int32
}

func newWorkerDispatchGate() *workerDispatchGate {
	return &workerDispatchGate{mailbox: make(chan func(), 1), rejected: make(chan struct{}, 1)}
}

func (g *workerDispatchGate) dispatch(fn func()) bool {
	g.attempts.Add(1)
	if !g.allow.Load() {
		select {
		case g.rejected <- struct{}{}:
		default:
		}
		return false
	}
	select {
	case g.mailbox <- fn:
		return true
	default:
		return false
	}
}

func (g *workerDispatchGate) waitRejected(t *testing.T) {
	t.Helper()
	select {
	case <-g.rejected:
	case <-time.After(5 * time.Second):
		t.Fatal("actual worker did not reach UI queue rejection")
	}
}

func cancelledFileChooser(context.Context, string, string, string) (string, error) {
	return "", errDialogCancelled
}

func TestFileDialogWorkerRetriesRejectedReceiptWithoutUIThreadIO(t *testing.T) {
	m := testModel(t)
	t.Cleanup(m.closeDocuments)
	saveActor(t, m)
	gate := newWorkerDispatchGate()
	var choices atomic.Int32
	workspace, initialPath := m.workspace, m.current().path
	stop := m.bindFileActions(context.Background(), gate.dispatch, func(ctx context.Context, root, kind, initial string) (string, error) {
		choices.Add(1)
		if root != workspace || kind != "open" || initial != initialPath {
			return "", errors.New("chooser received different frozen arguments")
		}
		return "", errDialogCancelled
	})
	defer stop()
	done := false
	var result error
	m.fileActions.choose("open", m.current().path, func(_ string, err error) { done, result = true, err })
	gate.waitRejected(t)
	if !m.fileActions.busy || done || choices.Load() != 1 {
		t.Fatal("dialog result mutated UI before queue acceptance")
	}
	gate.allow.Store(true)
	saveAck(t, gate.mailbox)()
	if m.fileActions.busy || !done || !errors.Is(result, errDialogCancelled) || choices.Load() != 1 || gate.attempts.Load() < 2 {
		t.Fatal("cancelled dialog receipt was lost after overload", result)
	}
}

func TestSaveAsWorkerRetriesImmutableReceiptAndDoesNotFalselySaveNewerText(t *testing.T) {
	m, diskMailbox, _ := autoSaveModel(t, "afterDelay")
	d := m.current()
	oldPath, oldDisk := d.path, autoSaveDisk(t, d.path)
	gate := newWorkerDispatchGate()
	stop := m.bindFileActions(context.Background(), gate.dispatch, cancelledFileChooser)
	defer stop()
	var saved int
	var closedPath, openedPath string
	m.onDocument = func(kind string, target *document, _ textbuffer.ChangeEvent) {
		if target != d {
			return
		}
		switch kind {
		case "save":
			saved++
		case "close":
			closedPath = target.path
		case "open":
			openedPath = target.path
		}
	}
	text(m, "frozen Save As😀")
	frozen := d.buffer.Text()
	newPath := filepath.Join(m.workspace, "保存世界😀.go")
	done := false
	var result error
	m.fileActions.writeAs(context.Background(), d, newPath, func(err error) { done, result = true, err })
	gate.waitRejected(t)
	if autoSaveDisk(t, newPath) != frozen || d.path != oldPath || !m.fileActions.busy || done {
		t.Fatal("Save As worker lost the frozen disk/UI boundary")
	}
	text(m, "newer unsaved 世界")
	newer := d.buffer.Text()
	gate.allow.Store(true)
	saveAck(t, gate.mailbox)()
	if !done || !errors.Is(result, errSaveChanged) || m.fileActions.busy || !d.dirty() || d.path != newPath || d.buffer.Text() != newer || d.saveID != 0 || saved != 0 || closedPath != oldPath || openedPath != newPath {
		t.Fatal("Save As acknowledgement falsely saved newer text or lost URI events", result, saved)
	}
	if autoSaveDisk(t, newPath) != frozen || autoSaveDisk(t, oldPath) != oldDisk {
		t.Fatal("Save As receipt rewrote either disk snapshot")
	}
	if entry := m.autoSave.pending[d]; entry == nil || entry.version != d.buffer.Version() || entry.due.IsZero() {
		t.Fatal("new-path dirty edits lost their automatic-save schedule")
	}
	m.requestSave(context.Background(), []*document{d}, nil)
	saveAck(t, diskMailbox)()
	if d.dirty() || d.saveID != 1 || saved != 1 || autoSaveDisk(t, newPath) != newer {
		t.Fatal("newer renamed snapshot did not save through the shared writer")
	}
	leftovers, err := filepath.Glob(filepath.Join(m.workspace, ".gocode-save-as-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatal("Save As retry left temporary files", leftovers, err)
	}
}

func TestSaveAsPendingReceiptExcludesOriginalPathSharedWriter(t *testing.T) {
	for _, order := range []string{"SaveAs-first", "shared-first"} {
		t.Run(order, func(t *testing.T) {
			m, diskMailbox, now := autoSaveModel(t, "afterDelay")
			d := m.current()
			oldPath, oldDisk := d.path, autoSaveDisk(t, d.path)
			gate := newWorkerDispatchGate()
			stop := m.bindFileActions(context.Background(), gate.dispatch, cancelledFileChooser)
			defer stop()
			var saved, opened, closed int
			m.onDocument = func(kind string, target *document, _ textbuffer.ChangeEvent) {
				if target != d {
					return
				}
				switch kind {
				case "save":
					saved++
				case "open":
					opened++
				case "close":
					closed++
				}
			}
			text(m, "frozen copy 世界😀")
			frozen := d.buffer.Text()
			target := filepath.Join(m.workspace, "pending-save-as.go")
			var renamed error
			m.fileActions.writeAs(context.Background(), d, target, func(err error) { renamed = err })
			gate.waitRejected(t)
			if autoSaveDisk(t, target) != frozen || d.path != oldPath {
				t.Fatal("actual Save As disk receipt was not held before path adoption")
			}
			text(m, "newer original URI edits\r\n")
			wanted := d.buffer.Text()
			done := false
			var shared error
			m.requestSave(context.Background(), []*document{d}, func(err error) { done, shared = true, err })
			// Capture an overlapping real writer if the production exclusion ever
			// regresses, then exercise both possible acknowledgement orders.
			var oldReceipt func()
			if m.saveBusy {
				oldReceipt = saveAck(t, diskMailbox)
			}
			if order == "shared-first" && oldReceipt != nil {
				oldReceipt()
			}
			gate.allow.Store(true)
			saveAck(t, gate.mailbox)()
			if order == "SaveAs-first" && oldReceipt != nil {
				oldReceipt()
			}
			if !done || !errors.Is(shared, errSaveFileOperation) || !errors.Is(renamed, errSaveChanged) || m.saveBusy || len(m.saveJobs) != 0 || d.path != target || !d.dirty() || d.buffer.Text() != wanted || d.saveID != 0 || saved != 0 || opened != 1 || closed != 1 {
				t.Fatal("pending Save As admitted an original-path writer or falsely cleaned the new URI", shared, renamed, saved, d.dirty())
			}
			if autoSaveDisk(t, oldPath) != oldDisk || autoSaveDisk(t, target) != frozen {
				t.Fatal("original-path save overlapped the committed Save As copy")
			}
			m.autoSaveTick(now)
			saveAck(t, diskMailbox)()
			if d.dirty() || d.saveID != 1 || saved != 1 || autoSaveDisk(t, target) != wanted || autoSaveDisk(t, oldPath) != oldDisk {
				t.Fatal("latest renamed version did not use the shared automatic writer exactly once")
			}
			leftovers, err := filepath.Glob(filepath.Join(m.workspace, ".gocode-save*"))
			if err != nil || len(leftovers) != 0 {
				t.Fatal("Save As/shared writer overlap left temporary files", leftovers, err)
			}
		})
	}
}

func TestSaveAsStartedByCompletionExcludesAlreadyQueuedOriginalWriter(t *testing.T) {
	m, diskMailbox, now := autoSaveModel(t, "afterDelay")
	d := m.current()
	oldPath := d.path
	gate := newWorkerDispatchGate()
	stop := m.bindFileActions(context.Background(), gate.dispatch, cancelledFileChooser)
	defer stop()
	target := filepath.Join(m.workspace, "save-as-from-completion.go")
	text(m, "first original disk 世界😀")
	first := d.buffer.Text()
	var firstErr, queuedErr, renamed error
	firstDone, queuedDone := false, false
	m.requestSave(context.Background(), []*document{d}, func(err error) {
		firstErr, firstDone = err, true
		m.fileActions.writeAs(context.Background(), d, target, func(err error) { renamed = err })
	})
	ack := saveAck(t, diskMailbox)
	text(m, "frozen queued copy\r\n")
	copyText := d.buffer.Text()
	m.requestSave(context.Background(), []*document{d}, func(err error) { queuedErr, queuedDone = err, true })
	ack() // Completion begins a real Save As before next can start the old job.
	gate.waitRejected(t)
	if m.saveBusy {
		saveAck(t, diskMailbox)()
	}
	if !firstDone || !errors.Is(firstErr, errSaveChanged) || !queuedDone || !errors.Is(queuedErr, errSaveFileOperation) || m.saveBusy || len(m.saveJobs) != 0 || !m.fileActions.busy || autoSaveDisk(t, oldPath) != first || autoSaveDisk(t, target) != copyText {
		t.Fatal("already queued original writer overlapped Save As started by completion", firstErr, queuedErr)
	}
	text(m, "newer path adoption edits😀")
	wanted := d.buffer.Text()
	gate.allow.Store(true)
	saveAck(t, gate.mailbox)()
	if !errors.Is(renamed, errSaveChanged) || d.path != target || !d.dirty() || d.saveID != 0 {
		t.Fatal("stale copy lost dirty state after excluding a queued writer", renamed)
	}
	m.autoSaveTick(now)
	saveAck(t, diskMailbox)()
	if d.dirty() || d.saveID != 1 || autoSaveDisk(t, target) != wanted || autoSaveDisk(t, oldPath) != first {
		t.Fatal("automatic writer did not save the latest renamed version")
	}
}

func TestSaveAsReceiptSurvivesRequestCancellationAndResetsBusyOnError(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "committed", true: "failed"}[failure], func(t *testing.T) {
			m := testModel(t)
			t.Cleanup(m.closeDocuments)
			saveActor(t, m)
			d := m.current()
			oldPath := d.path
			text(m, "request lifetime😀")
			wanted := d.buffer.Text()
			gate := newWorkerDispatchGate()
			stop := m.bindFileActions(context.Background(), gate.dispatch, cancelledFileChooser)
			defer stop()
			target := filepath.Join(m.workspace, "request-target.go")
			if failure {
				if err := os.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
			}
			request, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := false
			var result error
			m.fileActions.writeAs(request, d, target, func(err error) { done, result = true, err })
			gate.waitRejected(t)
			cancel() // An I/O request lifetime must not discard its UI receipt.
			gate.allow.Store(true)
			saveAck(t, gate.mailbox)()
			if !done || m.fileActions.busy {
				t.Fatal("expired request lost busy/error acknowledgement")
			}
			if failure {
				if result == nil || d.path != oldPath || !d.dirty() || d.saveID != 0 {
					t.Fatal("failed Save As discarded original dirty editor", result)
				}
			} else if result != nil || d.path != target || d.dirty() || d.saveID != 1 || autoSaveDisk(t, target) != wanted {
				t.Fatal("committed Save As receipt disappeared with expired request", result)
			}
		})
	}
}

func TestFileActionsCancelledRequestAndPermanentDispatchRejectionStop(t *testing.T) {
	m := testModel(t)
	t.Cleanup(m.closeDocuments)
	saveActor(t, m)
	d := m.current()
	text(m, "preserved dirty text")
	gate := newWorkerDispatchGate()
	stop := m.bindFileActions(context.Background(), gate.dispatch, cancelledFileChooser)
	defer stop()
	request, cancel := context.WithCancel(context.Background())
	cancel()
	cancelledTarget := filepath.Join(m.workspace, "must-not-create.go")
	var requestErr error
	m.fileActions.writeAs(request, d, cancelledTarget, func(err error) { requestErr = err })
	if !errors.Is(requestErr, context.Canceled) || m.fileActions.busy {
		stop()
		t.Fatal("already cancelled Save As request began I/O", requestErr)
	}
	if _, err := os.Stat(cancelledTarget); !os.IsNotExist(err) {
		stop()
		t.Fatal("cancelled request created a target", err)
	}
	target := filepath.Join(m.workspace, "committed-before-shutdown.go")
	wanted := d.buffer.Text()
	done := false
	m.fileActions.writeAs(context.Background(), d, target, func(error) { done = true })
	gate.waitRejected(t)
	if autoSaveDisk(t, target) != wanted {
		stop()
		t.Fatal("real Save As disk write did not complete")
	}
	stop()
	before := gate.attempts.Load()
	time.Sleep(30 * time.Millisecond)
	if gate.attempts.Load() != before || done || d.path == target || !d.dirty() || d.saveID != 0 {
		t.Fatal("shutdown retained a retry worker or applied a late Save As receipt")
	}
}
