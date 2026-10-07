package filewatch

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestRealWatcherRetriesRejectedReceiptAndKeepsWatching(t *testing.T) {
	path := filepath.Join(t.TempDir(), "retry.txt")
	if err := os.WriteFile(path, []byte("first 😀\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	mailbox := make(chan func(), 1)
	var attempts atomic.Int32
	var received Result
	applied := 0
	w := Start(context.Background(), func(fn func()) bool {
		if attempts.Add(1)%2 == 1 {
			return false
		}
		mailbox <- fn
		return true
	}, func(result Result) bool { applied++; received = result; return true })
	defer w.Close()
	if err := w.Update(State{Entries: []Entry{{ID: 1, Path: path, Version: 1}}}); err != nil {
		t.Fatal(err)
	}
	var held func()
	select {
	case held = <-mailbox:
	case <-time.After(6 * time.Second):
		t.Fatal("rejected initial disk receipt was lost")
	}
	if attempts.Load() != 2 || applied != 0 {
		t.Fatal("held worker receipt changed UI")
	}
	held()
	if received.Text != "first 😀\r\n" || applied != 1 {
		t.Fatal("initial real bytes lost", received)
	}
	if err := os.WriteFile(path, []byte("second 世界\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	receive(t, mailbox)
	if received.Text != "second 世界\r\n" || applied != 2 || attempts.Load() != 4 {
		t.Fatal("watcher stopped after overload", received, attempts.Load())
	}
}
