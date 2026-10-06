package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	ui "github.com/neko233-com/godesktop"
)

// Real owned-window input exercises the production tab model and GPU view.
// No user workspace, global input, extension service or paid request is used.
func runEditorTabsAcceptance() error {
	if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
		return err
	}
	workspace, err := os.MkdirTemp("", "gocode-tabs-native-")
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
	contents := "# Native overflow acceptance\n" + strings.Repeat("unchanged native editor source\n", 80)
	for i := 0; i < 40; i++ {
		name := fmt.Sprintf("document-%02d.md", i)
		if i%3 == 0 {
			name = fmt.Sprintf("阅读说明-%02d.md", i)
		}
		path := filepath.Join(workspace, name)
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			return err
		}
		d, err := loadDocument(context.Background(), path)
		if err != nil {
			return err
		}
		m.docs = append(m.docs, d)
		m.files = append(m.files, name)
	}
	original := append([]*document(nil), m.docs...)
	reopened, err := loadDocument(context.Background(), original[38].path)
	if err != nil {
		return err
	}
	m.active, m.showPanel = 39, false
	var closeIcon func()
	defer func() {
		if closeIcon != nil {
			closeIcon()
		}
	}()
	watchdog := time.AfterFunc(40*time.Second, func() { fmt.Fprintln(os.Stderr, "native editor tabs acceptance timed out"); os.Exit(2) })
	defer watchdog.Stop()
	phase, waiting, nextFrame := 0, false, uint64(8)
	var failure error
	var releaseX, releaseY, beforeWheel float32
	stages := []string{"last-revealed", "first-wrapped", "last-wrapped", "manual-wheel", "dragged-end", "reopened-pressed", "stable-release", "mru-first", "mru-second", "control-released"}
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(workspace), Width: 1280, Height: 820, Background: ui.RGB(editor), CustomTitlebar: true, Input: m.input}, func(cx *ui.Context) *ui.Element {
		m.native = cx
		if closeIcon == nil {
			closeIcon = applyAppIcon("gocode — " + filepath.Base(workspace))
		}
		fail := func(err error) { failure = err; cx.Quit() }
		advance := func(err error) {
			if err != nil {
				fail(err)
				return
			}
			phase++
			waiting = false
			nextFrame = cx.RenderedFrames() + 5
			cx.Invalidate()
		}
		if failure == nil && !waiting && phase < len(stages) && cx.RenderedFrames() >= nextFrame {
			if err := verifyEditorTabsPixels(cx, m, stages[phase]); err != nil {
				fail(err)
			} else {
				waiting = true
				switch phase {
				case 0:
					if m.current() != original[39] {
						fail(errors.New("initial last tab not selected"))
						break
					}
					if _, ok := cx.ElementBounds(m.tabKey(original[0])); ok {
						fail(errors.New("offscreen first tab has a hit target"))
						break
					}
					tabsNativeKey(cx, workspace, 34, ui.ModifierControl, true, advance)
				case 1:
					if m.current() != original[0] || m.tabs.offset != 0 {
						fail(errors.New("native PageDown did not wrap/reveal first tab"))
						break
					}
					tabsNativeKey(cx, workspace, 33, ui.ModifierControl, true, advance)
				case 2:
					if m.current() != original[39] {
						fail(errors.New("native PageUp did not wrap/reveal last tab"))
						break
					}
					b, ok := cx.ElementBounds("editor-tabs")
					if !ok {
						fail(errors.New("missing tab viewport"))
						break
					}
					original[39].scroll = 4
					beforeWheel = m.tabs.offset
					tabsNativeWheel(cx, workspace, b.X+b.Width/2, b.Y+12, advance)
				case 3:
					if m.current() != original[39] || original[39].scroll != 4 || m.tabs.offset >= beforeWheel-1 {
						fail(errors.New("native tab wheel failed or scrolled the document"))
						break
					}
					thumb, ok := cx.ElementBounds("editor-tabs-thumb")
					track, trackOK := cx.ElementBounds("editor-tabs-track")
					if !ok || !trackOK {
						fail(errors.New("missing native scrollbar controls"))
						break
					}
					tabsNativeDrag(cx, workspace, thumb, ui.Bounds{X: track.X + track.Width - 1}, advance)
				case 4:
					if m.tabs.dragging || m.tabs.offset < m.tabs.total-m.tabs.viewport-1 {
						fail(errors.New("native thumb drag did not reach the end"))
						break
					}
					b, ok := cx.ElementBounds(m.tabCloseKey(original[38]))
					if !ok {
						fail(errors.New("identity-test close control is offscreen"))
						break
					}
					releaseX, releaseY = b.X+b.Width/2, b.Y+b.Height/2
					tabsNativePointer(cx, workspace, releaseX, releaseY, true, func(err error) {
						if err != nil {
							advance(err)
							return
						}
						// Replace the pressed document with a new instance of the same
						// path/index before releasing. Captured input must retain identity.
						m.removeTab(38)
						m.docs = append(m.docs, nil)
						copy(m.docs[39:], m.docs[38:])
						m.docs[38] = reopened
						m.focusTab(original[39])
						advance(nil)
					})
				case 5:
					if m.docs[38] != reopened {
						fail(errors.New("pressed target was not replaced before release"))
						break
					}
					tabsNativePointer(cx, workspace, releaseX, releaseY, false, advance)
				case 6:
					if len(m.docs) != 40 || m.docs[38] != reopened || m.current() != original[39] {
						fail(errors.New("captured release closed a replacement or adjacent tab"))
						break
					}
					m.focusTab(original[2])
					m.focusTab(original[19])
					m.focusTab(original[39])
					tabsNativeKey(cx, workspace, 9, ui.ModifierControl, true, advance)
				case 7:
					if m.current() != original[19] {
						fail(errors.New("first native held-Control Tab did not select MRU"))
						break
					}
					tabsNativeKey(cx, workspace, 9, ui.ModifierControl, true, advance)
				case 8:
					if m.current() != original[2] {
						fail(errors.New("second native held-Control Tab oscillated"))
						break
					}
					tabsNativeKey(cx, workspace, 17, 0, false, advance)
				case 9:
					if m.tabs.switchOrder != nil {
						fail(errors.New("native Control release retained the switch order"))
						break
					}
					advance(nil)
					cx.Quit()
				}
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
	if phase != len(stages) {
		return fmt.Errorf("incomplete native tab acceptance: phase %d", phase)
	}
	for _, d := range original {
		body, err := os.ReadFile(d.path)
		if err != nil {
			return err
		}
		if string(body) != contents {
			return fmt.Errorf("native tabs changed source: %s", filepath.Base(d.path))
		}
	}
	fmt.Println("Native 40-tab clipping, wheel/drag routing, stable captured identity, ordered/MRU navigation and Control release passed; all source bytes unchanged")
	return nil
}

func verifyEditorTabsPixels(cx *ui.Context, m *model, stage string) error {
	pixels, scale, err := captureWorkbenchGPU(cx, m.workspace)
	if err != nil {
		return err
	}
	directory := os.Getenv("GOCODE_TABS_SCREENSHOTS")
	if directory == "" {
		directory = filepath.Join(".cache", "editor-tabs")
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(directory, stage+".png"))
	if err != nil {
		return err
	}
	err = errors.Join(png.Encode(f, pixels), f.Close())
	if err != nil {
		return err
	}
	viewport, ok := cx.ElementBounds("editor-tabs")
	if !ok {
		return errors.New("visible native tab viewport missing")
	}
	visible := 0
	for _, d := range m.docs {
		if b, ok := cx.ElementBounds(m.tabKey(d)); ok {
			visible++
			if b.X < viewport.X-.01 || b.X+b.Width > viewport.X+viewport.Width+.01 {
				return fmt.Errorf("%s tab hit rectangle escapes clip", stage)
			}
		}
	}
	if visible == 0 || visible > 12 {
		return fmt.Errorf("%s unexpected visible target count: %d", stage, visible)
	}
	count := func(b ui.Bounds, match func(byte, byte, byte) bool) int {
		r := image.Rect(int(float64(b.X)*scale), int(float64(b.Y)*scale), int(float64(b.X+b.Width)*scale), int(float64(b.Y+b.Height)*scale)).Intersect(pixels.Bounds())
		n := 0
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				p := pixels.RGBAAt(x, y)
				if match(p.R, p.G, p.B) {
					n++
				}
			}
		}
		return n
	}
	active, ok := cx.ElementBounds(m.tabKey(m.current()))
	if !ok || count(ui.Bounds{X: active.X, Y: active.Y, Width: active.Width, Height: 2}, func(r, g, b byte) bool { return r < 40 && g > 80 && b > 160 }) < 5 {
		return fmt.Errorf("%s native selected-tab pixels absent", stage)
	}
	thumb, ok := cx.ElementBounds("editor-tabs-thumb")
	if !ok || count(thumb, func(r, g, b byte) bool { return r == 85 && g == 85 && b == 85 }) < 5 {
		return fmt.Errorf("%s native scrollbar pixels absent", stage)
	}
	report := struct {
		Stage                                       string
		Documents, Visible, PixelWidth, PixelHeight int
		Scale                                       float64
		Offset, Viewport, Total                     float32
	}{stage, len(m.docs), visible, pixels.Bounds().Dx(), pixels.Bounds().Dy(), scale, m.tabs.offset, m.tabs.viewport, m.tabs.total}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, stage+".json"), append(body, '\n'), 0600)
}
