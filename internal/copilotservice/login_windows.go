//go:build windows

package copilotservice

import (
	"os/exec"
	"syscall"
)

func configureLoginProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
