package filewatch

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func receive(t *testing.T, mailbox chan func()) {
	t.Helper()
	select {
	case fn := <-mailbox:
		fn()
	case <-time.After(6 * time.Second):
		t.Fatal("OS file change was not delivered")
	}
}
func TestRealWatchAtomicReplacementDeletionRecreationAndPausedSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	os.WriteFile(path, []byte("initial"), 0600)
	entry := Entry{ID: 1, Path: path, Version: 1, Known: true, Hash: sha256.Sum256([]byte("initial"))}
	mailbox := make(chan func(), 1)
	var received Result
	w := Start(context.Background(), func(fn func()) bool { mailbox <- fn; return true }, func(r Result) bool { received = r; return true })
	defer w.Close()
	w.Update(State{Entries: []Entry{entry}})
	replace := func(text string) {
		t.Helper()
		temp := filepath.Join(dir, "tmp")
		if err := os.WriteFile(temp, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		if err := Replace(context.Background(), temp, path, nil); err != nil {
			t.Fatal(err)
		}
	}
	replace("atomic")
	receive(t, mailbox)
	if received.Kind != "text" || received.Text != "atomic" {
		t.Fatalf("replacement %+v", received)
	}
	os.Remove(path)
	receive(t, mailbox)
	if received.Kind != "missing" {
		t.Fatal("deletion lost", received.Kind)
	}
	replace("initial")
	receive(t, mailbox)
	if received.Kind != "text" || received.Text != "initial" {
		t.Fatal("return to original hash not delivered")
	}
	w.Update(State{Entries: []Entry{entry}, Paused: true})
	// Explicitly wait for the worker to consume Pause without relying on OS
	// notification ordering: each previous acknowledgement precedes this update.
	time.Sleep(150 * time.Millisecond)
	replace("during save")
	select {
	case fn := <-mailbox:
		fn()
		t.Fatal("paused save received disk content")
	case <-time.After(250 * time.Millisecond):
	}
	w.Update(State{Entries: []Entry{entry}})
	receive(t, mailbox)
	if received.Text != "during save" {
		t.Fatal("resume did not reconcile")
	}
}

func TestWatcherOnePendingBodyCoalescesStormAndShutdownRejectsCallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storm.txt")
	os.WriteFile(path, []byte("first"), 0600)
	mailbox := make(chan func(), 16)
	applied := 0
	w := Start(context.Background(), func(fn func()) bool { mailbox <- fn; return true }, func(Result) bool { applied++; return true })
	entry := Entry{ID: 1, Path: path, Version: 1}
	w.Update(State{Entries: []Entry{entry}})
	var held func()
	select {
	case held = <-mailbox:
	case <-time.After(5 * time.Second):
		t.Fatal("initial read")
	}
	for i := 0; i < 150; i++ {
		os.WriteFile(path, []byte(strings.Repeat("x", i+1)), 0600)
		entry.Version++
		w.Update(State{Entries: []Entry{entry}})
	}
	time.Sleep(200 * time.Millisecond)
	if len(mailbox) != 0 {
		t.Fatal("UI queue accumulated whole file bodies")
	}
	w.Close()
	held()
	if applied != 0 {
		t.Fatal("closed worker mutated UI")
	}
	if err := w.Update(State{}); err == nil {
		t.Fatal("closed watcher accepted more state")
	}
}

func TestReadPoliciesAndSubscriptionBounds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	entry := Entry{ID: 1, Path: path}
	for _, text := range []string{"\xff", "\x00", strings.Repeat("x", MaxBytes+1)} {
		os.WriteFile(path, []byte(text), 0600)
		result, _ := Read(context.Background(), entry)
		if result.Kind != "unavailable" || result.Err == nil || result.Text != "" {
			t.Fatal("unsafe/unbounded read", result.Kind)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	os.WriteFile(path, []byte("small"), 0600)
	result, _ := Read(ctx, entry)
	if result.Err == nil || result.Text != "" {
		t.Fatal("cancelled read published bytes")
	}
	w := Start(context.Background(), func(func()) bool { return false }, func(Result) bool { return true })
	defer w.Close()
	if w.Update(State{Entries: make([]Entry, MaxFiles+1)}) == nil || w.Update(State{Entries: []Entry{{Path: "relative"}}}) == nil {
		t.Fatal("subscription bounds")
	}
}
