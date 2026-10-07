//go:build windows && cgo

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNativeWorkbenchAcceptanceIsIdempotent(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "gocode-idempotence.exe")
	if data, err := exec.Command("go", "build", "-race", "-o", exe, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, data)
	}
	scratch, settings, evidence := filepath.Join(root, "scratch"), filepath.Join(root, "settings"), filepath.Join(root, "evidence")
	for _, path := range []string{scratch, settings, evidence} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(settings, "preserved-user-settings.json")
	if err := os.WriteFile(marker, []byte("unchanged 用户设置\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var evidenceCount int
	for run := 0; run < 2; run++ {
		cmd := exec.Command(exe, "-windows-workbench-smoke")
		cmd.Env = append(os.Environ(), "TMP="+scratch, "TEMP="+scratch, "APPDATA="+settings, "GOCODE_WINDOWS_WORKBENCH_SCREENSHOTS="+evidence)
		if data, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("repeated native run %d: %v %s", run, err, data)
		}
		entries, err := os.ReadDir(scratch)
		if err != nil || len(entries) != 0 {
			t.Fatalf("run %d left temporary workspaces/VSIX/files: %v %v", run, entries, err)
		}
		body, err := os.ReadFile(marker)
		if err != nil || string(body) != "unchanged 用户设置\n" {
			t.Fatal("acceptance changed user settings")
		}
		entries, err = os.ReadDir(settings)
		if err != nil || len(entries) != 1 {
			t.Fatal("acceptance wrote to user settings", entries, err)
		}
		entries, err = os.ReadDir(evidence)
		if err != nil || len(entries) == 0 {
			t.Fatal("missing native evidence", err)
		}
		if run == 0 {
			evidenceCount = len(entries)
		} else if len(entries) != evidenceCount {
			t.Fatal("repeated acceptance accumulated screenshot copies")
		}
	}
}
