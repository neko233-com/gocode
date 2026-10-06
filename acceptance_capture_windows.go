//go:build windows && cgo

package main

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

func captureCopilotAcceptance(workspace string) error {
	file := os.Getenv("GOCODE_AI_SCREENSHOT")
	return captureAcceptance(workspace, file)
}
func inputDuringOpen(cx *ui.Context, workspace string, _ func(), done func(error)) {
	go func() {
		window, err := winprobe.Find("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
		if err == nil {
			for _, r := range "// responsive " {
				if err = window.Send(0x102, uintptr(r), 0); err != nil {
					break
				}
			}
		}
		if err == nil {
			_, err = resizeTerminalAcceptance(workspace)
		}
		cx.Dispatch(func() { done(err) })
	}()
}
func captureOpenAcceptance(workspace, stage string, bounds ui.Bounds) error {
	window, err := winprobe.Find("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
	if err != nil {
		return err
	}
	pixels, err := window.Capture()
	if err != nil {
		return err
	}
	scale := float64(window.DPI()) / 96
	bounds.X += 68
	bounds.Width = min(bounds.Width-68, 240)
	region := image.Rect(int(float64(bounds.X)*scale), int(float64(bounds.Y)*scale), int(float64(bounds.X+bounds.Width)*scale), int(float64(bounds.Y+bounds.Height)*scale)).Intersect(pixels.Bounds())
	ink := 0
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			r, g, b, _ := pixels.At(x, y).RGBA()
			if stage == "pending-read" && matchesTerminalInk(uint32(r>>8)<<16|uint32(g>>8)<<8|uint32(b>>8), editor, 0x6a9955) || stage == "awaited-vsix" && r>>8 > 120 && g>>8 > 120 && b>>8 > 120 {
				ink++
			}
		}
	}
	if ink < 30 {
		return fmt.Errorf("%w: %s ink=%d", errOpenPixelsPending, stage, ink)
	}
	if dir := os.Getenv("GOCODE_OPEN_SCREENSHOTS"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		f, err := os.Create(filepath.Join(dir, stage+".png"))
		if err != nil {
			return err
		}
		defer f.Close()
		return png.Encode(f, pixels)
	}
	return nil
}
func activateFileWatchControl(cx *ui.Context, workspace string, bounds ui.Bounds, _ func(), done func(error)) {
	go func() {
		window, err := winprobe.Find("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
		if err == nil {
			err = window.Pointer(0x201, int(bounds.X+bounds.Width/2), int(bounds.Y+bounds.Height/2))
		}
		if err == nil {
			err = window.Pointer(0x202, int(bounds.X+bounds.Width/2), int(bounds.Y+bounds.Height/2))
		}
		cx.Dispatch(func() { done(err) })
	}()
}
func captureFileWatchAcceptance(workspace, stage string, suffix, control ui.Bounds) error {
	window, err := winprobe.Find("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
	if err != nil {
		return err
	}
	pixels, err := window.Capture()
	if err != nil {
		return err
	}
	scale := float64(window.DPI()) / 96
	region := func(b ui.Bounds) image.Rectangle {
		return image.Rect(int(math.Floor(float64(b.X)*scale)), int(math.Floor(float64(b.Y)*scale)), int(math.Ceil(float64(b.X+b.Width)*scale)), int(math.Ceil(float64(b.Y+b.Height)*scale))).Intersect(pixels.Bounds())
	}
	yellow, background := 0, 0
	for y := region(suffix).Min.Y; y < region(suffix).Max.Y; y++ {
		for x := region(suffix).Min.X; x < region(suffix).Max.X; x++ {
			r, g, b, _ := pixels.At(x, y).RGBA()
			value := uint32(r>>8)<<16 | uint32(g>>8)<<8 | uint32(b>>8)
			if matchesTerminalInk(value, 0x1f1f1f, 0xdcdcaa) {
				yellow++
			}
		}
	}
	wanted := uint32(0x332b00)
	if stage == "reload-dialog" {
		wanted = accent
	}
	if stage == "confirmed-reload" {
		control.X += control.Width * .5
		control.Width *= .5
	}
	for y := region(control).Min.Y; y < region(control).Max.Y; y++ {
		for x := region(control).Min.X; x < region(control).Max.X; x++ {
			r, g, b, _ := pixels.At(x, y).RGBA()
			if stage == "confirmed-reload" && r>>8 > 100 && g>>8 > 100 && b>>8 > 100 || stage != "confirmed-reload" && uint32(r>>8)<<16|uint32(g>>8)<<8|uint32(b>>8) == wanted {
				background++
			}
		}
	}
	// A dialog shades the source text; its real confirmation-button pixels are
	// the gate. Final/clean captures require function ink beyond the prior name.
	if stage != "reload-dialog" && yellow < 20 || control.Width > 0 && background < 100 {
		return fmt.Errorf("%w: %s ink=%d control=%d", errWatchPixelsPending, stage, yellow, background)
	}
	directory := os.Getenv("GOCODE_FILEWATCH_SCREENSHOTS")
	if directory == "" {
		return nil
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(directory, stage+".png"))
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, pixels)
}
func captureLargefileAcceptance(workspace string) error {
	return captureAcceptance(workspace, os.Getenv("GOCODE_LARGEFILE_SCREENSHOT"))
}
func captureLSPAcceptance(workspace string) error {
	return captureAcceptance(workspace, os.Getenv("GOCODE_LSP_SCREENSHOT"))
}
func captureRecoveredLSPAcceptance(workspace string, suffix, problem ui.Bounds) error {
	window, err := winprobe.Find("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
	if err != nil {
		return err
	}
	pixels, err := window.Capture()
	if err != nil {
		return err
	}
	scale := float64(window.DPI()) / 96
	region := func(b ui.Bounds) image.Rectangle {
		return image.Rect(int(math.Floor(float64(b.X)*scale)), int(math.Floor(float64(b.Y)*scale)), int(math.Ceil(float64(b.X+b.Width)*scale)), int(math.Ceil(float64(b.Y+b.Height)*scale))).Intersect(pixels.Bounds())
	}
	yellow, diagnostic := 0, 0
	for y := region(suffix).Min.Y; y < region(suffix).Max.Y; y++ {
		for x := region(suffix).Min.X; x < region(suffix).Max.X; x++ {
			r, g, b, _ := pixels.At(x, y).RGBA()
			if matchesTerminalInk(uint32(r>>8)<<16|uint32(g>>8)<<8|uint32(b>>8), 0x282828, 0xdcdcaa) {
				yellow++
			}
		}
	}
	// The prior hover Output is left-aligned. The final Problems button paints
	// its diagnostic in the middle/right of the row; an old frame has no ink here.
	problem.X += problem.Width * .5
	problem.Width *= .5
	for y := region(problem).Min.Y; y < region(problem).Max.Y; y++ {
		for x := region(problem).Min.X; x < region(problem).Max.X; x++ {
			r, g, b, _ := pixels.At(x, y).RGBA()
			if r>>8 > 100 && g>>8 > 100 && b>>8 > 100 {
				diagnostic++
			}
		}
	}
	if yellow < 5 || diagnostic < 20 {
		return fmt.Errorf("%w: completed-suffix=%d diagnostic=%d", errLSPPixelsPending, yellow, diagnostic)
	}
	file := os.Getenv("GOCODE_LSP_SCREENSHOT")
	if file == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, pixels)
}
func captureTerminalAcceptance(workspace, file string, bounds ui.Bounds) error {
	window, err := winprobe.Find("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
	if err != nil {
		return err
	}
	pixels, err := window.Capture()
	if err != nil {
		return err
	}
	colors := map[uint32]int{}
	scale := float64(window.DPI()) / 96
	region := image.Rect(int(math.Floor(float64(bounds.X)*scale)), int(math.Floor(float64(bounds.Y)*scale)), int(math.Ceil(float64(bounds.X+bounds.Width)*scale)), int(math.Ceil(float64(bounds.Y+bounds.Height)*scale))).Intersect(pixels.Bounds())
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			r, g, b, _ := pixels.At(x, y).RGBA()
			value := uint32(r>>8)<<16 | uint32(g>>8)<<8 | uint32(b>>8)
			for _, fg := range []uint32{0xdcdcaa, 0xce9178, 0xe5c07b} {
				if matchesTerminalInk(value, 0x181818, fg) {
					colors[fg]++
				}
			}
		}
	}
	if colors[0xdcdcaa] < 5 || colors[0xce9178] < 5 {
		return fmt.Errorf("%w: command=%d string=%d", errTerminalPixelsPending, colors[0xdcdcaa], colors[0xce9178])
	}
	if strings.Contains(filepath.Base(file), "output") && colors[0xe5c07b] < 5 {
		return fmt.Errorf("%w: native ANSI truecolor missing", errTerminalPixelsPending)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, pixels)
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
