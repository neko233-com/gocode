//go:build !windows

package copilotservice

import "os/exec"

func hideProcess(*exec.Cmd) {}
