package languageextension

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestPinnedIdentityAndCompleteInventory(t *testing.T) {
	for _, kind := range []string{"go", "typescript"} {
		pin, err := Package(kind)
		if err != nil || Kind(pin.ID) != kind || len(pin.SHA256) != 64 {
			t.Fatal(pin, err)
		}
		address, _ := url.Parse(pin.URL)
		if !downloadAllowed(address) {
			t.Fatal("pin URL not allowed")
		}
	}
	if _, err := Package("typescript-next"); err == nil {
		t.Fatal("arbitrary package treated as supported")
	}
	if Kind("fixture.go") != "" {
		t.Fatal("unrelated extension intercepted")
	}
	a := []File{{"b", "aa", 2}, {"a", "bb", 1}}
	b := []File{{"a", "bb", 1}, {"b", "aa", 2}}
	if inventory(a) != inventory(b) {
		t.Fatal("inventory depends on archive order")
	}
	b[0].Bytes++
	if inventory(a) == inventory(b) {
		t.Fatal("inventory did not bind actual bytes")
	}
	for _, raw := range []string{"http://open-vsx.org/a", "https://evil.test/a", "https://user@open-vsx.org/a", "https://open-vsx.org:8443/a"} {
		u, _ := url.Parse(raw)
		if downloadAllowed(u) {
			t.Fatal(raw)
		}
	}
}

func TestUnverifiedArchiveCancellationAndLinkedPaths(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "wrong.vsix")
	if err := os.WriteFile(archive, []byte("wrong"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallArchive(context.Background(), filepath.Join(root, "installed"), filepath.Join(root, "tools"), "go", archive); err == nil {
		t.Fatal("unverified VSIX accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "tools")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unverified archive started runtime installation", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Install(ctx, &http.Client{}, root, root, "go"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled installation continued", err)
	}
	if _, err := EnsureRuntime(ctx, root, "go"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled runtime reused", err)
	}
	if _, err := regular(root, "../escape", 1024); err == nil {
		t.Fatal("non-local package path accepted")
	}
	if _, err := regular(root, "wrong.vsix", 1); err == nil {
		t.Fatal("over-limit installed file accepted")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err == nil {
		if err := os.WriteFile(filepath.Join(outside, "x"), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := regular(root, filepath.Join("linked", "x"), 10); err == nil {
			t.Fatal("linked package parent accepted")
		}
	}
}

type forbidNetwork struct{ t *testing.T }

func (f forbidNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	f.t.Error("idempotent installation attempted network access")
	return nil, errors.New("network forbidden")
}

// Opt in with an owned cache of the genuine public archives. This is a real
// npm/go install and real gopls/tsserver protocol run, never a mocked native pass.
func TestActualInstalledVSIXServerAndIdempotence(t *testing.T) {
	cache := os.Getenv("GOCODE_LANGUAGE_EXTENSION_TEST_CACHE")
	if cache == "" {
		t.Skip("actual cached public VSIX/process gate requires explicit owned cache")
	}
	kind := os.Getenv("GOCODE_LANGUAGE_EXTENSION_TEST_KIND")
	if kind == "" {
		t.Fatal("explicit actual language kind required")
	}
	name := "typescript-vsix.vsix"
	if kind == "go" {
		name = "go-0.50.0.vsix"
	}
	root := filepath.Join(t.TempDir(), "extensions")
	tools := filepath.Join(cache, "tools")
	ctx, cancel := context.WithTimeout(context.Background(), 2*MaxInstallTime)
	defer cancel()
	first, err := InstallArchive(ctx, root, tools, kind, filepath.Join(cache, name))
	if err != nil {
		t.Fatal(err)
	}
	if first.Reused {
		t.Fatal("fresh original VSIX installation recorded as reused")
	}
	before, err := os.ReadFile(receiptPath(root, kind))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Install(ctx, &http.Client{Transport: forbidNetwork{t}}, root, tools, kind)
	if err != nil || !second.Reused {
		t.Fatal("actual idempotent install", second, err)
	}
	after, _ := os.ReadFile(receiptPath(root, kind))
	if string(before) != string(after) {
		t.Fatal("reinstall modified verified receipt")
	}
	report, err := Check(ctx, root, tools, kind)
	if err != nil {
		t.Fatalf("real %s check: %+v %v", kind, report, err)
	}
	if !report.ProcessClosed || len(report.Gates) != 7 || report.Completion != "greeting" || report.FormatEdits == 0 {
		t.Fatal("missing actual server proof", report)
	}
	t.Logf("actual installed server receipt: %+v", report)
	data, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(filepath.Join(cache, "actual-"+kind+"-current.json"), append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	modified := first
	modified.Files = append([]File(nil), first.Files[1:]...)
	if err = writeJSON(receiptPath(root, kind), modified); err != nil {
		t.Fatal(err)
	}
	if _, err = VerifyInstalled(root, kind); err == nil {
		t.Fatal("incomplete receipt bypassed original file inventory")
	}
	if err = os.WriteFile(receiptPath(root, kind), before, 0600); err != nil {
		t.Fatal(err)
	}
	pin, _ := Package(kind)
	if err = os.WriteFile(filepath.Join(installedPath(root, pin), "package.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = VerifyInstalled(root, kind); err == nil {
		t.Fatal("modified original VSIX accepted")
	}
}
