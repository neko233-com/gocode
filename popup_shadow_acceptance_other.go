//go:build !windows || !cgo

package main

import "fmt"

func runPopupShadowAcceptance() error {
	return fmt.Errorf("native popup shadow acceptance requires Windows and cgo")
}
