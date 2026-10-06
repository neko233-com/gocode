package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOnlySavePreservesFileAndLeavesNoTemporaryFile(t *testing.T) {
	m := testModel(t)
	d := m.current()
	original := d.buffer.Text()
	text(m, "local edit")
	if err := os.Chmod(d.path, 0444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(d.path, 0600)
	_, err := writeDocumentSnapshot(context.Background(), d.path, d.buffer.Snapshot(), &d.diskHash)
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatal("read-only file replaced", err)
	}
	data, _ := os.ReadFile(d.path)
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(d.path), ".gocode-save-*"))
	if string(data) != original || !d.dirty() || len(leftovers) > 0 {
		t.Fatal("rejected save changed source/buffer or left a temporary file")
	}
}
