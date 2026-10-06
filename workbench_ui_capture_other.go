//go:build (!windows && !darwin) || !cgo

package main

import (
	"errors"
	"image"

	ui "github.com/neko233-com/godesktop"
)

func captureWorkbenchGPU(*ui.Context, string) (*image.RGBA, float64, error) {
	return nil, 0, errors.New("native GPU UI acceptance requires Windows/macOS with cgo")
}
