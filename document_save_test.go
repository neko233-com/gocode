package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCancelledAndExternalChangeSavePreserveDiskAndBuffer(t *testing.T) {
	m := testModel(t)
	d := m.current()
	original, err := os.ReadFile(d.path)
	if err != nil {
		t.Fatal(err)
	}
	text(m, "native edit 😀")
	snapshot := d.buffer.Snapshot()
	version := d.buffer.Version()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := writeDocumentSnapshot(ctx, d.path, snapshot, &d.diskHash); err == nil {
		t.Fatal("cancelled save committed")
	}
	if data, _ := os.ReadFile(d.path); string(data) != string(original) {
		t.Fatal("cancelled save changed source")
	}
	external := append(append([]byte{}, original...), []byte("// external editor\n")...)
	if err := os.WriteFile(d.path, external, 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.save(); err == nil || !strings.Contains(err.Error(), "changed on disk") {
		t.Fatal("external changes overwritten", err)
	}
	if data, _ := os.ReadFile(d.path); string(data) != string(external) {
		t.Fatal("external source lost")
	}
	if d.buffer.Version() != version || !d.buffer.Dirty() || d.buffer.Text() != snapshot.Text() {
		t.Fatal("unsaved buffer lost")
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(d.path), ".gocode-save-*"))
	if len(leftovers) != 0 {
		t.Fatal("failed save left temporary files", leftovers)
	}
}
func TestCloseDeferralCancellationAndImmutableSavePlans(t *testing.T) {
	m := testModel(t)
	d := m.current()
	if !m.requestWindowClose(nil) {
		t.Fatal("clean window blocked")
	}
	text(m, "unsaved")
	version := d.buffer.Version()
	value := d.buffer.Text()
	editing := m.editing
	if m.requestWindowClose(nil) || !m.closePrompt || m.closeTarget != nil {
		t.Fatal("dirty window not deferred")
	}
	plans := m.closePlans()
	if len(plans) != 1 || plans[0].snapshot.Text() != value {
		t.Fatal("close snapshot mismatch")
	}
	m.cancelClose()
	if m.closePrompt || d.buffer.Version() != version || d.buffer.Text() != value || !d.buffer.Dirty() || m.editing != editing {
		t.Fatal("cancel lost data/focus")
	}
	text(m, "newer")
	if plans[0].snapshot.Text() != value {
		t.Fatal("worker snapshot changed with subsequent edits")
	}
	hash, err := writeDocumentSnapshot(context.Background(), d.path, plans[0].snapshot, plans[0].expected)
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(d.path); string(data) != value {
		t.Fatal("snapshot save mismatch")
	}
	if d.buffer.Version() == plans[0].snapshot.Version || !d.buffer.Dirty() {
		t.Fatal("newer version marked saved")
	}
	d.diskHash = hash
	d.diskKnown = true
	if err := m.save(); err != nil {
		t.Fatal("newer save failed", err)
	}
	if d.buffer.Dirty() {
		t.Fatal("newer save not acknowledged")
	}
	m.beginClose(d)
	m.finishClose(nil)
	if m.current() != nil {
		t.Fatal("confirmed tab did not close")
	}
}
