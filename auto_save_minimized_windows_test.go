//go:build windows && cgo

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestNativeMinimizedAutoSaveReceiptsAndIdempotence(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "gocode-auto-save-minimized.exe")
	if output, err := exec.Command("go", "build", "-buildvcs=false", "-race", "-o", exe, ".").CombinedOutput(); err != nil {
		t.Fatalf("build minimized Auto Save fixture: %v\n%s", err, output)
	}
	scratch, settings, evidence := filepath.Join(root, "scratch"), filepath.Join(root, "settings"), filepath.Join(root, "evidence")
	for _, path := range []string{scratch, settings, evidence} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(settings, "preserved-user-settings.json")
	const original = "unchanged settings 世界😀\r\n"
	if err := os.WriteFile(marker, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(evidence, "current.json")
	for run := range 2 {
		cmd := exec.Command(exe, "-auto-save-minimized-smoke")
		cmd.Env = append(os.Environ(), "TMP="+scratch, "TEMP="+scratch, "APPDATA="+settings, "GOCODE_AUTOSAVE_MINIMIZED_REPORT="+reportPath, "GODESKTOP_TEST_INPUT_ISOLATION=1")
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
			t.Fatalf("own minimized Auto Save process tree: %v", err)
		}
		finished := make(chan error, 1)
		go func() { finished <- cmd.Wait() }()
		select {
		case err = <-finished:
		case <-time.After(40 * time.Second):
			windows.CloseHandle(job)
			job = 0
			_ = cmd.Process.Kill()
			err = <-finished
			if err == nil {
				t.Fatal("minimized fixture exceeded its owned process guard")
			}
		}
		if job != 0 {
			windows.CloseHandle(job)
		}
		if err != nil || !strings.Contains(output.String(), "native minimized Auto Save acceptance passed") {
			t.Fatalf("minimized native repeat %d: %v\n%s", run, err, output.String())
		}
		body, err := os.ReadFile(reportPath)
		var report minimizedAutoSaveReport
		if err != nil || json.Unmarshal(body, &report) != nil || len(report.Phases) != 2 || !report.Restored {
			t.Fatal("actual minimized native report missing", report, err)
		}
		for i, phase := range report.Phases {
			if !phase.Iconic || phase.State.Dirty || phase.State.SaveBusy || phase.State.SaveID != uint64(i+1) || phase.State.DidSave != i+1 || phase.ViewsBefore != phase.ViewsAfter || phase.Before.Submitted != phase.After.Submitted || phase.After.Backend != "direct3d12" {
				t.Fatal("minimized native save/View/GPU invariants differ", phase)
			}
		}
		entries, err := os.ReadDir(scratch)
		if err != nil || len(entries) != 0 {
			t.Fatal("minimized fixture left temporary data", entries, err)
		}
		body, err = os.ReadFile(marker)
		if err != nil || string(body) != original {
			t.Fatal("minimized fixture changed private settings marker", err)
		}
		entries, err = os.ReadDir(settings)
		if err != nil || len(entries) != 1 {
			t.Fatal("minimized fixture wrote settings", entries, err)
		}
		entries, err = os.ReadDir(evidence)
		if err != nil || len(entries) != 1 || entries[0].Name() != "current.json" {
			t.Fatal("repeated minimized evidence accumulated files", entries, err)
		}
	}
}
