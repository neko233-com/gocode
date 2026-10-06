//go:build !windows

package copilotservice

import "os/exec"

func configureLoginProcess(*exec.Cmd) {}
