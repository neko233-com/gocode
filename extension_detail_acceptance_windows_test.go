//go:build windows && cgo

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/neko233-com/godesktop/extensions"
	"golang.org/x/sys/windows"
)

func TestExtensionDetailFixtureRealVSIXNode(t *testing.T) {
	root := t.TempDir()
	runtimeRoot := filepath.Join(root, "runtime")
	if err := os.Mkdir(runtimeRoot, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMP", runtimeRoot)
	t.Setenv("TEMP", runtimeRoot)
	t.Setenv("TMPDIR", runtimeRoot)
	fixture, err := installExtensionDetailFixture(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixture.Manifest.Contributes.Commands) != 2000 || !strings.HasPrefix(fixture.Manifest.Description, "START ") || !strings.HasSuffix(fixture.Manifest.Description, " END") {
		t.Fatal("real installed native fixture metadata differs")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	host, err := extensions.Start(ctx, root, []extensions.Extension{fixture})
	if err != nil {
		t.Fatal(err)
	}
	eventsDone := make(chan struct{})
	go func() {
		defer close(eventsDone)
		for range host.Events {
		}
	}()
	defer func() { _ = host.Close(); <-eventsDone }()
	if err = host.Call(ctx, "initialize", map[string]any{"storageRoot": filepath.Join(root, "state")}, nil); err != nil {
		t.Fatal(err)
	}
	var result string
	if err = host.Call(ctx, "execute", map[string]string{"command": "details.command.1999"}, &result); err != nil || result != "executed:1999" {
		t.Fatal("actual fixture Node command", result, err)
	}
}

func TestNativeExtensionDetailViewportAndIdempotence(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "gocode-extension-detail.exe")
	buildCtx, stopBuild := context.WithTimeout(context.Background(), 60*time.Second)
	build := exec.CommandContext(buildCtx, "go", "build", "-buildvcs=false", "-race", "-o", exe, ".")
	build.WaitDelay = 3 * time.Second
	output, err := build.CombinedOutput()
	stopBuild()
	if err != nil {
		t.Fatalf("build native extension detail: %v\n%s", err, output)
	}
	scratch, settings, evidence := filepath.Join(root, "scratch"), filepath.Join(root, "settings"), filepath.Join(root, "evidence")
	for _, dir := range []string{scratch, settings, evidence} {
		if err = os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(settings, "preserved-settings.json")
	const original = "unchanged private settings 世界😀\r\n"
	if err = os.WriteFile(marker, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	for run := range 2 {
		cmd := exec.Command(exe, "-extension-detail-smoke")
		cmd.Env = append(os.Environ(), "TMP="+scratch, "TEMP="+scratch, "TMPDIR="+scratch, "APPDATA="+settings, "GOCODE_EXTENSION_DETAIL_REPORT="+filepath.Join(evidence, "current.json"), "GODESKTOP_TEST_INPUT_ISOLATION=1")
		cmd.WaitDelay = 3 * time.Second
		var logs safeOutput
		cmd.Stdout, cmd.Stderr = &logs, &logs
		if err = cmd.Start(); err != nil {
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
			t.Fatalf("own native detail process tree: %v", err)
		}
		finished := make(chan error, 1)
		go func() { finished <- cmd.Wait() }()
		select {
		case err = <-finished:
		case <-time.After(60 * time.Second):
			windows.CloseHandle(job)
			job = 0
			_ = cmd.Process.Kill()
			<-finished
			t.Fatal("native detail fixture exceeded 60s owned process guard")
		}
		if job != 0 {
			windows.CloseHandle(job)
		}
		if err != nil || !strings.Contains(logs.String(), "native extension detail acceptance passed") {
			t.Fatalf("native detail repeat %d: %v\n%s", run, err, logs.String())
		}
		body, err := os.ReadFile(filepath.Join(evidence, "current.json"))
		var report extensionDetailNativeReport
		if err != nil || len(body) > 4096 || json.Unmarshal(body, &report) != nil || len(report.Gates) != 10 || report.NativeFontWidth <= 0 || report.DescriptionRows < 10 || report.Commands != 2000 || report.ExecutedCommand != "details.command.1999" || len(report.DiskSHA256) != 64 || report.Backend != "direct3d12" || report.Submitted < 3 {
			t.Fatalf("native detail gates missing: %#v %v", report, err)
		}
		entries, err := os.ReadDir(scratch)
		if err != nil || len(entries) != 0 {
			t.Fatal("native detail left disposable runtimes/workspaces", entries, err)
		}
		body, err = os.ReadFile(marker)
		if err != nil || string(body) != original {
			t.Fatal("native detail changed preserved settings", err)
		}
		entries, err = os.ReadDir(settings)
		if err != nil || len(entries) != 1 {
			t.Fatal("native detail created user settings", entries, err)
		}
		entries, err = os.ReadDir(evidence)
		if err != nil || len(entries) != 4 {
			t.Fatal("native detail evidence accumulates files", entries, err)
		}
		for _, name := range []string{"details.png", "narrow.png", "contributions-bottom.png"} {
			if info, err := os.Stat(filepath.Join(evidence, name)); err != nil || info.Size() == 0 {
				t.Fatal("real native GPU capture missing", name, err)
			}
		}
	}
}
