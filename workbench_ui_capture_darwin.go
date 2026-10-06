//go:build darwin && cgo

package main

import (
	"image"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/metalprobe"
)

func captureWorkbenchGPU(cx *ui.Context, _ string) (*image.RGBA, float64, error) {
	pixels, _, err := metalprobe.Snapshot()
	if err != nil {
		return nil, 0, err
	}
	width, _ := cx.WindowSize()
	return pixels, float64(pixels.Bounds().Dx()) / float64(width), nil
}
