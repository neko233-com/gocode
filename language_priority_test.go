package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gocode/internal/languageserver"
	"github.com/neko233-com/godesktop/extensions"
	"github.com/neko233-com/godesktop/lsp"
)

func TestLanguagePriorityServerProcess(t *testing.T) {
	if len(os.Args) < 2 || os.Args[len(os.Args)-2] != "gocode-priority-fixture" {
		t.Skip("owned protocol-selection fixture only")
	}
	identity := os.Args[len(os.Args)-1]
	peer := lsp.Connect(os.Stdin, os.Stdout, nil)
	defer peer.Close()
	peer.Register("initialize", func(context.Context, json.RawMessage) (any, error) {
		return map[string]any{"capabilities": map[string]any{"positionEncoding": "utf-16", "textDocumentSync": 1, "hoverProvider": true}}, nil
	})
	peer.Register("textDocument/hover", func(context.Context, json.RawMessage) (any, error) {
		return map[string]any{"contents": map[string]string{"kind": "plaintext", "value": identity}}, nil
	})
	for range peer.Notifications() {
	}
}

func TestLanguageProviderPriorityAndAutomaticGoProvenance(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("APPDATA", configRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	managed, err := languageserver.ManagedGopls()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(managed), 0700); err != nil {
		t.Fatal(err)
	}
	// The real automatic loader chooses this actual owned executable. Its RPC
	// identity fixture exercises routing, not genuine gopls/VSIX compatibility.
	input, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	output, err := os.OpenFile(managed, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		input.Close()
		t.Fatal(err)
	}
	_, copyErr := io.Copy(output, io.LimitReader(input, 64<<20))
	inErr, outErr := input.Close(), output.Close()
	if copyErr != nil || inErr != nil || outErr != nil {
		t.Fatal(copyErr, inErr, outErr)
	}
	fixture := func(name, language, identity string) languageserver.Config {
		return languageserver.Config{Name: name, Languages: []string{language}, Command: os.Args[0], Arguments: []string{"-test.run=^TestLanguagePriorityServerProcess$", "--", "gocode-priority-fixture", identity}}
	}
	goNative := fixture("extension:golang.go", "go", "native-go")
	tsNative := fixture("extension:vscode.typescript-language-features", "typescript", "native-typescript")
	for _, explicit := range []bool{false, true} {
		name := "automatic"
		if explicit {
			name = "explicit-same-name"
		}
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "extensions")
			var initial []languageserver.Config
			if explicit {
				user := []languageserver.Config{fixture("gopls", "go", "explicit-gopls"), fixture("user-last", "go", "explicit-last")}
				// A user JSON property cannot forge internal automatic provenance.
				data, _ := json.Marshal(user)
				path := filepath.Join(t.TempDir(), "lsp.json")
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				initial, _, err = loadWorkbenchLanguageConfigs(path, root, nil, extensionSettings{})
				if err != nil || len(initial) != 2 || initial[0].AutomaticGoFallback || initial[1].AutomaticGoFallback {
					t.Fatal("explicit same-name configuration lost its true provenance", initial, err)
				}
			} else {
				initial, _, err = loadWorkbenchLanguageConfigs("", root, nil, extensionSettings{})
				if err != nil || len(initial) != 1 || !initial[0].AutomaticGoFallback || initial[0].Command != managed {
					t.Fatal("real automatic startup provenance missing", initial, err)
				}
				initial[0].Arguments = []string{"-test.run=^TestLanguagePriorityServerProcess$", "--", "gocode-priority-fixture", "automatic-go"}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			m := testModel(t)
			mailbox := make(chan func(), 32)
			shutdown := m.bindLanguages(ctx, func(fn func()) bool { mailbox <- fn; return true }, initial)
			defer shutdown()
			pump := func(until func() bool) {
				t.Helper()
				for !until() {
					select {
					case fn := <-mailbox:
						fn()
					case <-ctx.Done():
						t.Fatal("actual owned provider selection stalled", m.lspStatus, m.message)
					case <-time.After(time.Millisecond):
					}
				}
			}
			ready := func() bool {
				for _, b := range m.languageBindings {
					if b.session == nil {
						return false
					}
				}
				return true
			}
			pump(ready)
			original := append([]*languageBinding(nil), m.languageBindings...)
			sessions := make([]*languageserver.Session, len(original))
			for i, b := range original {
				sessions[i] = b.session
			}
			request := func(want string) {
				t.Helper()
				m.output = nil
				m.requestLSP(m.current(), "textDocument/hover")
				pump(func() bool { return len(m.output) > 0 })
				if m.output[0] != want {
					t.Fatal("actual RPC request chose wrong provider", m.output, want)
				}
			}
			want := "automatic-go"
			if explicit {
				want = "explicit-last"
			}
			request(want)
			// A TypeScript-only installation keeps the unrelated automatic Go service.
			if err := m.reconcileLanguageExtensionConfigs([]languageserver.Config{tsNative}, false); err != nil {
				t.Fatal(err)
			}
			pump(ready)
			request(want)
			for i, b := range original {
				if b.session != sessions[i] || !m.currentLanguageBinding(b) {
					t.Fatal("TS installation replaced an independent session")
				}
			}
			ts := m.languageBindings[0]
			tsSession := ts.session
			// This actual persistent marker models an installed-but-Disabled Go
			// receipt: there is deliberately no active native Go config yet.
			if err := os.MkdirAll(filepath.Join(root, ".gocode-language"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".gocode-language", "go.json"), []byte(`{}`), 0600); err != nil {
				t.Fatal(err)
			}
			suppress, err := suppressImplicitGoFallback(root, nil)
			if err != nil || !suppress {
				t.Fatal("persistent Go marker not propagated", suppress, err)
			}
			unsupported := extensions.Extension{Manifest: extensions.Manifest{Publisher: "golang", Name: "go", Version: "unsupported"}}
			if installed, err := suppressImplicitGoFallback(t.TempDir(), []extensions.Extension{unsupported}); err != nil || !installed {
				t.Fatal("recognized unsupported/disabled installed Go not propagated", installed, err)
			}
			if err := m.reconcileLanguageExtensionConfigs([]languageserver.Config{tsNative}, suppress); err != nil {
				t.Fatal(err)
			}
			if explicit {
				request("explicit-last")
			} else {
				pump(sessions[0].Client.ProcessClosed)
				if m.currentLanguageBinding(original[0]) {
					t.Fatal("first disabled Go install retained automatic fallback")
				}
			}
			for _, action := range []string{"enable", "disable", "enable", "uninstall"} {
				configs := []languageserver.Config{tsNative}
				if action == "enable" {
					configs = append(configs, goNative)
				}
				if err := m.reconcileLanguageExtensionConfigs(configs, false); err != nil {
					t.Fatal(err)
				}
				pump(ready)
				if ts.session != tsSession || !m.currentLanguageBinding(ts) {
					t.Fatal("unchanged TS session replaced")
				}
				if explicit {
					request("explicit-last")
					var actual []*languageBinding
					for _, b := range m.languageBindings {
						if !strings.HasPrefix(b.config.Name, "extension:") {
							actual = append(actual, b)
						}
					}
					if !reflect.DeepEqual(actual, original) {
						t.Fatal("independent order/identity changed", actual, original)
					}
					for i, b := range original {
						if b.session != sessions[i] || b.session.Client.ProcessClosed() {
							t.Fatal("explicit same-name user session was closed or replaced")
						}
					}
				} else if action == "enable" {
					request("native-go")
				} else {
					for _, b := range m.languageBindings {
						if b.config.AutomaticGoFallback || b.config.Supports(m.current().path) {
							t.Fatal("disabled/uninstalled Go resurrected a fallback", b.config)
						}
					}
				}
			}
		})
	}
}
