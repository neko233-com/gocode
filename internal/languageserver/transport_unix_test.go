//go:build darwin || linux

package languageserver

import (
	"errors"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func configureLanguageSibling(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func observeLanguageProcess(t *testing.T, pid int) func(bool) {
	return func(alive bool) {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for {
			err := syscall.Kill(pid, 0)
			actual := !errors.Is(err, syscall.ESRCH)
			if actual == alive {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("actual process%d alive=%t: %v", pid, alive, err)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
