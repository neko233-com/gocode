package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type settingsWorkerFixture struct {
	model         *model
	path          string
	prefix        string
	selectValue   func(int)
	readLatest    func() error
	persistActive func() bool
	stop          func()
}

func realSettingsWorker(t *testing.T, kind string, dispatch func(func()) bool, path string) settingsWorkerFixture {
	t.Helper()
	m := testModel(t)
	t.Cleanup(m.closeDocuments)
	fixture := settingsWorkerFixture{model: m, path: path}
	switch kind {
	case "auto-save":
		values := []autoSaveConfig{{Mode: "afterDelay", DelayMS: 100}, {Mode: "onWindowChange", DelayMS: 2500}}
		fixture.prefix = "Auto Save settings: "
		fixture.selectValue = func(index int) { m.configureAutoSave(values[index]) }
		fixture.readLatest = func() error {
			value, err := readAutoSaveConfig(path)
			if err == nil && value != values[1] {
				err = fmt.Errorf("saved Auto Save configuration is %v", value)
			}
			return err
		}
		fixture.persistActive = func() bool { return m.autoSave.persist != nil }
		fixture.stop = m.startAutoSaveSettings(context.Background(), dispatch, path)
	case "keyboard":
		values := []string{"jetbrains", "vscode"}
		fixture.prefix = "Keyboard shortcuts: "
		fixture.selectValue = func(index int) { m.selectKeymap(values[index]) }
		fixture.readLatest = func() error {
			value, err := readKeymap(path)
			if err == nil && value != values[1] {
				err = fmt.Errorf("saved keyboard preset is %q", value)
			}
			if err == nil {
				_, err = os.Stat(path) // Missing config defaults to VS Code, not proof of a write.
			}
			return err
		}
		fixture.persistActive = func() bool { return m.keyboard.persist != nil }
		fixture.stop = m.bindKeyboardSettings(context.Background(), dispatch, path)
	default:
		t.Fatal("unknown settings worker", kind)
	}
	t.Cleanup(fixture.stop)
	return fixture
}

func malformedSettingsParent(t *testing.T) (string, string) {
	t.Helper()
	parent := filepath.Join(t.TempDir(), "gocode")
	if err := os.WriteFile(parent, []byte("owned file blocks the config directory"), 0600); err != nil {
		t.Fatal(err)
	}
	return parent, filepath.Join(parent, "settings.json")
}

func repairSettingsParent(t *testing.T, parent string) {
	t.Helper()
	if err := os.Remove(parent); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
}

func waitForSettingsDisk(t *testing.T, fixture settingsWorkerFixture) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := fixture.readLatest(); err == nil {
			return
		} else if time.Now().After(deadline) {
			t.Fatal("settings writer failed to persist latest value behind rejected error receipt", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func assertOnlyFinalSettingsFile(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatal("settings writer left temporary or duplicate configuration files", entries, err)
	}
}

func TestSettingsWorkersRetryLatestRealWriteFailure(t *testing.T) {
	for _, kind := range []string{"auto-save", "keyboard"} {
		t.Run(kind, func(t *testing.T) {
			parent, path := malformedSettingsParent(t)
			gate := newWorkerDispatchGate()
			fixture := realSettingsWorker(t, kind, gate.dispatch, path)
			fixture.selectValue(0)
			gate.waitRejected(t)
			before := fixture.model.message
			if strings.HasPrefix(before, fixture.prefix) {
				t.Fatal("settings error changed UI before receipt acceptance")
			}
			fixture.selectValue(1) // Old retry must yield to this newer real failure.
			gate.allow.Store(true)
			for callbacks := 0; !strings.HasPrefix(fixture.model.message, fixture.prefix); callbacks++ {
				if callbacks == 2 {
					t.Fatal("latest configuration failure lost under rejected admission")
				}
				saveAck(t, gate.mailbox)()
			}
			if !fixture.persistActive() || gate.attempts.Load() < 2 {
				t.Fatal("real settings failure permanently stopped its persistence actor")
			}
			if value, err := os.ReadFile(parent); err != nil || string(value) != "owned file blocks the config directory" {
				t.Fatal("failed config write modified its blocking file", err)
			}
			if _, err := os.Stat(path); err == nil {
				t.Fatal("malformed config directory unexpectedly accepted a file")
			}
		})
	}
}

func TestSettingsWorkersBlockedErrorDoesNotDelayLatestDiskOrShutdown(t *testing.T) {
	for _, kind := range []string{"auto-save", "keyboard"} {
		t.Run(kind, func(t *testing.T) {
			parent, path := malformedSettingsParent(t)
			gate := newWorkerDispatchGate()
			goroutines := runtime.NumGoroutine()
			fixture := realSettingsWorker(t, kind, gate.dispatch, path)
			fixture.selectValue(0)
			gate.waitRejected(t)
			repairSettingsParent(t, parent)
			for index := range 512 {
				fixture.selectValue(index % 2)
			}
			fixture.selectValue(1)
			waitForSettingsDisk(t, fixture) // UI admission remains rejected throughout.
			if len(gate.mailbox) != 0 || runtime.NumGoroutine() > goroutines+8 {
				t.Fatal("settings selections accumulated UI callbacks or unbounded workers")
			}
			fixture.model.message = "latest settings saved; obsolete errors must stay quiet"
			started := time.Now()
			fixture.stop()
			if elapsed := time.Since(started); elapsed >= 2500*time.Millisecond {
				t.Fatal("rejected obsolete error delayed latest-settings shutdown drain", elapsed)
			}
			before := gate.attempts.Load()
			gate.allow.Store(true)
			time.Sleep(30 * time.Millisecond)
			if fixture.persistActive() || gate.attempts.Load() != before || len(gate.mailbox) != 0 || fixture.model.message != "latest settings saved; obsolete errors must stay quiet" || fixture.readLatest() != nil {
				t.Fatal("obsolete config error survived success/shutdown or latest file was lost")
			}
			assertOnlyFinalSettingsFile(t, path)
		})
	}
}

func TestSettingsWorkersAdmittedFailureCannotMutateAfterSupersessionOrStop(t *testing.T) {
	for _, kind := range []string{"auto-save", "keyboard"} {
		for _, transition := range []string{"new-success", "stop"} {
			t.Run(kind+"/"+transition, func(t *testing.T) {
				parent, path := malformedSettingsParent(t)
				gate := newWorkerDispatchGate()
				gate.allow.Store(true)
				fixture := realSettingsWorker(t, kind, gate.dispatch, path)
				fixture.selectValue(0)
				callback := saveAck(t, gate.mailbox) // Accepted does not mean executed.
				if transition == "new-success" {
					repairSettingsParent(t, parent)
					fixture.selectValue(1)
					waitForSettingsDisk(t, fixture)
				} else {
					fixture.stop()
				}
				fixture.model.message = "new UI state"
				callback()
				if fixture.model.message != "new UI state" {
					t.Fatal("admitted obsolete failure mutated current or closed UI", fixture.model.message)
				}
				fixture.stop() // Shutdown is idempotent and never applies a delayed receipt.
				if transition == "new-success" {
					if err := fixture.readLatest(); err != nil {
						t.Fatal("settings success disappeared while cancelling its old failure", err)
					}
					assertOnlyFinalSettingsFile(t, path)
				} else if value, err := os.ReadFile(parent); err != nil || string(value) != "owned file blocks the config directory" {
					t.Fatal("shutdown modified the malformed settings parent", err)
				}
			})
		}
	}
}
