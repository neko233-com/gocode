package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neko233-com/gocode/internal/languageserver"
	"github.com/neko233-com/gocode/internal/languageserver/lspfixture"
	workspaceSearch "github.com/neko233-com/gocode/internal/search"
)

// Every admission first rejects, then holds the genuine worker receipt for the
// test's UI thread. Rejection never invokes or owns the callback.
func rejectingWorkerMailbox() (func(func()) bool, chan func(), *atomic.Int32) {
	mailbox := make(chan func(), 16)
	var attempts, rejected atomic.Int32
	dispatch := func(fn func()) bool {
		if attempts.Add(1)%2 == 1 {
			rejected.Add(1)
			return false
		}
		mailbox <- fn
		return true
	}
	return dispatch, mailbox, &rejected
}

func TestOpenWorkerRetriesTransferAndFinalReceipt(t *testing.T) {
	m := testModel(t)
	original := m.current()
	dispatch, mailbox, rejected := rejectingWorkerMailbox()
	stop := m.startFileOpens(context.Background(), dispatch, nil)
	defer stop()
	m.open("README.md")
	held := saveAck(t, mailbox)
	if rejected.Load() != 1 || m.current() != original || !m.openBusy || len(m.docs) != 1 {
		t.Fatal("rejected/held transfer mutated UI")
	}
	held()
	final := saveAck(t, mailbox)
	if rejected.Load() != 2 || !m.openBusy || len(m.docs) != 2 {
		t.Fatal("file-open final receipt was lost or applied off UI")
	}
	final()
	if m.openBusy || m.current() == original || m.current().buffer.Text() == "" {
		t.Fatal("disk document not adopted after overload")
	}
	path := filepath.Join(m.workspace, "next.txt")
	if err := os.WriteFile(path, []byte("next 世界\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m.open(path)
	drainOpens(t, m, mailbox)
	if rejected.Load() != 4 || m.current().buffer.Text() != "next 世界\r\n" || m.openBusy {
		t.Fatal("file worker did not survive queue overload")
	}
}

func TestSearchAndHistoryWorkersRetryAndRemainAlive(t *testing.T) {
	t.Run("search", func(t *testing.T) {
		m := testModel(t)
		dispatch, mailbox, rejected := rejectingWorkerMailbox()
		stop := m.startSearch(context.Background(), dispatch, nil)
		defer stop()
		for _, query := range []string{"package", "func"} {
			m.search.query = workspaceSearch.Query{Text: query, CaseSensitive: true}
			m.search.submit()
			held := saveAck(t, mailbox)
			if !m.search.busy || len(m.search.report.Matches) != 0 {
				t.Fatal("held search receipt changed UI")
			}
			held()
			if m.search.busy || len(m.search.report.Matches) == 0 {
				t.Fatal("real search result lost after overload")
			}
		}
		if rejected.Load() != 2 {
			t.Fatal("search did not retry both jobs", rejected.Load())
		}
	})
	t.Run("history", func(t *testing.T) {
		m := testModel(t)
		dispatch, mailbox, rejected := rejectingWorkerMailbox()
		stop := m.startHistory(context.Background(), dispatch)
		defer stop()
		path := filepath.Join(m.workspace, "history-worker.txt")
		for _, text := range []string{"first 😀\r\n", "second 世界\r\n"} {
			completed := false
			m.history.submit(func(context.Context) error { return os.WriteFile(path, []byte(text), 0600) }, func(err error) {
				if err != nil {
					t.Error(err)
				}
				completed = true
			})
			held := saveAck(t, mailbox)
			raw, err := os.ReadFile(path)
			if err != nil || string(raw) != text || completed {
				t.Fatal("genuine worker write/held receipt mismatch", err)
			}
			held()
			if !completed {
				t.Fatal("history worker acknowledgement lost")
			}
		}
		if rejected.Load() != 2 {
			t.Fatal("history did not retry both jobs", rejected.Load())
		}
	})
}

func TestWorkspaceScanCancelledRequestStillAcknowledgesAfterRejection(t *testing.T) {
	m, err := newWorkspaceModel(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mailbox := make(chan func(), 1)
	rejected := make(chan struct{})
	var attempts atomic.Int32
	dispatch := func(fn func()) bool {
		if attempts.Add(1) == 1 {
			close(rejected)
			return false
		}
		mailbox <- fn
		return true
	}
	var ready bool
	var result error
	stop := m.startWorkspace(context.Background(), dispatch, nil, "", scanWorkspace, func(err error) { ready, result = true, err })
	defer stop()
	select {
	case <-rejected:
	case <-time.After(5 * time.Second):
		t.Fatal("actual disk scan did not reach rejected receipt")
	}
	m.cancelWorkspace()
	held := saveAck(t, mailbox)
	if !m.workspaceBusy || ready {
		t.Fatal("held cancelled scan mutated UI")
	}
	held()
	if m.workspaceBusy || !ready || !errors.Is(result, context.Canceled) || !strings.Contains(m.workspaceStatus, "canceled") {
		t.Fatal("request cancellation hid required UI acknowledgement", m.workspaceStatus, result)
	}
}

func TestWorkerShutdownCancelsRejectedReceipt(t *testing.T) {
	m := testModel(t)
	rejected := make(chan struct{}, 1)
	stop := m.startHistory(context.Background(), func(func()) bool {
		select {
		case rejected <- struct{}{}:
		default:
		}
		return false
	})
	m.history.submit(func(context.Context) error { return nil }, func(error) { t.Error("closed actor applied UI result") })
	select {
	case <-rejected:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not reach rejection")
	}
	start := time.Now()
	stop()
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatal("retry ignored shutdown", elapsed)
	}
}

func TestSCMWorkerRetriesRealStatusAndIndexReceipts(t *testing.T) {
	root := gitFixture(t)
	path := filepath.Join(root, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(root)
	if err != nil {
		t.Fatal(err)
	}
	defer m.closeDocuments()
	dispatch, mailbox, rejected := rejectingWorkerMailbox()
	stop := m.startSCM(context.Background(), dispatch)
	defer stop()
	status := saveAck(t, mailbox)
	if !m.scm.busy || m.scm.repository != nil {
		t.Fatal("held real Git status mutated UI")
	}
	status()
	if m.scm.busy || len(m.scm.snapshot.Entries) != 1 {
		t.Fatal("actual Git status acknowledgement lost", m.scm.status)
	}
	m.scmAction("stage", []string{"main.go"}, false)
	index := saveAck(t, mailbox)
	if !m.scm.busy || m.scm.snapshot.Entries[0].Staged() {
		t.Fatal("held index receipt changed UI")
	}
	index()
	if m.scm.busy || !m.scm.snapshot.Entries[0].Staged() || rejected.Load() != 2 {
		t.Fatal("real index write acknowledgement lost", m.scm.status)
	}
}

func TestLanguageWorkerRetriesOwnedServerLifecycleAndResult(t *testing.T) {
	m := testModel(t)
	d := m.current()
	dispatch, mailbox, rejected := rejectingWorkerMailbox()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	config := languageserver.Config{Name: "retry", Languages: []string{"go"}, Command: os.Args[0], Arguments: []string{"-test.run=^TestWorkbenchLanguageServerProcess$", "--", lspfixture.Flag, "serve"}}
	stop := m.bindLanguages(ctx, dispatch, []languageserver.Config{config})
	defer stop()
	pump := func(until func() bool) {
		t.Helper()
		for !until() {
			select {
			case fn := <-mailbox:
				fn()
			case <-ctx.Done():
				t.Fatal("owned LSP acknowledgement stalled", m.message)
			}
		}
	}
	pump(func() bool {
		return m.languageBindings[0].session != nil && len(m.diagnostics["lsp:retry\x00"+d.path]) > 0
	})
	before := rejected.Load()
	m.output = nil
	m.requestLSP(d, "textDocument/hover")
	pump(func() bool { return len(m.output) > 0 })
	if rejected.Load() <= before || strings.TrimSpace(strings.Join(m.output, "\n")) != strings.TrimSpace(d.buffer.Text()) {
		t.Fatal("real hover response did not survive overload", m.output)
	}
}

func TestLargePageWorkerReplacesRejectedDeliveryAndRetainsHeldReceipt(t *testing.T) {
	m := testModel(t)
	path := filepath.Join(m.workspace, "pages.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("row 世界😀\r\n", 128)), 0600); err != nil {
		t.Fatal(err)
	}
	d, err := openLargeDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.large.close()
	if err := d.large.file.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	mailbox := make(chan func(), 32)
	var accept atomic.Bool
	dispatch := func(fn func()) bool {
		if !accept.Load() {
			return false
		}
		mailbox <- fn
		return true
	}
	for line := range 20 {
		called := make(chan struct{})
		var once sync.Once
		rejectPage := func(fn func()) bool {
			once.Do(func() { close(called) })
			return dispatch(fn)
		}
		d.scroll = line
		m.requestLargePageWithDispatch(d, 3, rejectPage)
		select {
		case <-called:
		case <-time.After(5 * time.Second):
			t.Fatal("real page read never reached rejected delivery", line)
		}
	}
	if !d.large.loading || len(d.large.page) != 0 {
		t.Fatal("rejected page receipt changed UI")
	}
	d.large.requestCancel()
	finished := make(chan struct{})
	go func() { d.large.workers.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("superseded page retry workers remained alive while queue was full")
	}
	if len(mailbox) != 0 {
		t.Fatal("rejected page bodies unexpectedly entered UI queue")
	}
	d.scroll = 20
	latestRejected := make(chan struct{})
	var latestOnce sync.Once
	m.requestLargePageWithDispatch(d, 3, func(fn func()) bool {
		latestOnce.Do(func() { close(latestRejected) })
		return dispatch(fn)
	})
	select {
	case <-latestRejected:
	case <-time.After(5 * time.Second):
		t.Fatal("latest real page read never reached rejected delivery")
	}
	accept.Store(true)
	held := saveAck(t, mailbox)
	// Successful queueing must retain the delivery context until the UI reads
	// it; cancelling it on worker return would silently drop this valid page.
	if !d.large.loading || len(d.large.page) != 0 {
		t.Fatal("held page changed UI")
	}
	held()
	if d.large.loading || d.large.start != 20 || len(d.large.page) != 3 || d.large.err != nil {
		t.Fatal("latest real page receipt lost", d.large.start, d.large.err)
	}
	finished = make(chan struct{})
	go func() { d.large.workers.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("superseded page retry workers remained alive")
	}
	if len(mailbox) != 0 {
		t.Fatal("superseded page bodies accumulated in UI queue")
	}
	accept.Store(false)
	d.scroll = 21
	closingRejected := make(chan struct{})
	var closingOnce sync.Once
	m.requestLargePageWithDispatch(d, 3, func(fn func()) bool {
		closingOnce.Do(func() { close(closingRejected) })
		return dispatch(fn)
	})
	select {
	case <-closingRejected:
	case <-time.After(5 * time.Second):
		t.Fatal("closing page never reached rejected delivery")
	}
	closed := make(chan struct{})
	go func() { d.large.close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("large reader close waited forever on full UI queue")
	}
}
