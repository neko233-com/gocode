package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	ui "github.com/neko233-com/godesktop"
)

func captureTerminalAcceptance(cx *ui.Context, workspace, file string, bounds ui.Bounds) error {
	pixels, scale, err := captureWorkbenchGPU(cx, workspace)
	if err != nil {
		return err
	}
	colors := map[uint32]int{}
	intrinsic := 0
	region := image.Rect(int(math.Floor(float64(bounds.X)*scale)), int(math.Floor(float64(bounds.Y)*scale)), int(math.Ceil(float64(bounds.X+bounds.Width)*scale)), int(math.Ceil(float64(bounds.Y+bounds.Height)*scale))).Intersect(pixels.Bounds())
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			p := pixels.RGBAAt(x, y)
			value := uint32(p.R)<<16 | uint32(p.G)<<8 | uint32(p.B)
			for _, fg := range []uint32{0xdcdcaa, 0xce9178, 0xe5c07b} {
				if matchesTerminalInk(value, 0x181818, fg) {
					colors[fg]++
				}
			}
			if p.R > 150 && p.G > 110 && p.B < 70 {
				intrinsic++
			}
		}
	}
	if colors[0xdcdcaa] < 5 || colors[0xce9178] < 5 {
		return fmt.Errorf("%w: command=%d string=%d", errTerminalPixelsPending, colors[0xdcdcaa], colors[0xce9178])
	}
	if strings.Contains(filepath.Base(file), "output") {
		if colors[0xe5c07b] < 5 {
			return fmt.Errorf("%w: native ANSI truecolor missing", errTerminalPixelsPending)
		}
		if runtime.GOOS == "darwin" && intrinsic < 8 {
			return fmt.Errorf("%w: native PTY emoji color missing: %d pixels", errTerminalPixelsPending, intrinsic)
		}
	}
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	if err := errors.Join(png.Encode(f, pixels), f.Close()); err != nil {
		return err
	}
	data, err := json.MarshalIndent(struct {
		Width, Height int
		Scale         float64
		Region        image.Rectangle
		Colors        map[uint32]int
		ColorGlyph    int
	}{pixels.Bounds().Dx(), pixels.Bounds().Dy(), scale, region, colors, intrinsic}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(strings.TrimSuffix(file, filepath.Ext(file))+".json", append(data, '\n'), 0600)
}
