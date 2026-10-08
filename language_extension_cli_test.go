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

	"github.com/neko233-com/gocode/internal/languageextension"
	"github.com/neko233-com/gocode/internal/nativeguard"
	"github.com/neko233-com/godesktop/extensions"
)

// This exercises the actual main install flag in a CGO0 subprocess. Original
// pinned packages/tools are opt-in local inputs, never downloaded by a test.
func TestActualLanguageExtensionCLIReinstall(t *testing.T) {
	cache := os.Getenv("GOCODE_LANGUAGE_EXTENSION_TEST_CACHE")
	if cache == "" {
		t.Skip("actual CLI reinstall requires explicit verified original-package cache")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 120*time.Second)
	defer cancel()
	private := t.TempDir()
	exe := filepath.Join(private, "gocode-cli.exe")
	environment := cliLanguageTestEnvironment(os.Environ(), map[string]string{"CGO_ENABLED": "0", "GOWORK": "off", "GOFLAGS": ""})
	build, err := nativeguard.Run(ctx, nativeguard.Options{Command: []string{"go", "build", "-buildvcs=false", "-o", exe, "."}, Environment: environment, Timeout: 120 * time.Second})
	if err != nil {
		t.Fatalf("actual CGO0 CLI build: %v: %s", err, build.Stderr)
	}
	for _, kind := range []string{"go", "typescript"} {
		t.Run(kind, func(t *testing.T) {
			pin, _ := languageextension.Package(kind)
			root := filepath.Join(private, kind, "extensions")
			config := filepath.Join(private, kind, "APPDATA")
			tools := filepath.Join(config, "gocode", "tools", "language-extensions")
			version, archive := languageextension.TSVersion, "typescript-vsix.vsix"
			if kind == "go" {
				version, archive = "v0.23.0", "go-0.50.0.vsix"
			}
			copyLanguageCLIRuntime(t, filepath.Join(cache, "tools", kind, version), filepath.Join(tools, kind, version))
			if _, err := languageextension.InstallArchive(ctx, root, tools, kind, filepath.Join(cache, archive)); err != nil {
				t.Fatal("original verified VSIX/runtime setup", err)
			}
			state := extensionSettings{Disabled: []string{strings.ToUpper(pin.ID), "fixture.keep"}, Uninstall: []string{strings.ToUpper(pin.ID), "fixture.other"}}
			if err := writeExtensionSettings(root, state); err != nil {
				t.Fatal(err)
			}
			tmp := filepath.Join(private, kind, "TMP")
			if err := os.MkdirAll(tmp, 0700); err != nil {
				t.Fatal(err)
			}
			env := cliLanguageTestEnvironment(os.Environ(), map[string]string{"APPDATA": config, "XDG_CONFIG_HOME": config, "TMP": tmp, "TEMP": tmp, "HTTP_PROXY": "http://127.0.0.1:1", "HTTPS_PROXY": "http://127.0.0.1:1", "NO_PROXY": ""})
			invoke := func() nativeguard.Result {
				t.Helper()
				result, err := nativeguard.Run(ctx, nativeguard.Options{Command: []string{exe, "-install-language-extension", kind, "-extensions-dir", root}, Environment: env, Timeout: 30 * time.Second})
				if err != nil {
					t.Fatalf("actual CLI install: %v: %s", err, result.Stderr)
				}
				var receipt languageextension.Receipt
				if err := json.Unmarshal(result.Stdout, &receipt); err != nil || receipt.ID != pin.ID || receipt.Version != pin.Version || receipt.ArchiveSHA256 != pin.SHA256 || !receipt.Reused {
					t.Fatal("CLI did not return its actual original-package reuse receipt", receipt, err)
				}
				if !result.Report.RootReaped || !result.Report.TreeClosed {
					t.Fatal("actual CLI process was not reaped", result.Report)
				}
				return result
			}
			first := invoke()
			persisted, err := readExtensionSettings(root)
			if err != nil || !reflect.DeepEqual(persisted.Disabled, state.Disabled) || !reflect.DeepEqual(persisted.Uninstall, []string{"fixture.other"}) {
				t.Fatal("CLI failed to clear only its own pending uninstall or changed Disabled", persisted, err)
			}
			// Real startup removal must preserve the explicitly reinstalled package.
			if err := removePendingExtensions(root, &persisted); err != nil {
				t.Fatal(err)
			}
			if _, err := languageextension.VerifyInstalled(root, kind); err != nil {
				t.Fatal("next startup deleted the actual successful CLI reinstall", err)
			}
			settingsPath := filepath.Join(root, ".gocode-state.json")
			before, err := os.ReadFile(settingsPath)
			if err != nil {
				t.Fatal(err)
			}
			second := invoke()
			after, err := os.ReadFile(settingsPath)
			if err != nil || string(before) != string(after) {
				t.Fatal("already-finished CLI reinstall changed settings", err)
			}
			installed, err := extensions.List(root)
			if err != nil {
				t.Fatal(err)
			}
			configs, issues := nativeLanguageExtensionConfigs(installed, persisted, tools)
			if len(configs) != 0 || len(issues) != 0 {
				t.Fatal("CLI silently enabled the user's disabled adapter", configs, issues)
			}
			t.Logf("actual %s CLI PIDs=%d/%d reaped; original package preserved, own pending uninstall cleared, Disabled retained; reuse with denied network", kind, first.Report.PID, second.Report.PID)
		})
	}
}

func cliLanguageTestEnvironment(base []string, values map[string]string) []string {
	var result []string
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		keep := true
		for name := range values {
			if strings.EqualFold(name, key) {
				keep = false
				break
			}
		}
		if keep {
			result = append(result, entry)
		}
	}
	for name, value := range values {
		result = append(result, name+"="+value)
	}
	return result
}

func copyLanguageCLIRuntime(t *testing.T, source, target string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(source, "runtime.json"))
	if err != nil {
		t.Fatal(err)
	}
	var runtime languageextension.Runtime
	if err := json.Unmarshal(data, &runtime); err != nil || len(runtime.Files) == 0 || len(runtime.Files) > 4096 {
		t.Fatal("actual cached runtime inventory", err)
	}
	var total int64
	for _, file := range runtime.Files {
		if !filepath.IsLocal(file.Path) || file.Bytes < 0 || file.Bytes > 64<<20 {
			t.Fatal("runtime copy boundary", file)
		}
		total += file.Bytes
		if total > 64<<20 {
			t.Fatal("runtime copy aggregate bound")
		}
		from, to := filepath.Join(source, file.Path), filepath.Join(target, file.Path)
		info, err := os.Lstat(from)
		if err != nil || !info.Mode().IsRegular() || info.Size() != file.Bytes {
			t.Fatal("cached runtime regular-file boundary", from, err)
		}
		if err := os.MkdirAll(filepath.Dir(to), 0700); err != nil {
			t.Fatal(err)
		}
		input, err := os.Open(from)
		if err != nil {
			t.Fatal(err)
		}
		output, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			input.Close()
			t.Fatal(err)
		}
		n, copyErr := io.CopyN(output, input, file.Bytes)
		inErr, outErr := input.Close(), output.Close()
		if copyErr != nil || inErr != nil || outErr != nil || n != file.Bytes {
			t.Fatal("actual runtime byte copy", copyErr, inErr, outErr)
		}
	}
	if err := os.WriteFile(filepath.Join(target, "runtime.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}
