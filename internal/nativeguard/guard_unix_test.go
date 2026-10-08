//go:build darwin || linux

package nativeguard

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func configureSibling(*exec.Cmd) {}

func observeProcess(t *testing.T, pid int) func(bool) {
	t.Helper()
	return func(alive bool) {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for {
			output, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "stat=").Output()
			state := strings.TrimSpace(string(output))
			actualAlive := err == nil && state != "" && !strings.HasPrefix(state, "Z")
			// Orphan zombies have terminated; the OS reaper owns their final entry.
			if actualAlive == alive {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("owned PID%d alive=%t: state=%q error=%v", pid, alive, state, err)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
