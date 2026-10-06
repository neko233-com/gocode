package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func openActor(t *testing.T, m *model, read func(context.Context, string) (*document, error)) chan func() {
	t.Helper()
	mailbox := make(chan func(), 64)
	stop := m.startFileOpens(context.Background(), func(fn func()) bool { mailbox <- fn; return true }, read)
	t.Cleanup(stop)
	return mailbox
}
func drainOpens(t *testing.T, m *model, mailbox chan func()) {
	t.Helper()
	for m.openBusy || len(m.openJobs) > 0 {
		saveAck(t, mailbox)()
	}
}

func TestOpenWorkerPreservesNewerFocusAndDirtyBuffer(t *testing.T) {
	m := testModel(t)
	original := m.current()
	mailbox := openActor(t, m, nil)
	var result *document
	var failure error
	m.openThen(context.Background(), "README.md", func(d *document, err error) { result, failure = d, err })
	ack := saveAck(t, mailbox)
	if m.current() != original || len(m.docs) != 1 || !m.openBusy {
		t.Fatal("worker mutated UI before acknowledgement")
	}
	text(m, "// unsaved 😀\n")
	version, source := original.buffer.Version(), original.buffer.Text()
	m.open(original.path)
	ack()
	drainOpens(t, m, mailbox)
	if result == nil || !errors.Is(failure, errOpenSuperseded) || m.current() != original || len(m.docs) != 2 {
		t.Fatal("late open stole newer focus", failure)
	}
	if !original.dirty() || original.buffer.Version() != version || original.buffer.Text() != source {
		t.Fatal("live unsaved buffer overwritten")
	}
}

func TestOpenWorkerSerialQueueAndAwaitedTarget(t *testing.T) {
	m := testModel(t)
	original := m.current()
	third := filepath.Join(m.workspace, "third.go")
	if err := os.WriteFile(third, []byte("package third\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var reads atomic.Int32
	mailbox := openActor(t, m, func(ctx context.Context, path string) (*document, error) {
		reads.Add(1)
		return loadDocument(ctx, path)
	})
	var latest *document
	m.open("README.md")
	ack := saveAck(t, mailbox)
	m.openThen(context.Background(), third, func(d *document, err error) {
		if err != nil {
			t.Error(err)
		}
		latest = d
	})
	ack()
	if reads.Load() != 1 || m.current() != original || latest != nil {
		t.Fatal("next worker ran before transfer/disposal acknowledgement")
	}
	drainOpens(t, m, mailbox)
	if latest == nil || m.current() != latest || latest.buffer.Text() != "package third\n" || reads.Load() != 2 {
		t.Fatal("awaited target/focus incorrect")
	}
}

func TestOpenWorkerClosedAliasDoesNotReopen(t *testing.T) {
	m := testModel(t)
	d := m.current()
	mailbox := openActor(t, m, func(ctx context.Context, _ string) (*document, error) { return loadDocument(ctx, d.path) })
	m.open("unknown-alias.go")
	ack := saveAck(t, mailbox)
	m.removeTab(m.active)
	ack()
	drainOpens(t, m, mailbox)
	if len(m.docs) != 0 || m.current() != nil {
		t.Fatal("closed file reopened by late physical-alias result")
	}
	m.open(d.path)
	drainOpens(t, m, mailbox)
	if m.current() == nil || m.current() == d {
		t.Fatal("explicit later reopen rejected or reused closed identity")
	}
}

func TestOpenWorkerCancellationAndBounds(t *testing.T) {
	m := testModel(t)
	d := m.current()
	mailbox := openActor(t, m, nil)
	m.open("README.md")
	ack := saveAck(t, mailbox)
	var calls int
	for range maxOpenJobs {
		m.openThen(context.Background(), "README.md", func(_ *document, err error) {
			if err == nil {
				t.Error("cancel accepted")
			}
			calls++
		})
	}
	var full error
	m.openThen(context.Background(), "overflow", func(_ *document, err error) { full = err })
	if full == nil || !strings.Contains(full.Error(), "queue is full") || len(m.openJobs) != maxOpenJobs {
		t.Fatal("unbounded queue", full)
	}
	m.cancelPendingOpens()
	ack()
	drainOpens(t, m, mailbox)
	if calls != maxOpenJobs || len(m.docs) != 1 || m.current() != d {
		t.Fatal("cancel retained queued result or lost original")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var failure error
	m.openThen(ctx, d.path, func(_ *document, err error) { failure = err })
	if !errors.Is(failure, context.Canceled) {
		t.Fatal("cancelled cached open focused document", failure)
	}
}

func TestOpenWorkerShutdownDisposesDroppedLargeTransfer(t *testing.T) {
	m := testModel(t)
	path := filepath.Join(m.workspace, "large.txt")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(editableFileLimit + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	mailbox := make(chan func(), 4)
	var loaded *document
	ctx, cancel := context.WithCancel(context.Background())
	stop := m.startFileOpens(ctx, func(fn func()) bool { mailbox <- fn; return true }, func(ctx context.Context, path string) (*document, error) {
		var err error
		loaded, err = loadDocument(ctx, path)
		return loaded, err
	})
	m.open(path)
	ack := saveAck(t, mailbox)
	cancel()
	stop()
	if loaded == nil || loaded.large == nil || loaded.large.ctx.Err() == nil {
		t.Fatal("dropped transfer leaked its index")
	}
	before := len(m.docs)
	ack()
	if len(m.docs) != before {
		t.Fatal("late shutdown callback adopted closed index")
	}
}

func TestExtensionPathsResolveOnWorkerAndPreserveRPCFields(t *testing.T) {
	m := testModel(t)
	raw := json.RawMessage(`{"documents":[{"path":"main.go","version":42,"edits":[]}],"reason":"atomic"}`)
	resolved, err := resolveExtensionPaths(context.Background(), m.workspace, raw)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Documents []struct {
			Path    string
			Version int
		}
		Reason string
	}
	if err := json.Unmarshal(resolved, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Documents) != 1 || got.Documents[0].Path != m.current().path || got.Documents[0].Version != 42 || got.Reason != "atomic" {
		t.Fatal("RPC identity/fields changed", string(resolved))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := resolveExtensionPaths(ctx, m.workspace, raw); !errors.Is(err, context.Canceled) {
		t.Fatal("resolution ignored cancellation", err)
	}
	for _, bad := range []string{`{`, `{"path":123}`, `{"documents":[{"path":null,"version":42}]}`} {
		if _, err := resolveExtensionPaths(context.Background(), m.workspace, json.RawMessage(bad)); err == nil {
			t.Fatal("invalid request accepted", bad)
		}
	}
}
