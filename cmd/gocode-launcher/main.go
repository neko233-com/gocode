package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/neko233-com/gocode/internal/installlayout"
)

func main() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	gui := strings.Contains(strings.ToLower(filepath.Base(exe)), "launch")
	program, _, err := installlayout.Resolve(filepath.Dir(exe), gui)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gocode installation:", err)
		os.Exit(1)
	}
	cmd := exec.Command(program, os.Args[1:]...)
	if gui && len(os.Args) == 1 {
		config, err := os.UserConfigDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		workspace := filepath.Join(config, "gocode", "workspaces", "scratch")
		if err := os.MkdirAll(workspace, 0700); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		cmd = exec.Command(program, "-workspace", workspace)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
