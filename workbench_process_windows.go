//go:build windows

package main

import (
	"golang.org/x/sys/windows"
	"os/exec"
	"syscall"
)

func configureWorkbenchProcess(cmd *exec.Cmd) {
	// This is the requested interactive workbench. Suppress its console only;
	// STARTUPINFO/SW_HIDE would suppress the first native ShowWindow as well.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
}
func commitNewSave(source, target string) error {
	s, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	t, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(s, t, windows.MOVEFILE_WRITE_THROUGH)
}
