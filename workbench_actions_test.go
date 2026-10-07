package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gitrepo "github.com/neko233-com/gocode/internal/git"

	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
	"github.com/neko233-com/godesktop/extensions"
)

func TestVirtualDiffCommandsPreserveSourceEditor(t *testing.T) {
	m, err := newModel(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.closeDocuments()
	m.newTextFile()
	d := m.current()
	m.scm.diff = &gitrepo.Diff{}
	for _, c := range m.workbenchCommands() {
		if (c.ID == "save" || c.ID == "saveAs" || c.ID == "format" || c.ID == "definition" || c.ID == "undo") && c.Enabled {
			t.Fatal("source command enabled in read-only diff", c.ID)
		}
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'W', Modifiers: ui.ModifierControl})
	if m.scm.diff != nil || m.current() != d || len(m.docs) != 1 || m.closePrompt {
		t.Fatal("closing virtual diff closed or prompted the source document")
	}
}

func TestSaveAsImmutableUnicodeAndExclusiveNewTarget(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "保存世界.go")
	buffer, _ := textbuffer.New("original\r\n世界 😀\r\n")
	snapshot := buffer.Snapshot()
	buffer.ReplaceSelection("newer ")
	if _, err := writeSaveAs(context.Background(), path, snapshot); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != snapshot.Text() {
		t.Fatalf("saved snapshot mutated: %q", data)
	}
	temp := filepath.Join(root, "prepared")
	os.WriteFile(temp, []byte("overwrite"), 0600)
	if err := commitNewSave(temp, path); err == nil {
		t.Fatal("exclusive new target overwrote an existing file")
	}
	data, _ = os.ReadFile(path)
	if string(data) != snapshot.Text() {
		t.Fatal("collision changed target")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	missing := filepath.Join(root, "cancelled")
	if _, err := writeSaveAs(ctx, missing, snapshot); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("cancelled Save As created target")
	}
}
func TestQuickOpenCommandsAreDistinctAndMenuDisabledIsInert(t *testing.T) {
	m, _ := newWorkspaceModel(t.TempDir())
	m.files = []string{"alpha.go", "src/beta.go", "README.md"}
	m.openQuickInput(false)
	m.query = "src beta"
	items := m.quickItems()
	if len(items) != 1 || items[0].ID != "src/beta.go" {
		t.Fatal(items)
	}
	m.closeQuickInput()
	m.openQuickInput(true)
	m.query = ">new text"
	items = m.quickItems()
	if len(items) != 1 || items[0].ID != "newFile" {
		t.Fatal(items)
	}
	m.chooseQuickItem(items[0])
	if m.current() == nil || !m.current().untitled || m.palette {
		t.Fatal("command execution failed")
	}
	m.menu.name = "Run"
	m.selectMenuItem(nil, 0, 0, true)
	if m.menu.name != "Run" || m.message != "" {
		t.Fatal("disabled menu executed or dismissed")
	}
	m.menu.name = "File"
	m.menu.index = -1
	m.menuInput(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 40})
	if m.menu.index != 0 {
		t.Fatal("menu keyboard selection did not start at first enabled row")
	}
}
func fixtureExtension(t *testing.T, root string) extensions.Extension {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.vsix")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for _, e := range []struct{ name, text string }{{"package.json", `{"name":"native","publisher":"fixture","version":"0.1.0","main":"extension.cjs","contributes":{"commands":[{"command":"unrelated.command.id","title":"Actual Contributed Command"}]}}`}, {"extension.cjs", `exports.activate=c=>c.subscriptions.push(require('vscode').commands.registerCommand('unrelated.command.id',()=> 'real'));`}} {
		w, _ := z.Create("extension/" + e.name)
		w.Write([]byte(e.text))
	}
	if err = errors.Join(z.Close(), f.Close()); err != nil {
		t.Fatal(err)
	}
	e, err := extensions.Install(root, path)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func TestExtensionSettingsActuallyControlHostAndRemoveOnlyOwnedVersions(t *testing.T) {
	root := t.TempDir()
	e := fixtureExtension(t, root)
	state := extensionSettings{}
	call := func(active []extensions.Extension) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		h, err := extensions.Start(ctx, root, active)
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		if err = h.Call(ctx, "initialize", map[string]any{"documents": []any{}}, nil); err != nil {
			t.Fatal(err)
		}
		var result string
		err = h.Call(ctx, "execute", map[string]string{"command": "unrelated.command.id"}, &result)
		if err == nil && result != "real" {
			t.Fatalf("fabricated extension result %q", result)
		}
		return err
	}
	if err := call(enabledExtensions([]extensions.Extension{e}, state)); err != nil {
		t.Fatal(err)
	}
	state.Disabled = []string{e.ID()}
	if err := writeExtensionSettings(root, state); err != nil {
		t.Fatal(err)
	}
	state, err := readExtensionSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	if err = call(enabledExtensions([]extensions.Extension{e}, state)); err == nil || !strings.Contains(err.Error(), "Command not found") {
		t.Fatalf("disabled extension still active: %v", err)
	}
	keep := filepath.Join(root, "fixture.native-user-not-extension")
	os.Mkdir(keep, 0700)
	os.WriteFile(filepath.Join(keep, "user.txt"), []byte("keep"), 0600)
	state.Uninstall = []string{e.ID()}
	if err = removePendingExtensions(root, &state); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(e.Path); !os.IsNotExist(err) {
		t.Fatal("uninstalled extension directory survived")
	}
	if data, err := os.ReadFile(filepath.Join(keep, "user.txt")); err != nil || string(data) != "keep" {
		t.Fatal("uninstall removed unrelated user directory")
	}
	contributions := extensionContributions([]extensions.Extension{e})
	if contributions[e.ID()][0].ID != "unrelated.command.id" {
		t.Fatal("contributions guessed by command prefix")
	}
}

