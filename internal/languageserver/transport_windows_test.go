//go:build windows

package languageserver

import (
	"os/exec"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

func configureLanguageSibling(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
}

func observeLanguageProcess(t *testing.T, pid int) func(bool) {
	t.Helper()
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(handle) })
	return func(alive bool) {
		t.Helper()
		wait, expected := uint32(1000), uint32(windows.WAIT_OBJECT_0)
		if alive {
			wait, expected = 0, uint32(windows.WAIT_TIMEOUT)
		}
		actual, err := windows.WaitForSingleObject(handle, wait)
		if err != nil || actual != expected {
			t.Fatalf("actual process%d alive=%t: wait%d error%v", pid, alive, actual, err)
		}
	}
}
