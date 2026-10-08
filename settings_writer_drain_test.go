package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSettingsWriterDrainHeldFinalRealWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keyboard.json")
	if err := writeKeymap(path, "jetbrains"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	started, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	allowWrite := func() { releaseOnce.Do(func() { close(release) }) }
	persist, stop, finished := startSettingsWriterWithDrain(ctx, func(func()) bool { return false }, func(profile string) error {
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
		return writeKeymap(path, profile)
	}, func(string, error) { t.Error("late stopped error callback executed") })
	t.Cleanup(func() { cancel(); allowWrite(); stop() })
	persist("vscode")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("final actual writer did not start")
	}
	select {
	case <-finished:
		t.Fatal("held real writer was acknowledged")
	default:
	}
	stopping, stopped := make(chan struct{}), make(chan struct{})
	go func() { close(stopping); stop(); close(stopped) }()
	<-stopping
	select {
	case <-stopped:
		t.Fatal("stop bypassed the held final real write")
	case <-time.After(20 * time.Millisecond):
	}
	allowWrite()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("released final writer did not drain")
	}
	select {
	case <-finished:
	default:
		t.Fatal("stop returned without true final writer completion")
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "{\"keymap\":\"vscode\"}\n" {
		t.Fatal("actual drained final config differs", string(body), err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	stop()
	persist("jetbrains") // Stopped producer cannot schedule another write.
	after, err := os.Stat(path)
	if err != nil || !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("idempotent stop changed persisted configuration", err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 || entries[0].Name() != "keyboard.json" {
		t.Fatal("real final writer accumulated staging files", entries, err)
	}
}

func TestKeyboardBindingDrainCancelsAdmittedErrorAndStopsPersistence(t *testing.T) {
	m := testModel(t)
	t.Cleanup(m.closeDocuments)
	root := t.TempDir()
	blockedParent := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blockedParent, []byte("unchanged\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blockedParent, "keyboard.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	admitted := make(chan func(), 1)
	stop, finished := m.bindKeyboardSettingsWithDrain(ctx, func(fn func()) bool { admitted <- fn; return true }, path)
	t.Cleanup(stop)
	m.selectKeymap("jetbrains")
	var callback func()
	select {
	case callback = <-admitted:
	case <-time.After(time.Second):
		t.Fatal("real invalid-directory write did not publish its error")
	}
	m.message = "keep native state"
	stop()
	select {
	case <-finished:
	default:
		t.Fatal("failed-I/O writer did not actually finish")
	}
	if m.keyboard.persist != nil {
		t.Fatal("closed binding still accepts persistence requests")
	}
	callback()
	if m.message != "keep native state" {
		t.Fatal("admitted error mutated state after shutdown", m.message)
	}
	stop()
	m.selectKeymap("vscode")
	body, err := os.ReadFile(blockedParent)
	if err != nil || string(body) != "unchanged\n" {
		t.Fatal("stopped binder changed its real private file", string(body), err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "not-a-directory" {
		t.Fatal("binding stop left temporary configuration", entries, err)
	}
}