type catalogRoundTrip func(*http.Request) (*http.Response, error)

func (f catalogRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCatalogResolvesWindowsBeforeDownloadingAndVerifiesRealHash(t *testing.T) {
	body := []byte("actual bounded VSIX body")
	sum := sha256.Sum256(body)
	wrongHash := false
	windowsLookups := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/fixture/native/win32-x64/0.1.0":
			windowsLookups++
			http.NotFound(w, r)
		case "/api/fixture/native/universal/0.1.0":
			json.NewEncoder(w).Encode(map[string]any{"name": "native", "namespace": "fixture", "version": "0.1.0", "targetPlatform": "universal", "files": map[string]string{"download": "https://open-vsx.org/archive", "sha256": "https://open-vsx.org/hash"}})
		case "/archive":
			w.Write(body)
		case "/hash":
			if wrongHash {
				w.Write([]byte(strings.Repeat("0", 64)))
			} else {
				w.Write([]byte(hex.EncodeToString(sum[:])))
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	transport := server.Client().Transport
	client := &http.Client{Transport: catalogRoundTrip(func(r *http.Request) (*http.Response, error) {
		copy := r.Clone(r.Context())
		address := *r.URL
		address.Host = u.Host
		copy.URL = &address
		response, err := transport.RoundTrip(copy)
		if response != nil {
			response.Request = r
		}
		return response, err
	})}
	e := catalogExtension{Name: "native", Publisher: "fixture", Version: "0.1.0", TargetPlatform: "darwin-arm64"}
	e.Files.Download = "https://open-vsx.org/must-not-download-mac"
	path, cleanup, err := downloadCatalogVSIX(context.Background(), client, t.TempDir(), e)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	data, _ := os.ReadFile(path)
	if string(data) != string(body) || windowsLookups != 1 {
		t.Fatal("wrong platform/body")
	}
	wrongHash = true
	if _, _, err = downloadCatalogVSIX(context.Background(), client, t.TempDir(), e); err == nil {
		t.Fatal("bad hash was installed")
	}
}
