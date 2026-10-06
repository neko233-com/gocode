//go:build !windows

package update

import "os/exec"

func hideProcess(*exec.Cmd) {}
