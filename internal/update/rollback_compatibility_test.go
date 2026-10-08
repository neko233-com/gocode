package update

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gocode/internal/installlayout"
	"github.com/neko233-com/gocode/internal/languageextension"
)

func rollbackSelection(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	root := t.TempDir()
	before := map[string][]byte{}
	for _, item := range []struct{ name, version string }{{"base.json", "0.23.0"}, {"current.json", "0.24.0"}, {"previous.json", "0.23.0"}} {
		data, _ := json.Marshal(installlayout.Manifest{Owner: installlayout.Owner, Schema: 1, Version: item.version, Source: strings.Repeat("a", 40)})
		if err := os.WriteFile(filepath.Join(root, item.name), data, 0600); err != nil {
			t.Fatal(err)
		}
		before[item.name] = data
	}
	for _, version := range []string{"0.23.0", "0.24.0"} {
		if err := os.MkdirAll(filepath.Join(root, "versions", version), 0700); err != nil {
			t.Fatal(err)
		}
	}
	return root, before
}

func unchangedRollbackSelection(t *testing.T, root string, before map[string][]byte) {
	t.Helper()
	for name, wanted := range before {
		actual, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(actual) != string(wanted) {
			t.Fatal("rejected rollback changed selection", name, err)
		}
	}
}

func TestRollbackPreferenceFailurePrecedesHealthAndKeepsPointers(t *testing.T) {
	root, before := rollbackSelection(t)
	extensions := t.TempDir()
	data := []byte(strings.Repeat("x", languageextension.MaxStateBytes+1))
	if err := os.WriteFile(filepath.Join(extensions, ".gocode-state.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Rollback(context.Background(), root, nil, extensions); err == nil || !strings.Contains(err.Error(), "extension preferences") {
		t.Fatal("unbounded preferences reached executable probe/selection", err)
	}
	unchangedRollbackSelection(t, root, before)
	actual, _ := os.ReadFile(filepath.Join(extensions, ".gocode-state.json"))
	if string(data) != string(actual) {
		t.Fatal("rollback modified preferences")
	}
}

func TestActualRollbackRechecksEnableDuringRealHealthProbe(t *testing.T) {
	profile := os.Getenv("GOCODE_ROLLBACK_TEST_PROFILE")
	if profile == "" {
		t.Skip("actual native-only package/control requires a verified owned profile")
	}
	root, before := rollbackSelection(t)
	extensions := t.TempDir()
	for _, kind := range []string{"go", "typescript"} {
		receipt, err := languageextension.VerifyInstalled(profile, kind)
		if err != nil {
			t.Fatal(err)
		}
		pin, _ := languageextension.Package(kind)
		packageName := pin.ID + "-" + pin.Version
		for _, file := range receipt.Files {
			data, err := os.ReadFile(filepath.Join(profile, packageName, file.Path))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(extensions, packageName, file.Path)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		marker := filepath.Join(".gocode-language", kind+".json")
		data, err := os.ReadFile(filepath.Join(profile, marker))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(extensions, marker)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(extensions, marker), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	statePath := filepath.Join(extensions, ".gocode-state.json")
	if err := os.WriteFile(statePath, []byte(`{"disabled":["golang.go","vscode.typescript-language-features"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	// A real non-GUI health executable holds the same five-second Probe window.
	ready, release := filepath.Join(root, "probe-ready"), filepath.Join(root, "probe-release")
	sourcePath := filepath.Join(t.TempDir(), "health.go")
	text := "package main\nimport(\"fmt\";\"os\";\"time\")\nfunc main(){if os.WriteFile(" + strconvQuote(ready) + ",nil,0600)!=nil{os.Exit(2)};deadline:=time.Now().Add(3*time.Second);for{if _,e:=os.Stat(" + strconvQuote(release) + ");e==nil{break};if time.Now().After(deadline){os.Exit(3)};time.Sleep(time.Millisecond)};fmt.Println(" + strconvQuote("gocode 0.23.0 "+runtime.GOOS+"/"+runtime.GOARCH+" "+strings.Repeat("a", 40)) + ")}"
	if err := os.WriteFile(sourcePath, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	name := "gocode-app"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	buildCtx, stopBuild := context.WithTimeout(context.Background(), 2*time.Minute)
	defer stopBuild()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", filepath.Join(root, "versions", "0.23.0", name), sourcePath)
	build.WaitDelay = 3 * time.Second
	hideProcess(build)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("real health fixture build: %v %s", err, output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Rollback(ctx, root, nil, extensions) }()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		select {
		case err := <-done:
			t.Fatal("rollback ended before actual probe handoff", err)
		case <-ctx.Done():
			t.Fatal("actual probe never started")
		case <-ticker.C:
		}
	}
	if err := os.WriteFile(statePath, []byte(`{"disabled":[],"uninstall":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil || !strings.Contains(err.Error(), "disable these extensions first") {
		t.Fatal("re-enabled adapter bypassed final selection check", err)
	}
	unchangedRollbackSelection(t, root, before)
	state, err := languageextension.ReadState(extensions)
	if err != nil || len(state.Disabled) != 0 || len(state.Uninstall) != 0 {
		t.Fatal("rejected rollback silently disabled user adapters", state, err)
	}
}

func strconvQuote(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}
