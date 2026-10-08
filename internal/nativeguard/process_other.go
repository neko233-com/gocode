//go:build !windows && !darwin && !linux

package nativeguard

import (
	"errors"
	"os"
)

func startProcess(string, []string, string, []string, *os.File, *os.File, *os.File) (process, error) {
	return nil, errors.New("native guard process ownership is unsupported on this platform")
}
