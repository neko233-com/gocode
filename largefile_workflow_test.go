package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ui "github.com/neko233-com/godesktop"
)

func TestLargeDocumentPolicyNavigationAndClose(t *testing.T) {
	m := testModel(t)
	defer m.closeDocuments()
	path := filepath.Join(m.workspace, "large.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("line\n", editableFileLimit/5+1)), 0600); err != nil {
		t.Fatal(err)
	}
	m.open(path)
	d := m.current()
	if d == nil || d.large == nil || d.buffer != nil {
		t.Fatal("large document did not use a file-backed reader")
	}
	if err := d.large.file.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	d.large.stats = d.large.file.Stats()
	if stateOf(d) != nil {
		t.Fatal("large file sent to extension host")
	}
	m.moveCursor(d, 42, 0, false)
	if d.scroll != 42 {
		t.Fatal("line navigation")
	}
	m.goTo(":1048576")
	if !d.large.byteMode || d.large.byteOffset != 1048576 {
		t.Fatal("byte navigation")
	}
	m.goTo("11")
	if d.large.byteMode || d.scroll != 10 {
		t.Fatal("return to lines")
	}
	m.input(nil, ui.InputEvent{Kind: ui.Character, Key: 'x'})
	if d.dirty() || !strings.Contains(m.message, "read-only") {
		t.Fatal("large-file edit was accepted")
	}
	if err := m.save(); err == nil {
		t.Fatal("large-file overwrite allowed")
	}
	m.attachChatContext(false)
	if m.chatContext != "" {
		t.Fatal("huge file attached to chat")
	}
	reader := d.large.file
	m.closeTab(m.active)
	// The asynchronous close is cancellable before the OS file handle is closed.
	if _, err := reader.Bytes(context.Background(), 0, 4); err == nil {
		t.Fatal("closed viewer still readable")
	}
}
