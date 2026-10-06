//go:build darwin

package terminal

import (
	"golang.org/x/sys/unix"
)

func processRunning(pid int) bool {
	info, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	return err == nil && info.Proc.P_pid == int32(pid) && info.Proc.P_stat != 5
}
