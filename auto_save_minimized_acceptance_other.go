//go:build !windows || !cgo

package main

import "errors"

func runMinimizedAutoSaveAcceptance() error {
	return errors.New("minimized Auto Save native acceptance requires Windows amd64 with cgo")
}
