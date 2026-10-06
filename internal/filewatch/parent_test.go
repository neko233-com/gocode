package filewatch

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReconcileRecreatedParentDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "parent")
	os.Mkdir(dir, 0700)
	path := filepath.Join(dir, "file.txt")
	os.WriteFile(path, []byte("original"), 0600)
	entry := Entry{ID: 1, Path: path, Version: 1, Known: true, Hash: sha256.Sum256([]byte("original"))}
	mailbox := make(chan func(), 1)
	received := ""
	w := Start(context.Background(), func(fn func()) bool { mailbox <- fn; return true }, func(r Result) bool { received = r.Text; return true })
	defer w.Close()
	w.Update(State{Entries: []Entry{entry}})
	time.Sleep(150 * time.Millisecond)
	if err := os.Rename(dir, dir+"-previous"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("recreated"), 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(6 * time.Second)
	for received != "recreated" {
		select {
		case fn := <-mailbox:
			fn()
		case <-deadline:
			t.Fatal("recreated parent not reconciled")
		}
	}
	if err := os.WriteFile(path, []byte("after recreation"), 0600); err != nil {
		t.Fatal(err)
	}
	receive(t, mailbox)
	if received != "after recreation" {
		t.Fatal("recreated parent did not continue observing changes")
	}
}
