//go:build windows && cgo

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNativeWorkbenchKeymapDrainRejectsHeldOldWriteBehindFinalBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keyboard.json")
	if err := writeKeymap(path, "vscode"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	oldStarted, releaseOld, returnOld := make(chan struct{}), make(chan struct{}), make(chan struct{})
	oldWritten := make(chan error, 1)
	var releaseOnce, returnOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseOld) }) }
	allowReturn := func() { returnOnce.Do(func() { close(returnOld) }) }
	persist, stop, finished := startSettingsWriterWithDrain(ctx, func(func()) bool { return false }, func(profile string) error {
		if profile != "jetbrains" {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return writeKeymap(path, profile)
		}
		close(oldStarted)
		<-releaseOld
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := writeKeymap(path, profile)
		oldWritten <- err
		<-returnOld
		return err
	}, func(string, error) { t.Error("stopped error callback executed") })
	t.Cleanup(func() {
		cancel()
		release()
		allowReturn()
		stop()
		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Error("owned old writer did not exit")
		}
	})
	persist("jetbrains")
	select {
	case <-oldStarted:
	case <-time.After(time.Second):
		t.Fatal("actual old write did not start")
	}
	persist("vscode")
	stop() // The original three-second stop bound expires with the old I/O held.
	select {
	case <-finished:
		t.Fatal("cancelled old OS write was falsely acknowledged as finished")
	default:
	}
	if err := verifyWindowsWorkbenchKeymap(path); err != nil {
		t.Fatal("negative control lacks the coincidental real vscode file", err)
	}
	if err := verifyWindowsWorkbenchKeymapAfterDrain(finished, path); err == nil || !strings.Contains(err.Error(), "existing 3s shutdown") {
		t.Fatal("native acceptance mistook the bounded stop for drained I/O", err)
	}
	release()
	select {
	case err := <-oldWritten:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("actual old file did not commit after its release")
	}
	profile, readErr := readKeymap(path)
	if readErr != nil || profile != "jetbrains" {
		t.Fatal("real late old write was not observed", profile, readErr)
	}
	select {
	case <-finished:
		t.Fatal("write callback had not returned, but actor completion was published")
	default:
	}
	cancel()
	allowReturn()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("released real writer never finished")
	}
	if err := verifyWindowsWorkbenchKeymapAfterDrain(finished, path); err == nil || !strings.Contains(err.Error(), `actual="jetbrains"`) {
		t.Fatal("writer completion bypassed the exact final config check", err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 || entries[0].Name() != "keyboard.json" {
		t.Fatal("timed-out writer left staging files", entries, err)
	}
}

func TestNativeWorkbenchKeymapDiskRequiresWriterDrain(t *testing.T) {
	m := testModel(t)
	t.Cleanup(m.closeDocuments)
	path := filepath.Join(t.TempDir(), "keyboard.json")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	jetbrainsDone := make(chan error, 1)
	finalStarted := make(chan struct{})
	allowFinalWrite := make(chan struct{})
	persist, stop, finished := startSettingsWriterWithDrain(ctx, func(func()) bool { return false }, func(profile string) error {
		if profile == "vscode" {
			close(finalStarted)
			select {
			case <-allowFinalWrite:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		err := writeKeymap(path, profile)
		if profile == "jetbrains" {
			jetbrainsDone <- err
		}
		return err
	}, func(string, error) { t.Error("cancelled settings error should not mutate UI") })
	m.keyboard.persist = persist
	t.Cleanup(stop)
	m.selectKeymap("jetbrains")
	select {
	case err := <-jetbrainsDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	m.selectKeymap("vscode")
	select {
	case <-finalStarted:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if m.keymapProfile() != "vscode" {
		t.Fatal("native model did not acknowledge new preset")
	}
	profile, readErr := readKeymap(path)
	if readErr != nil || profile != "jetbrains" {
		t.Fatal("negative control did not retain real old file behind correct UI", profile, readErr)
	}
	if err := verifyWindowsWorkbenchKeymap(path); err == nil || !strings.Contains(err.Error(), `actual="jetbrains"`) {
		t.Fatal("old fixture would not detect the premature final preset check", err)
	}
	stopStarted, stopDone := make(chan struct{}), make(chan struct{})
	go func() { close(stopStarted); stop(); close(stopDone) }()
	<-stopStarted
	if err := verifyWindowsWorkbenchKeymapAfterDrain(finished, path); err == nil {
		t.Fatal("held writer was accepted as finished")
	}
	select {
	case <-stopDone:
		t.Fatal("drain acknowledged a held real write")
	case <-time.After(20 * time.Millisecond):
	}
	close(allowFinalWrite)
	select {
	case <-stopDone:
	case <-ctx.Done():
		t.Fatal("existing bounded shutdown did not finish real final write", ctx.Err())
	}
	if err := verifyWindowsWorkbenchKeymapAfterDrain(finished, path); err != nil {
		t.Fatal("drained actual preset was not exact", err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 || entries[0].Name() != "keyboard.json" {
		t.Fatal("final config accumulated staging files", entries, err)
	}
}

func TestNativeWorkbenchKeymapFinalGuardRejectsDefaultMissingAndMalformedFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keyboard.json")
	if err := verifyWindowsWorkbenchKeymapAfterDrain(nil, path); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatal("missing writer acknowledgement reached the config read", err)
	}
	profile, err := readKeymap(path)
	if err != nil || profile != "vscode" {
		t.Fatal("missing-config baseline changed", profile, err)
	}
	if err = verifyWindowsWorkbenchKeymap(path); err == nil || !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), `actual="vscode"`) {
		t.Fatal("default missing profile was accepted as successful persistence", err)
	}
	for _, body := range []string{`{"keymap":"jetbrains"}`, `{"keymap":"vscode","extra":true}`, "{invalid", strings.Repeat("x", 4097)} {
		if err = os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if err = verifyWindowsWorkbenchKeymap(path); err == nil || !strings.Contains(err.Error(), "actual=") || !strings.Contains(err.Error(), "read=") {
			t.Fatal("bad real final config accepted or diagnostics missing", err)
		}
	}
	if err = writeKeymap(path, "vscode"); err != nil {
		t.Fatal(err)
	}
	if err = verifyWindowsWorkbenchKeymap(path); err != nil {
		t.Fatal(err)
	}
}
