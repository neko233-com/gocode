//go:build linux || darwin

package terminal

import "golang.org/x/sys/unix"

func killTerminalTree(pid int) error {
	// Descendants are collected while the root still exists. Closing the PTY
	// also hangs up its foreground job; the session's shell group is ours only.
	killDescendants(pid)
	err := unix.Kill(-pid, unix.SIGKILL)
	if err == unix.ESRCH {
		return nil
	}
	return err
}
