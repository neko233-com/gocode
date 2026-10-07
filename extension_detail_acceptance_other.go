//go:build !windows || !cgo

package main

import "errors"

func runExtensionDetailAcceptance() error {
	return errors.New("extension detail acceptance requires Windows amd64 with cgo")
}
