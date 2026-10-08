package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	workspaceSearch "github.com/neko233-com/gocode/internal/search"
	ui "github.com/neko233-com/godesktop"
)

type acceptanceNavigationFixture struct {
	m             *model
	original      *document
	observer      *searchAcceptanceNavigation
	opens, search chan func()
	match         workspaceSearch.Match
	path          string
	hash          [32]byte
	stopSearch    func()
}

func newAcceptanceNavigationFixture(t *testing.T) *acceptanceNavigationFixture {
	t.Helper()
	m := testModel(t)
	t.Cleanup(m.closeDocuments)
	f := &acceptanceNavigationFixture{m: m, original: m.current(), path: filepath.Join(m.workspace, "huge.log"), opens: make(chan func(), 64), search: make(chan func(), 64)}
	disk, err := os.Create(f.path)
	if err != nil {
		t.Fatal(err)
	}
	block := bytes.Repeat([]byte("x"), 128<<10)
	for i := 0; i < 72; i++ {
		if _, err := disk.Write(block); err != nil {
			disk.Close()
			t.Fatal(err)
		}
	}
	if _, err := disk.Write([]byte(" 😀needle\r\n")); err != nil {
		disk.Close()
		t.Fatal(err)
	}
	if err := disk.Close(); err != nil {
		t.Fatal(err)
	}
	f.hash = acceptanceNavigationHash(t, f.path)
	stopOpens := m.startFileOpens(context.Background(), func(fn func()) bool { f.opens <- fn; return true }, nil)
	t.Cleanup(stopOpens)
	f.stopSearch = m.startSearch(context.Background(), func(fn func()) bool { f.search <- fn; return true }, nil)
	t.Cleanup(f.stopSearch)
	f.observer = bindSearchAcceptanceNavigation(m, time.Now())
	m.search.query = workspaceSearch.Query{Text: "needle", Include: "huge.log", CaseSensitive: true}
	m.search.submit()
	saveAck(t, f.search)()
	if m.search.report.Err != nil || len(m.search.report.Matches) != 1 {
		t.Fatalf("actual query failed: %#v", m.search.report)
	}
	f.match = m.search.report.Matches[0]
	if f.match.Start <= editableFileLimit {
		t.Fatal("fixture does not exercise a real large file")
	}
	return f
}

func acceptanceNavigationHash(t *testing.T, path string) [32]byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.CopyBuffer(h, f, make([]byte, 128<<10)); err != nil {
		t.Fatal(err)
	}
	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result
}

func (f *acceptanceNavigationFixture) begin(t *testing.T) {
	t.Helper()
	f.observer.arm(f.m, f.path, f.match.Start)
	f.m.activateSearchResult(nil, 0)
	if f.m.openBusy {
		saveAck(t, f.opens)() // Real loaded-document transfer.
		saveAck(t, f.opens)() // Real loader disposal/busy acknowledgement.
	}
}

func (f *acceptanceNavigationFixture) checkFreshNavigation(t *testing.T) {
	t.Helper()
	f.begin(t)
	saveAck(t, f.search)()
	d := f.m.current()
	if d == nil || d.large == nil || !d.large.byteMode || d.large.byteOffset != f.match.Start || !f.observer.current.verified {
		t.Fatal("fresh actual Verify receipt did not navigate")
	}
	f.m.requestLargePageWithDispatch(d, 10, func(fn func()) bool { f.search <- fn; return true })
	saveAck(t, f.search)()
	if err := f.observer.failure(f.m); err != nil || d.large.err != nil || d.large.loading || !strings.Contains(d.large.window.Text, "needle") {
		t.Fatalf("fresh actual byte page: observation=%v page=%v", err, d.large.err)
	}
	if acceptanceNavigationHash(t, f.path) != f.hash {
		t.Fatal("navigation wrote actual source")
	}
}

func TestSearchAcceptanceNavigationReportsRealOpenError(t *testing.T) {
	f := newAcceptanceNavigationFixture(t)
	if err := os.Remove(f.path); err != nil {
		t.Fatal(err)
	}
	f.begin(t)
	if err := f.observer.failure(f.m); !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "open failed") {
		t.Fatalf("real current opening failure was not reported: %v", err)
	}
	if f.m.openBusy || f.m.current() != f.original || f.observer.current.verifyQueued {
		t.Fatal("failed open changed focus or started Verify")
	}
}

