//go:build windows && cgo

package main

import (
	"image"
	"os"
	"path/filepath"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

func captureWorkbenchGPU(_ *ui.Context, workspace string) (*image.RGBA, float64, error) {
	w, err := winprobe.Find("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
	if err != nil {
		return nil, 0, err
	}
	pixels, err := w.Capture()
	return pixels, float64(w.DPI()) / 96, err
}
