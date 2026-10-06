//go:build darwin

package terminal

import "golang.org/x/sys/unix"

func killDescendants(parent int) {
	list, err := unix.SysctlKinfoProcSlice("kern.proc.uid", unix.Getuid())
	if err != nil {
		return
	}
	children := map[int][]int{}
	for _, process := range list {
		children[int(process.Eproc.Ppid)] = append(children[int(process.Eproc.Ppid)], int(process.Proc.P_pid))
	}
	var kill func(int)
	kill = func(pid int) {
		for _, child := range children[pid] {
			kill(child)
			_ = unix.Kill(child, unix.SIGKILL)
		}
	}
	kill(parent)
}
