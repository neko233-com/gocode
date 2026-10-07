package main

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExtensionManagerRealSettingsRetryReceiptAndPreserveFocus(t *testing.T) {
	m := testModel(t)
	t.Cleanup(m.closeDocuments)
	root := t.TempDir()
	e := fixtureExtension(t, root)
	gate := newWorkerDispatchGate()
	stop := m.bindExtensionManager(context.Background(), gate.dispatch, root)
	defer stop()
	m.extensionsView.manage("disable", e.ID())
	gate.waitRejected(t)
	disk, err := readExtensionSettings(root)
	if err != nil || !containsExtension(disk.Disabled, e.ID()) || !m.extensionsView.busy || containsExtension(m.extensionsView.settings.Disabled, e.ID()) {
		t.Fatal("extension settings did not keep the real disk/UI boundary", disk, err)
	}
	m.palette, m.activity, m.editing = true, "search", false
	gate.allow.Store(true)
	saveAck(t, gate.mailbox)()
	if m.extensionsView.busy || !m.extensionsView.reload || !containsExtension(m.extensionsView.settings.Disabled, e.ID()) || len(m.extensionsView.contributions[e.ID()]) != 1 || !m.palette || m.activity != "search" || m.editing {
		t.Fatal("extension receipt was lost or stole newer workbench focus")
	}
	m.extensionsView.manage("enable", e.ID())
	saveAck(t, gate.mailbox)()
	disk, err = readExtensionSettings(root)
	if err != nil || m.extensionsView.busy || containsExtension(disk.Disabled, e.ID()) || containsExtension(m.extensionsView.settings.Disabled, e.ID()) {
		t.Fatal("worker remained busy after recovered UI receipt", err)
	}
	leftovers, err := filepath.Glob(filepath.Join(root, ".state-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatal("extension settings retry left temporary files", leftovers, err)
	}
}

func TestExtensionManagerRealVSIXInstallRetriesRejectedReceipt(t *testing.T) {
	m := testModel(t)
	t.Cleanup(m.closeDocuments)
	prepared := fixtureExtension(t, t.TempDir())
	// Repackage the existing real fixture payload, then run the production
	// manager's Install path into a separate empty extension store.
	vsix := filepath.Join(t.TempDir(), "real-managed.vsix")
	f, err := os.Create(vsix)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for _, name := range []string{"package.json", "extension.cjs"} {
		body, readErr := os.ReadFile(filepath.Join(prepared.Path, name))
		if readErr != nil {
			z.Close()
			f.Close()
			t.Fatal(readErr)
		}
		part, writeErr := z.Create("extension/" + name)
		if writeErr == nil {
			_, writeErr = part.Write(body)
		}
		if writeErr != nil {
			z.Close()
			f.Close()
			t.Fatal(writeErr)
		}
	}
	if err := errors.Join(z.Close(), f.Close()); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	gate := newWorkerDispatchGate()
	stop := m.bindExtensionManager(context.Background(), gate.dispatch, root)
	defer stop()
	m.extensionsView.manage("install", vsix)
	gate.waitRejected(t)
	if _, err := os.Stat(filepath.Join(root, prepared.ID()+"-"+prepared.Manifest.Version, "package.json")); err != nil || !m.extensionsView.busy || len(m.installed) != 0 {
		t.Fatal("real installed VSIX did not await UI ownership transfer", err)
	}
	gate.allow.Store(true)
	saveAck(t, gate.mailbox)()
	found := false
	for _, info := range m.installed {
		found = found || info.ID == prepared.ID()
	}
	if !found || m.extensionsView.busy || len(m.extensionsView.contributions[prepared.ID()]) != 1 {
		t.Fatal("installed VSIX UI receipt was lost")
	}
}

func TestExtensionManagerErrorsResetBusyAndPermanentRejectionCancels(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		m := testModel(t)
		t.Cleanup(m.closeDocuments)
		gate := newWorkerDispatchGate()
		stop := m.bindExtensionManager(context.Background(), gate.dispatch, t.TempDir())
		defer stop()
		m.extensionsView.manage("install", filepath.Join(t.TempDir(), "missing.vsix"))
		gate.waitRejected(t)
		gate.allow.Store(true)
		saveAck(t, gate.mailbox)()
		if m.extensionsView.busy || m.message == "" || m.extensionsView.reload || len(m.installed) != 0 {
			t.Fatal("failed real VSIX install lost its busy/error receipt")
		}
	})
	t.Run("shutdown", func(t *testing.T) {
		m := testModel(t)
		t.Cleanup(m.closeDocuments)
		root := t.TempDir()
		e := fixtureExtension(t, root)
		gate := newWorkerDispatchGate()
		stop := m.bindExtensionManager(context.Background(), gate.dispatch, root)
		defer stop()
		m.extensionsView.manage("disable", e.ID())
		gate.waitRejected(t)
		stop()
		before := gate.attempts.Load()
		time.Sleep(30 * time.Millisecond)
		if gate.attempts.Load() != before || containsExtension(m.extensionsView.settings.Disabled, e.ID()) || m.extensionsView.reload {
			t.Fatal("cancelled extension receipt retained worker or applied UI state")
		}
		disk, err := readExtensionSettings(root)
		if err != nil || !containsExtension(disk.Disabled, e.ID()) {
			t.Fatal("shutdown lost the already committed extension settings", err)
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".state-") {
				t.Fatal("cancelled extension operation left a temporary file", entry.Name())
			}
		}
	})
}
