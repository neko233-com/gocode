package languageextension

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRollbackCompatibilityOrphanMarkerStateBoundsAndCancellation(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".gocode-language"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath(root, "go"), []byte(`{"stale":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CheckRollback(context.Background(), root, "0.23.0"); err != nil {
		t.Fatal("uninstalled persistent marker prevented rollback", err)
	}
	pin, _ := Package("typescript")
	if err := os.MkdirAll(installedPath(root, pin), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installedPath(root, pin), "package.json"), []byte(`{"publisher":"vscode","name":"typescript-language-features","version":"1.95.3"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CheckRollback(context.Background(), root, "0.23.0"); err != nil {
		t.Fatal("ordinary VSIX without native receipt was mistaken for native support", err)
	}
	statePath := filepath.Join(root, ".gocode-state.json")
	for _, data := range [][]byte{[]byte(strings.Repeat("x", MaxStateBytes+1)), []byte(`{"disabled":[` + strings.Repeat(`"x",`, 512) + `"x"]}`), []byte(`{"uninstall":false}`)} {
		if err := os.WriteFile(statePath, data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := CheckRollback(context.Background(), root, "0.23.0"); err == nil {
			t.Fatal("invalid or unbounded preferences permitted rollback")
		}
		actual, _ := os.ReadFile(statePath)
		if string(actual) != string(data) {
			t.Fatal("rollback rewrote user preferences")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := CheckRollback(ctx, root, "0.24.0"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled current-compatible rollback continued", err)
	}
	if err := CheckRollback(context.Background(), "", "0.23.0"); err == nil {
		t.Fatal("unspecified extension root bypassed compatibility check")
	}
	if err := CheckRollback(context.Background(), root, "0.24.0"); err != nil {
		t.Fatal("current adapter-capable version was blocked by old-only policy", err)
	}
}

func copyActualRollbackPackages(t *testing.T) string {
	t.Helper()
	profile := os.Getenv("GOCODE_ROLLBACK_TEST_PROFILE")
	if profile == "" {
		t.Skip("actual original pinned VSIX rollback control requires an owned verified profile")
	}
	root := t.TempDir()
	for _, kind := range []string{"go", "typescript"} {
		original, err := VerifyInstalled(profile, kind)
		if err != nil {
			t.Fatal(err)
		}
		pin, _ := Package(kind)
		for _, file := range original.Files {
			data, err := os.ReadFile(filepath.Join(installedPath(profile, pin), file.Path))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(installedPath(root, pin), file.Path)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		data, err := os.ReadFile(receiptPath(profile, kind))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(receiptPath(root, kind)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(receiptPath(root, kind), data, 0600); err != nil {
			t.Fatal(err)
		}
		copied, err := VerifyInstalled(root, kind)
		if err != nil || !reflect.DeepEqual(copied, original) {
			t.Fatal("original VSIX copy differs", err)
		}
	}
	return root
}

func TestActualRollbackCompatibilityEnabledDisabledPendingAndDamagedPackage(t *testing.T) {
	root := copyActualRollbackPackages(t)
	ctx := context.Background()
	if err := CheckRollback(ctx, root, "0.23.0"); err == nil || !strings.Contains(err.Error(), "disable these extensions first") || !strings.Contains(err.Error(), "golang.go") || !strings.Contains(err.Error(), "vscode.typescript-language-features") {
		t.Fatal("actual enabled native adapters admitted to old host", err)
	}
	statePath := filepath.Join(root, ".gocode-state.json")
	for _, state := range []State{{Disabled: []string{"GOLANG.GO", "vscode.typescript-language-features", "independent.keep"}}, {Uninstall: []string{"golang.go", "VSCODE.TYPESCRIPT-LANGUAGE-FEATURES"}}} {
		data, _ := json.Marshal(state)
		if err := os.WriteFile(statePath, data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := CheckRollback(ctx, root, "0.23.0"); err != nil {
			t.Fatal("disabled or deferred uninstalled adapter remained active", err)
		}
		actual, _ := os.ReadFile(statePath)
		if string(data) != string(actual) {
			t.Fatal("compatibility changed user settings")
		}
	}
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	pin, _ := Package("go")
	if err := os.WriteFile(filepath.Join(installedPath(root, pin), "README.md"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CheckRollback(ctx, root, "0.23.0"); err == nil || !strings.Contains(err.Error(), "cannot verify golang.go") {
		t.Fatal("damaged original package allowed unchecked rollback", err)
	}
}
