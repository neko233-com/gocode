//go:build !windows

package main

import (
	"os"
	"os/exec"
)

func configureWorkbenchProcess(*exec.Cmd)       {}
func commitNewSave(source, target string) error { return os.Link(source, target) }
