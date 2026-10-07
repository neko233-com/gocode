//go:build darwin || linux

package git

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestOwnedUnixCancellationKillsDescendant(t *testing.T) {
	cancelOwnedTree(t, func(pid int) func() {
		return func() {
			// A terminated orphan may briefly remain a zombie while init reaps it.
			deadline := time.Now().Add(time.Second)
			for time.Now().Before(deadline) {
				output, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "stat=").Output()
				state := strings.TrimSpace(string(output))
				if err != nil || state == "" || strings.HasPrefix(state, "Z") {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Fatal("owned descendant survived process-group cancellation")
		}
	})
}
