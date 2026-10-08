//go:build !windows && !darwin && !linux

package languageserver

import (
	"errors"
	"os"
)

func startServerProcess(string, []string, string, []string, *os.File, *os.File, *os.File) (process, error) {
	return nil, errors.New("language server process ownership is unsupported on this platform")
}
