//go:build (!windows && !darwin) || !cgo

package main

import (
	"errors"
	ui "github.com/neko233-com/godesktop"
)

func tabsNativeKey(_ *ui.Context, _ string, _, _ int, _ bool, done func(error)) {
	done(errors.New("native tabs require Windows/macOS and cgo"))
}
func tabsNativeWheel(_ *ui.Context, _ string, _, _ float32, done func(error)) {
	done(errors.New("native tabs require Windows/macOS and cgo"))
}
func tabsNativeDrag(_ *ui.Context, _ string, _, _ ui.Bounds, done func(error)) {
	done(errors.New("native tabs require Windows/macOS and cgo"))
}
func tabsNativePointer(_ *ui.Context, _ string, _, _ float32, _ bool, done func(error)) {
	done(errors.New("native tabs require Windows/macOS and cgo"))
}
