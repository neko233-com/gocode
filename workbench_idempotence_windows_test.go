//go:build windows && cgo

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestNativeWorkbenchAcceptanceIsIdempotent(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "gocode-idempotence.exe")
	buildCtx, stopBuild := context.WithTimeout(context.Background(), 2*time.Minute)
	defer stopBuild()
	build := exec.CommandContext(buildCtx, "go", "build", "-race", "-o", exe, ".")
	build.WaitDelay = 3 * time.Second
	if data, err := build.CombinedOutput(); err != nil {
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
		nativeCtx, stopNative := context.WithTimeout(context.Background(), 60*time.Second)
		cmd := exec.CommandContext(nativeCtx, exe, "-windows-workbench-smoke")
		cmd.WaitDelay = 3 * time.Second
		cmd.Env = append(os.Environ(), "TMP="+scratch, "TEMP="+scratch, "APPDATA="+settings, "GOCODE_WINDOWS_WORKBENCH_SCREENSHOTS="+evidence)
		var output safeOutput
		cmd.Stdout, cmd.Stderr = &output, &output
		if err := cmd.Start(); err != nil {
			stopNative()
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
			stopNative()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			t.Fatalf("own repeated native process tree: %v", err)
		}
		finished := make(chan error, 1)
		go func() { finished <- cmd.Wait() }()
		select {
		case err = <-finished:
		case <-nativeCtx.Done():
			windows.CloseHandle(job)
			job = 0
			_ = cmd.Process.Kill()
			<-finished
			err = nativeCtx.Err()
		}
		stopNative()
		if job != 0 {
			windows.CloseHandle(job)
		}
		if err != nil {
			t.Fatalf("repeated native run %d: %v %s", run, err, output.String())
		}
		t.Logf("repeated native run %d ownedPID=%d: %s", run, cmd.Process.Pid, output.String())
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
