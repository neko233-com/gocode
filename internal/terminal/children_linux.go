//go:build linux

package terminal

import (
	"golang.org/x/sys/unix"
	"os"
	"strconv"
	"strings"
)

func killDescendants(parent int) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(parent) + "/task/" + strconv.Itoa(parent) + "/children")
	if err != nil {
		return
	}
	for _, value := range strings.Fields(string(data)) {
		pid, err := strconv.Atoi(value)
		if err == nil {
			killDescendants(pid)
			_ = unix.Kill(pid, unix.SIGKILL)
		}
	}
}
