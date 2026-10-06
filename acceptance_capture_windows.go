//go:build windows && cgo

package main

import (
	"image/png"
	"os"
	"path/filepath"

	"github.com/neko233-com/godesktop/testing/winprobe"
)

func captureCopilotAcceptance(workspace string) error {
	file := os.Getenv("GOCODE_AI_SCREENSHOT")
	return captureAcceptance(workspace, file)
}
func captureLargefileAcceptance(workspace string) error {
	return captureAcceptance(workspace, os.Getenv("GOCODE_LARGEFILE_SCREENSHOT"))
}
func captureLSPAcceptance(workspace string) error {
	return captureAcceptance(workspace, os.Getenv("GOCODE_LSP_SCREENSHOT"))
}
func captureAcceptance(workspace, file string) error {
	if file == "" {
		return nil
	}
	window, err := winprobe.Find("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
	if err != nil {
		return err
	}
	pixels, err := window.Capture()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, pixels)
}
