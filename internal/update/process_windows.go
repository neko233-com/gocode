//go:build windows

package update

import (
	"os/exec"
	"syscall"
)

func hideProcess(command *exec.Cmd) { command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
