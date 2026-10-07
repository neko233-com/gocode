//go:build !windows || !cgo

package main

import "errors"

func runWindowsWorkbenchAcceptance() error {
	return errors.New("Windows workbench acceptance requires Windows amd64 with cgo")
}
