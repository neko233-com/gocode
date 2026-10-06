//go:build windows && cgo

package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"

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
func captureTerminalAcceptance(workspace, file string) error {
	window, err := winprobe.Find("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
	if err != nil {
		return err
	}
	pixels, err := window.Capture()
	if err != nil {
		return err
	}
	colors := map[uint32]int{}
	for y := pixels.Bounds().Dy() / 2; y < pixels.Bounds().Dy(); y++ {
		for x := pixels.Bounds().Dx() / 4; x < pixels.Bounds().Dx(); x++ {
			r, g, b, _ := pixels.At(x, y).RGBA()
			colors[uint32(r>>8)<<16|uint32(g>>8)<<8|uint32(b>>8)]++
		}
	}
	if colors[0xdcdcaa] < 5 || colors[0xce9178] < 5 {
		return fmt.Errorf("native shell syntax colors missing from GPU: command=%d string=%d", colors[0xdcdcaa], colors[0xce9178])
	}
	if strings.Contains(filepath.Base(file), "output") && colors[0xe5c07b] < 5 {
		return fmt.Errorf("native ANSI truecolor missing from GPU")
	}
	return captureAcceptance(workspace, file)
}
func resizeTerminalAcceptance(workspace string) (float32, error) {
	window, err := winprobe.Find("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
	if err != nil {
		return 0, err
	}
	if err := window.Resize(1040, 760); err != nil {
		return 0, err
	}
	width, _, err := window.ClientSize()
	return float32(width) * 96 / float32(window.DPI()), err
}
func captureCloseAcceptance(workspace, mode string) error {
	directory := os.Getenv("GOCODE_CLOSE_SCREENSHOTS")
	if directory == "" {
		return nil
	}
	return captureAcceptance(workspace, filepath.Join(directory, "smoke-"+mode+".png"))
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