func TestSearchAcceptanceNavigationForwardsRealWorkAndReceiptsOnce(t *testing.T) {
	f := newAcceptanceNavigationFixture(t)
	open, validate := f.m.requestOpen, f.m.search.validate
	openCalls, openReceipts, validations, verifyReceipts := 0, 0, 0, 0
	var workCalls atomic.Uint32
	f.m.requestOpen = func(ctx context.Context, path string, done func(*document, error)) {
		openCalls++
		open(ctx, path, func(d *document, err error) {
			openReceipts++
			done(d, err)
		})
	}
	f.m.search.validate = func(work func(context.Context) error, done func(error)) {
		validations++
		validate(func(ctx context.Context) error {
			workCalls.Add(1)
			return work(ctx)
		}, func(err error) {
			verifyReceipts++
			done(err)
		})
	}
	f.observer = bindSearchAcceptanceNavigation(f.m, time.Now())
	f.begin(t)
	held := saveAck(t, f.search)
	if openCalls != 1 || openReceipts != 1 || validations != 1 || workCalls.Load() != 1 || verifyReceipts != 0 {
		t.Fatalf("real call boundaries before held receipt: open=%d/%d Verify=%d/%d/%d", openCalls, openReceipts, validations, workCalls.Load(), verifyReceipts)
	}
	held()
	if verifyReceipts != 1 || !f.m.current().large.byteMode || f.observer.failure(f.m) != nil || acceptanceNavigationHash(t, f.path) != f.hash {
		t.Fatal("actual work/result was not forwarded exactly once")
	}
}

func TestSearchAcceptanceNavigationReportsOriginal30sVerifyDeadline(t *testing.T) {
	f := newAcceptanceNavigationFixture(t)
	validate := f.m.search.validate
	type actualDeadline struct{ queued, deadline time.Time }
	began := make(chan actualDeadline, 1)
	f.m.search.validate = func(work func(context.Context) error, done func(error)) {
		queued := time.Now()
		validate(func(ctx context.Context) error {
			deadline, ok := ctx.Deadline()
			if !ok {
				return errors.New("original search actor supplied no deadline")
			}
			began <- actualDeadline{queued, deadline}
			<-ctx.Done()     // Exercise the unchanged actor's real 30-second budget.
			return work(ctx) // Actual file/open/Verify closure, no fabricated response.
		}, done)
	}
	f.begin(t)
	var timing actualDeadline
	select {
	case timing = <-began:
	case <-time.After(5 * time.Second):
		t.Fatal("actual Verify work did not start")
	}
	if budget := timing.deadline.Sub(timing.queued); budget < 29900*time.Millisecond || budget > 30100*time.Millisecond {
		t.Fatalf("original request budget changed: %s", budget)
	}
	select {
	case receipt := <-f.search:
		receipt()
	case <-time.After(35 * time.Second):
		t.Fatal("original deadline did not produce a receipt")
	}
	if err := f.observer.failure(f.m); !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "Verify failed") {
		t.Fatalf("actual original Verify deadline was not reported: %v", err)
	}
	if f.m.current().large.byteMode || acceptanceNavigationHash(t, f.path) != f.hash {
		t.Fatal("timed-out verification navigated or wrote source")
	}
	t.Logf("actual unchanged actor budget=%s; original Verify deadline receipt=%s; failure reports the actual error", timing.deadline.Sub(timing.queued), time.Since(timing.queued))
}

func TestSearchAcceptanceNavigationDistinguishesLateFocusAndQuery(t *testing.T) {
	for _, cause := range []string{"focus", "query"} {
		t.Run(cause, func(t *testing.T) {
			f := newAcceptanceNavigationFixture(t)
			f.begin(t)
			held := saveAck(t, f.search) // Successful real-file Verify, before UI receipt.
			large := f.m.current()
			selection := f.original.buffer.Selection()
			if cause == "focus" {
				f.m.focusTab(f.original)
			} else {
				f.m.showSearch(nil)
				f.m.searchInput(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'A', Modifiers: ui.ModifierControl})
				f.m.searchInput(nil, ui.InputEvent{Kind: ui.Character, Key: 'x'})
			}
			held()
			if err := f.observer.failure(f.m); err == nil || !strings.Contains(err.Error(), "superseded") {
				t.Fatalf("actual %s change was not diagnosed as superseded: %v", cause, err)
			}
			if large.large.byteMode || f.original.buffer.Selection() != selection {
				t.Fatal("obsolete receipt moved the old view or selection")
			}
			if cause == "query" {
				f.m.search.query = workspaceSearch.Query{Text: "needle", Include: "huge.log", CaseSensitive: true}
				f.m.search.submit()
				saveAck(t, f.search)()
			}
			f.checkFreshNavigation(t)
		})
	}
}

