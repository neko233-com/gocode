//go:build windows && cgo

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/neko233-com/gocode/internal/terminal"
	"golang.org/x/sys/windows"
)

func TestNativeAutoSaveAcceptanceIsIdempotent(t *testing.T) {
	runtimeContext, stopRuntime := context.WithTimeout(context.Background(), 2*time.Minute)
	defer stopRuntime()
	conPTYRoot, err := terminal.EnsureConPTY(runtimeContext, "")
	if err != nil {
		t.Fatalf("prepare verified ConPTY runtime before isolated native test: %v", err)
	}
	root := t.TempDir()
	exe := filepath.Join(root, "gocode-auto-save.exe")
	if output, err := exec.Command("go", "build", "-buildvcs=false", "-race", "-o", exe, ".").CombinedOutput(); err != nil {
		t.Fatalf("build Auto Save fixture: %v\n%s", err, output)
	}
	scratch, settings, evidence := filepath.Join(root, "scratch"), filepath.Join(root, "settings"), filepath.Join(root, "evidence")
	for _, path := range []string{scratch, settings, evidence} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(settings, "preserved-user-settings.json")
	const original = "unchanged user settings 世界😀\r\n"
	if err := os.WriteFile(marker, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	var previous []string
	for run := range 2 {
		cmd := exec.Command(exe, "-auto-save-smoke")
		cmd.Env = append(os.Environ(), "TMP="+scratch, "TEMP="+scratch, "APPDATA="+settings, "GOCODE_AUTOSAVE_SCREENSHOTS="+evidence, "GOCODE_CONPTY_DIR="+conPTYRoot)
		cmd.WaitDelay = 3 * time.Second
		var output safeOutput
		cmd.Stdout, cmd.Stderr = &output, &output
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		job, err := windows.CreateJobObject(nil, nil)
		if err == nil {
			limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
			limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
			_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)))
		}
		if err == nil {
			var process windows.Handle
			process, err = windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
			if err == nil {
				err = windows.AssignProcessToJobObject(job, process)
				windows.CloseHandle(process)
			}
		}
		if err != nil {
			if job != 0 {
				windows.CloseHandle(job)
			}
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			t.Fatalf("own Auto Save process tree: %v", err)
		}
		finished := make(chan error, 1)
		go func() { finished <- cmd.Wait() }()
		select {
		case err = <-finished:
		case <-time.After(65 * time.Second):
			windows.CloseHandle(job)
			job = 0
			_ = cmd.Process.Kill()
			err = <-finished
			if err == nil {
				t.Fatal("Auto Save fixture exceeded its owned process guard")
			}
		}
		if job != 0 {
			windows.CloseHandle(job)
		}
		if err != nil || !strings.Contains(output.String(), "native Auto Save acceptance passed: 66 phases") {
			t.Fatalf("native Auto Save repeat %d: %v\n%s", run, err, output.String())
		}
		entries, err := os.ReadDir(scratch)
		if err != nil || len(entries) != 0 {
			t.Fatalf("repeat %d left temporary data: %v %v", run, entries, err)
		}
		body, err := os.ReadFile(marker)
		if err != nil || string(body) != original {
			t.Fatal("native acceptance changed user settings", err)
		}
		entries, err = os.ReadDir(settings)
		if err != nil || len(entries) != 1 {
			t.Fatal("native acceptance wrote outside its private settings", entries, err)
		}
		entries, err = os.ReadDir(evidence)
		if err != nil || len(entries) != 6 {
			t.Fatal("missing native Auto Save evidence", entries, err)
		}
		var names []string
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		if run != 0 && strings.Join(names, "\n") != strings.Join(previous, "\n") {
			t.Fatal("native acceptance accumulated evidence copies")
		}
		previous = names
	}
}
