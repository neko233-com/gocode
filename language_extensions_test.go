package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gocode/internal/languageextension"
	"github.com/neko233-com/gocode/internal/languageserver"
	"github.com/neko233-com/gocode/internal/languageserver/lspfixture"
	textbuffer "github.com/neko233-com/godesktop/editor"
	"github.com/neko233-com/godesktop/extensions"
)

func TestLanguageExtensionInventoryAndExplicitConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	goExtension := extensions.Extension{Manifest: extensions.Manifest{Publisher: "golang", Name: "go", Version: "0.50.0"}}
	goExtension.Manifest.Contributes.Commands = []extensions.Command{{Command: "go.test.package", Title: "Go Test"}}
	ordinary := extensions.Extension{Manifest: extensions.Manifest{Publisher: "fixture", Name: "command", Version: "1.0.0"}}
	ordinary.Manifest.Contributes.Commands = []extensions.Command{{Command: "fixture.run", Title: "Run"}}
	installed := []extensions.Extension{goExtension, ordinary}
	if got := languageHostExtensions(installed); len(got) != 1 || got[0].ID() != ordinary.ID() {
		t.Fatal("recognized JS was advertised", got)
	}
	if got := extensionContributions(installed); len(got[goExtension.ID()]) != 0 || len(got[ordinary.ID()]) != 1 {
		t.Fatal("unsupported manifest command callback advertised", got)
	}
	state := extensionSettings{Disabled: []string{goExtension.ID()}}
	configs, issues, err := loadWorkbenchLanguageConfigs("", root, installed, state)
	if err != nil || len(configs) != 0 || len(issues) != 0 {
		t.Fatal("disabled adapter resurrected implicit Go server", configs, issues, err)
	}
	file := filepath.Join(root, "independent.json")
	data, _ := json.Marshal([]languageserver.Config{{Name: "user-go", Command: os.Args[0], Languages: []string{"go"}}})
	if err = os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	configs, issues, err = loadWorkbenchLanguageConfigs(file, root, installed, state)
	if err != nil || len(configs) != 1 || configs[0].Name != "user-go" || len(issues) != 0 {
		t.Fatal("explicit independent config disabled", configs, issues, err)
	}
	if err = os.MkdirAll(filepath.Join(root, ".gocode-language"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, ".gocode-language", "go.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	configs, _, err = loadWorkbenchLanguageConfigs("", root, nil, extensionSettings{})
	if err != nil || len(configs) != 0 {
		t.Fatal("uninstalled adapter resurrected implicit Go fallback", configs, err)
	}
}

func TestLanguageAdapterReconcileOnceAndRejectRemovedReceipts(t *testing.T) {
	m := testModel(t)
	d := m.current()
	m.diagnostics = map[string][]diagnostic{}
	d.buffer, _ = textbuffer.New("package main\n// real helper 😀\n")
	mailbox := make(chan func(), 32)
	dispatch := func(fn func()) bool { mailbox <- fn; return true }
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	count := 0
	m.onDocument = func(string, *document, textbuffer.ChangeEvent) { count++ }
	shutdown := m.bindLanguages(ctx, dispatch, nil)
	defer shutdown()
	config := languageserver.Config{Name: "extension:golang.go", Languages: []string{"go"}, Command: os.Args[0], Arguments: []string{"-test.run=^TestWorkbenchLanguageServerProcess$", "--", lspfixture.Flag, "serve"}}
	pump := func(until func() bool) {
		t.Helper()
		for !until() {
			select {
			case fn := <-mailbox:
				fn()
			case <-ctx.Done():
				t.Fatal("adapter lifecycle stalled", m.lspStatus)
			}
		}
	}
	for round := 0; round < 3; round++ {
		m.reconcileLanguageExtensionConfigs([]languageserver.Config{config}, false)
		pump(func() bool { return len(m.languageBindings) == 1 && m.languageBindings[0].session != nil })
		old := m.languageBindings[0]
		session := old.session
		m.reconcileLanguageExtensionConfigs([]languageserver.Config{config}, false)
		if m.languageBindings[0] != old {
			t.Fatal("unchanged config restarted actual process")
		}
		before := count
		m.onDocument("focus", d, textbuffer.ChangeEvent{})
		if count != before+1 {
			t.Fatal("reconcile stacked document hooks")
		}
		m.diagnostics["lsp:"+config.Name+"\x00"+d.path] = []diagnostic{{Message: "old generation"}}
		m.reconcileLanguageExtensionConfigs(nil, false)
		if len(m.languageBindings) != 0 || old.session != nil || len(m.diagnostics["lsp:"+config.Name+"\x00"+d.path]) != 0 {
			t.Fatal("removed adapter retained UI state")
		}
		m.applyLanguageEvent(old, languageserver.Event{State: "ready", Session: session})
		if old.session != nil || len(m.languageBindings) != 0 {
			t.Fatal("queued old identity was revived")
		}
		deadline := time.NewTimer(3 * time.Second)
		for !session.Client.ProcessClosed() {
			select {
			case fn := <-mailbox:
				fn()
			case <-time.After(5 * time.Millisecond):
			case <-deadline.C:
				t.Fatal("real removed server not reaped")
			}
		}
		deadline.Stop()
	}
}

func TestActualLanguageAdapterDisableEnableUninstall(t *testing.T) {
	cache, kind := os.Getenv("GOCODE_LANGUAGE_EXTENSION_TEST_CACHE"), os.Getenv("GOCODE_LANGUAGE_EXTENSION_TEST_KIND")
	if cache == "" || kind == "" {
		t.Skip("actual installed Go/TS adapter gate requires explicit owned cache")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	root := filepath.Join(t.TempDir(), "extensions")
	name := "typescript-vsix.vsix"
	if kind == "go" {
		name = "go-0.50.0.vsix"
	}
	tools := filepath.Join(cache, "tools")
	if _, err := languageextension.InstallArchive(ctx, root, tools, kind, filepath.Join(cache, name)); err != nil {
		t.Fatal(err)
	}
	installed, err := extensions.List(root)
	if err != nil {
		t.Fatal(err)
	}
	configs, issues := nativeLanguageExtensionConfigs(installed, extensionSettings{}, tools)
	if len(configs) != 1 || len(issues) != 0 {
		t.Fatal(configs, issues)
	}
	m := testModel(t)
	d := m.current()
	if kind == "typescript" {
		d.path = filepath.Join(m.workspace, "main.ts")
		d.buffer, _ = textbuffer.New("export const greeting = '😀';\n")
	} else {
		d.buffer, _ = textbuffer.New("package main\nvar greeting = \"😀\"\n")
	}
	if err := os.WriteFile(d.path, []byte(d.buffer.Text()), 0600); err != nil {
		t.Fatal(err)
	}
	mailbox := make(chan func(), 32)
	dispatch := func(fn func()) bool { mailbox <- fn; return true }
	shutdown := m.bindLanguages(ctx, dispatch, nil)
	defer shutdown()
	pump := func(until func() bool) {
		t.Helper()
		for !until() {
			select {
			case fn := <-mailbox:
				fn()
			case <-ctx.Done():
				t.Fatal("real adapter stalled", m.lspStatus)
			}
		}
	}
	for _, action := range []string{"disable", "uninstall"} {
		m.reconcileLanguageExtensionConfigs(configs, false)
		pump(func() bool { return len(m.languageBindings) == 1 && m.languageBindings[0].session != nil })
		old := m.languageBindings[0]
		client := old.session.Client
		if client.ProcessID() <= 0 || !client.Supports("textDocument/completion") {
			t.Fatal("no real installed-server activation")
		}
		ids, dead, err := languageextension.ObserveProcesses(client, kind)
		if err != nil {
			t.Fatal("actual child observation", err)
		}
		state := extensionSettings{}
		if action == "disable" {
			state.Disabled = []string{installed[0].ID()}
		} else {
			state.Uninstall = []string{installed[0].ID()}
		}
		removed, issues := nativeLanguageExtensionConfigs(installed, state, tools)
		if len(removed) != 0 || len(issues) != 0 {
			t.Fatal("removed adapter still selected", removed, issues)
		}
		m.reconcileLanguageExtensionConfigs(removed, false)
		if len(m.languageBindings) != 0 || old.session != nil {
			t.Fatal("disabled/uninstalled UI binding remained")
		}
		deadline := time.NewTimer(3 * time.Second)
		for !client.ProcessClosed() {
			select {
			case fn := <-mailbox:
				fn()
			case <-time.After(5 * time.Millisecond):
			case <-deadline.C:
				t.Fatal("actual disabled/uninstalled language process survived")
			}
		}
		deadline.Stop()
		if err := dead(); err != nil {
			t.Fatal("actual observed server worker survived", err)
		}
		t.Logf("actual %s %s observed job PIDs=%v dead by retained OS handles", kind, action, ids)
		t.Logf("actual %s %s rootPID=%d explicitly closed", kind, action, client.ProcessID())
		if action == "uninstall" {
			receipt, err := languageextension.VerifyInstalled(root, kind)
			if err != nil {
				t.Fatal(err)
			}
			reinstallState := extensionSettings{Uninstall: append([]string(nil), state.Uninstall...), Disabled: []string{installed[0].ID()}}
			finished, err := finishLanguageExtensionInstall(root, reinstallState, receipt)
			if err != nil || len(finished.Uninstall) != 0 || !containsExtension(finished.Disabled, installed[0].ID()) {
				t.Fatal("explicit reinstall did not cancel only pendingUninstall", finished, err)
			}
			persisted, err := readExtensionSettings(root)
			if err != nil || len(persisted.Uninstall) != 0 || !containsExtension(persisted.Disabled, installed[0].ID()) {
				t.Fatal("real explicit reinstall settings were not persisted", persisted, err)
			}
			if len(reinstallState.Uninstall) != 1 {
				t.Fatal("immutable input settings were modified")
			}
			if err := removePendingExtensions(root, &state); err != nil {
				t.Fatal("actual deferred package removal", err)
			}
			remaining, err := extensions.List(root)
			if err != nil || len(remaining) != 0 {
				t.Fatal("actual uninstalled VSIX remains", remaining, err)
			}
			if _, err := languageextension.VerifyInstalled(root, kind); err == nil {
				t.Fatal("removed VSIX still verifies")
			}
			if _, err := languageextension.InstallArchive(ctx, root, tools, kind, filepath.Join(cache, name)); err != nil {
				t.Fatal("actual reinstall after uninstall", err)
			}
		}
	}
	if !strings.Contains(configs[0].Name, "extension:") {
		t.Fatal("activation was not explicit native adapter")
	}
}

func TestLanguageInstallRejectsStaleManagerReceipt(t *testing.T) {
	m := testModel(t)
	root := t.TempDir()
	m.extensionsView.settings = extensionSettings{Uninstall: []string{"golang.go"}}
	mailbox := make(chan func(), 4)
	dispatch := func(fn func()) bool { mailbox <- fn; return true }
	oldCtx, oldCancel := context.WithCancel(context.Background())
	stopOld := m.bindExtensionManager(oldCtx, dispatch, root)
	m.extensionsView.manage("disable", "fixture.old")
	var oldAck func()
	select {
	case oldAck = <-mailbox:
	case <-time.After(3 * time.Second):
		t.Fatal("old actual settings worker did not finish")
	}
	oldCancel()
	stopOld()
	m.extensionsView.settings = extensionSettings{Disabled: []string{"golang.go"}}
	m.extensionsView.busy = false
	stopNew := m.bindExtensionManager(context.Background(), dispatch, root)
	defer stopNew()
	m.extensionsView.manage("disable", "fixture.new")
	var newAck func()
	select {
	case newAck = <-mailbox:
	case <-time.After(3 * time.Second):
		t.Fatal("new actual settings worker did not finish")
	}
	oldAck()
	if !m.extensionsView.busy || len(m.extensionsView.settings.Uninstall) != 0 || !containsExtension(m.extensionsView.settings.Disabled, "golang.go") {
		t.Fatal("old actor acknowledgement changed new actor admission/settings")
	}
	newAck()
	if m.extensionsView.busy || len(m.extensionsView.settings.Uninstall) != 0 || !containsExtension(m.extensionsView.settings.Disabled, "fixture.new") {
		t.Fatal("current actual settings receipt was lost", m.extensionsView.busy, m.extensionsView.settings, m.message)
	}
}