func TestSearchAcceptanceNavigationOldMessageAndPageErrorAreNotCurrentFailure(t *testing.T) {
	f := newAcceptanceNavigationFixture(t)
	f.m.message = "Search result is stale: earlier expected negative"
	f.begin(t)
	held := saveAck(t, f.search)
	if err := f.observer.failure(f.m); err != nil {
		t.Fatalf("old phase9 message was treated as this request's failure: %v", err)
	}
	held()
	d := f.m.current()
	info, err := os.Stat(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(f.path, info.ModTime(), info.ModTime().Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	f.m.requestLargePageWithDispatch(d, 10, func(fn func()) bool { f.search <- fn; return true })
	saveAck(t, f.search)()
	if d.large.err == nil || f.observer.failure(f.m) == nil {
		t.Fatal("actual changed-file byte-page error was not reported")
	}
	if err := os.Chtimes(f.path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	f.m.navigateLarge(d, 0, f.match.Start, true)
	if err := f.observer.failure(f.m); err != nil {
		t.Fatal("previous error was reported before the replacement page request", err)
	}
	f.m.requestLargePageWithDispatch(d, 10, func(fn func()) bool { f.search <- fn; return true })
	if err := f.observer.failure(f.m); err != nil {
		t.Fatal("previous actual page error was reported while replacement was loading", err)
	}
	saveAck(t, f.search)()
	if err := f.observer.failure(f.m); err != nil || d.large.err != nil || !strings.Contains(d.large.window.Text, "needle") {
		t.Fatalf("real replacement page did not clear the old failure: %v/%v", err, d.large.err)
	}
	if acceptanceNavigationHash(t, f.path) != f.hash {
		t.Fatal("page checks changed source bytes")
	}
}

func TestSearchAcceptanceNavigationShutdownAndRearmedReceipts(t *testing.T) {
	for _, cause := range []string{"shutdown", "rearm"} {
		t.Run(cause, func(t *testing.T) {
			f := newAcceptanceNavigationFixture(t)
			f.begin(t)
			held := saveAck(t, f.search)
			old := f.observer.current
			if cause == "shutdown" {
				f.stopSearch()
				held()
				if old.verified || f.m.current().large.byteMode {
					t.Fatal("admitted late receipt mutated after actual actor shutdown")
				}
			} else {
				f.begin(t)
				newer := f.observer.current
				held()
				if newer == old || newer.verified || f.m.current().large.byteMode {
					t.Fatal("old receipt contaminated the rearmed request")
				}
				saveAck(t, f.search)()
				if err := f.observer.failure(f.m); err != nil || !newer.verified || !f.m.current().large.byteMode {
					t.Fatalf("fresh rearmed receipt failed: %v", err)
				}
			}
		})
	}
}

func TestSearchAcceptanceSnapshotIsDetachedFromLiveState(t *testing.T) {
	f := newAcceptanceNavigationFixture(t)
	f.begin(t)
	saveAck(t, f.search)()
	snapshot := snapshotSearchAcceptance(f.m, f.observer, time.Now(), 12, false, 20, 19, 3, 18)
	before, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 0; i < 100; i++ {
			data, err := json.Marshal(snapshot)
			if err != nil || !bytes.Equal(data, before) {
				t.Error("published snapshot changed with live state")
				return
			}
		}
	})
	f.m.message, f.m.search.status = "new live message", "new live status"
	f.observer.current = nil
	f.m.current().large.byteOffset++
	wg.Wait()
}

func TestSearchAcceptanceWatchdogPublicationAndFinalHashDeadline(t *testing.T) {
	for i := 0; i < 1000; i++ {
		var active atomic.Pointer[ui.Context]
		var expired atomic.Bool
		cx := new(ui.Context) // Opaque identity only; no native Context method runs.
		ready := make(chan struct{})
		observed := make(chan bool, 1)
		go func() {
			<-ready
			expired.Store(true)
			observed <- active.Load() == cx
		}()
		close(ready)
		publisherMustQuit := publishSearchAcceptanceContext(&active, &expired, cx)
		if !publisherMustQuit && !<-observed {
			t.Fatal("timer/publisher race lost the real Quit obligation")
		}
		if publisherMustQuit {
			<-observed
		}
	}
	var active atomic.Pointer[ui.Context]
	var expired atomic.Bool
	expired.Store(true)
	if !publishSearchAcceptanceContext(&active, &expired, new(ui.Context)) {
		t.Fatal("timeout before first View publication was lost")
	}
	started := time.Now()
	for _, elapsed := range []time.Duration{90 * time.Second, 90*time.Second + time.Nanosecond, 120 * time.Second} {
		if !searchAcceptanceExpired(started, started.Add(elapsed), false) {
			t.Fatal("late hash allowed PASS with a delayed timer", elapsed)
		}
	}
	if searchAcceptanceExpired(started, started.Add(90*time.Second-time.Nanosecond), false) || !searchAcceptanceExpired(started, started, true) {
		t.Fatal("original90s/timer bound changed")
	}
}
