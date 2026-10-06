//go:build linux

package terminal

import (
	"os"
	"strconv"
	"strings"
)

func processRunning(pid int) bool {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	i := strings.LastIndex(string(data), ") ")
	return i < 0 || len(data) <= i+2 || data[i+2] != 'Z'
}
