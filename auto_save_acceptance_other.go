//go:build !windows || !cgo

package main

import "errors"

func runAutoSaveAcceptance() error {
	return errors.New("Auto Save native acceptance requires Windows amd64 with cgo")
}
