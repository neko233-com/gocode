package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"time"

	ui "github.com/neko233-com/godesktop"
)

// This acceptance uses real disposable files and the production view/input/tab
// model. It captures this process's GPU output; no extension or AI request is
// needed for the visual contract. Other smoke gates validate those services.
func runWorkbenchUIAcceptance() error {
	if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
		return err
	}
	workspace, err := os.MkdirTemp("", "gocode-ui-native-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workspace)
	m, err := newWorkspaceModel(workspace)
	if err != nil {
		return err
	}
	defer m.closeDocuments()
	m.logo, err = loadWorkbenchLogo()
	if err != nil {
		return err
	}
	names := []string{"main.go", "README.md", "阅读说明与备注.md"}
	contents := []string{"package main\nfunc main() {}\n", "# Native Go workbench\n\nImmutable GPU images and complete tab captions.\n", "# 原生桌面\n窗口、字体与标签验收。\n"}
	for i, name := range names {
		path := filepath.Join(workspace, name)
		if err := os.WriteFile(path, []byte(contents[i]), 0600); err != nil {
			return err
		}
		d, err := loadDocument(context.Background(), path)
		if err != nil {
			return err
		}
		m.docs = append(m.docs, d)
		m.files = append(m.files, name)
	}
	m.active, m.showPanel = 1, false
	var closeIcon func()
	defer func() {
		if closeIcon != nil {
			closeIcon()
		}
	}()
	watchdog := time.AfterFunc(30*time.Second, func() { fmt.Fprintln(os.Stderr, "workbench UI acceptance timed out"); os.Exit(2) })
	defer watchdog.Stop()
	phase, waiting, nextFrame := 0, false, uint64(8)
	var failure error
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(workspace), Width: 1280, Height: 820, Background: ui.RGB(editor), CustomTitlebar: true, Input: m.input}, func(cx *ui.Context) *ui.Element {
		m.native = cx
		if closeIcon == nil {
			closeIcon = applyAppIcon("gocode — " + filepath.Base(workspace))
		}
		if !waiting && cx.RenderedFrames() >= nextFrame && phase < 3 {
			stage := []string{"initial-readme", "selected-unicode", "closed-readme"}[phase]
			failure = verifyWorkbenchPixels(cx, m, stage)
			if failure != nil {
				cx.Quit()
			} else if phase < 2 {
				key := "tab-2"
				action := func() { m.active = 2 }
				if phase == 1 {
					key = "close-1"
					action = func() { m.closeTab(1) }
				}
				bounds, ok := cx.ElementBounds(key)
				if !ok {
					failure = fmt.Errorf("missing native control %s", key)
					cx.Quit()
				} else {
					waiting = true
					activateFileWatchControl(cx, workspace, bounds, action, func(err error) {
						failure = err
						phase++
						waiting = false
						nextFrame = cx.RenderedFrames() + 5
						if err != nil {
							cx.Quit()
						}
					})
				}
			} else {
				phase = 3
				cx.Quit()
			}
		}
		cx.Invalidate()
		return m.view(cx)
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	if phase != 3 || len(m.docs) != 2 || filepath.Base(m.current().path) != names[2] {
		return errors.New("native tab select/close result incorrect")
	}
	for i, name := range names {
		body, err := os.ReadFile(filepath.Join(workspace, name))
		if err != nil {
			return fmt.Errorf("UI acceptance changed source %s: %w", name, err)
		}
		if string(body) != contents[i] {
			return fmt.Errorf("UI acceptance changed source bytes: %s", name)
		}
	}
	fmt.Println("Native owned workbench GPU logo, complete measured Latin/Unicode captions, tab selection/close and unchanged source passed")
	return nil
}

func verifyWorkbenchPixels(cx *ui.Context, m *model, stage string) error {
	pixels, scale, err := captureWorkbenchGPU(cx, m.workspace)
	if err != nil {
		return err
	}
	directory := os.Getenv("GOCODE_UI_SCREENSHOTS")
	if directory == "" {
		directory = filepath.Join(".cache", "workbench-ui")
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(directory, stage+".png"))
	if err != nil {
		return err
	}
	err = png.Encode(f, pixels)
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	region := func(x, y, w, h float32) image.Rectangle {
		return image.Rect(int(float64(x)*scale), int(float64(y)*scale), int(float64(x+w)*scale), int(float64(y+h)*scale)).Intersect(pixels.Bounds())
	}
	ink := func(r image.Rectangle, predicate func(uint8, uint8, uint8) bool) int {
		count := 0
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				p := pixels.RGBAAt(x, y)
				if predicate(p.R, p.G, p.B) {
					count++
				}
			}
		}
		return count
	}
	if runtime.GOOS == "windows" {
		blue := ink(region(9, 9, 17, 17), func(r, g, b uint8) bool { return b > 110 && int(b) > int(r)+40 && int(g) > int(r)+20 })
		white := ink(region(9, 9, 17, 17), func(r, g, b uint8) bool { return r > 130 && g > 130 && b > 130 })
		if blue < 3 || white < 2 {
			return fmt.Errorf("%s upstream GPU logo absent: blue=%d white=%d", stage, blue, white)
		}
	}
	for i, d := range m.docs {
		bounds, ok := cx.ElementBounds(fmt.Sprintf("tab-%d", i))
		if !ok {
			return fmt.Errorf("%s tab %d missing", stage, i)
		}
		name := filepath.Base(d.path)
		closeBounds, ok := cx.ElementBounds(fmt.Sprintf("close-%d", i))
		nameWidth, _ := ui.MeasureText(name, 13, "")
		if !ok || closeBounds.X+.01 < bounds.X+33+nameWidth {
			return fmt.Errorf("%s caption %s overlaps native close control", stage, name)
		}
		// Check the final two caption glyphs separately, before the close control.
		// This catches real clipping rather than merely asserting a width formula.
		runes := []rune(name)
		for j := len(runes) - 2; j < len(runes); j++ {
			start, _ := ui.MeasureText(string(runes[:j]), 13, "")
			end, _ := ui.MeasureText(string(runes[:j+1]), 13, "")
			count := ink(region(bounds.X+33+start, bounds.Y+5, end-start, 25), func(r, g, b uint8) bool { return r > 85 && g > 85 && b > 85 })
			if count < 2 {
				return fmt.Errorf("%s tab %s final glyph %q is clipped: ink=%d", stage, name, runes[j], count)
			}
		}
	}
	active, ok := cx.ElementBounds(fmt.Sprintf("tab-%d", m.active))
	if !ok || ink(region(active.X+4, active.Y, active.Width-8, 2), func(r, g, b uint8) bool {
		return r < 40 && g > 80 && b > 160
	}) < 5 {
		return fmt.Errorf("%s selected-tab GPU indicator missing", stage)
	}
	return nil
}
