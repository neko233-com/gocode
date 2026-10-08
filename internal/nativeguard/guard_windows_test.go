//go:build windows

package nativeguard

import (
	"context"
	"os/exec"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestGuardWindowsEnvironmentUsesActualOrdinalNameOrder(t *testing.T) {
	environment := []string{"PATHEXT=exe", "éclair=old雪", "b=B", "Path=private path", "a=A", "界=😀", "Ångström=å", `=C:=C:\private`, "TMP=parent", "tMp=owned private", "Éclair=final雪"}
	before := append([]string(nil), environment...)
	block, err := environmentBlock(environment)
	if err != nil {
		t.Fatal(err)
	}
	var actual []string
	for offset := 0; offset < len(block)-1; {
		end := offset
		for block[end] != 0 {
			end++
		}
		actual = append(actual, windows.UTF16ToString(block[offset:end]))
		offset = end + 1
	}
	expected := []string{`=C:=C:\private`, "a=A", "b=B", "Path=private path", "PATHEXT=exe", "tMp=owned private", "Ångström=å", "Éclair=final雪", "界=😀"}
	if !reflect.DeepEqual(actual, expected) || !reflect.DeepEqual(environment, before) || block[len(block)-1] != 0 || block[len(block)-2] != 0 {
		t.Fatalf("actual Unicode/case-insensitive name block=%q original=%q", actual, environment)
	}
	result, err := Run(context.Background(), Options{Command: helperCommand(t, "environment", "Path", "PATHEXT", "éclair", "界", "TMP"), Environment: environment, Timeout: 5 * time.Second})
	if err != nil || !result.Report.RootReaped || !result.Report.TreeClosed || !strings.Contains(string(result.Stdout), "Path=private path\nPATHEXT=exe\néclair=final雪\n界=😀\nTMP=owned private") || !reflect.DeepEqual(environment, before) {
		t.Fatalf("actual child environment changed: report=%+v stdout=%q error=%v", result.Report, result.Stdout, err)
	}
}

func configureSibling(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
}

func observeProcess(t *testing.T, pid int) func(bool) {
	t.Helper()
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(handle) })
	return func(alive bool) {
		t.Helper()
		wait := uint32(1000)
		if alive {
			wait = 0
		}
		result, err := windows.WaitForSingleObject(handle, wait)
		expected := uint32(windows.WAIT_OBJECT_0)
		if alive {
			expected = uint32(windows.WAIT_TIMEOUT)
		}
		if err != nil || result != expected {
			t.Fatalf("owned process PID%d alive=%t: wait=%d error=%v", pid, alive, result, err)
		}
	}
}
