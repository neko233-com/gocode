package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	ui "github.com/neko233-com/godesktop"
)

func runReplacementAcceptance() error {
	if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
		return err
	}
	root, err := os.MkdirTemp("", "gocode-replace-native-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	a, b := "first line\r\n😀界 needle and NEEDLE\r\n", "needle 😀\n"
	for name, body := range map[string]string{"a.txt": a, "b.txt": b} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			return err
		}
	}
	m, err := newWorkspaceModel(root)
	if err != nil {
		return err
	}
	defer m.closeDocuments()
	m.logo, err = loadWorkbenchLogo()
	if err != nil {
		return err
	}
	d, err := loadDocument(context.Background(), filepath.Join(root, "a.txt"))
	if err != nil {
		return err
	}
	m.docs = []*document{d}
	m.active = 0
	m.editing = true
	m.showPanel = false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stopSearch, stopSave, stopIcon, stopHistory func()
	defer func() {
		if stopHistory != nil {
			stopHistory()
		}
		if stopSearch != nil {
			stopSearch()
		}
		if stopSave != nil {
			stopSave()
		}
		if stopIcon != nil {
			stopIcon()
		}
	}()
	var diagnostic atomic.Value
	diagnostic.Store("not started")
	watchdog := time.AfterFunc(60*time.Second, func() { fmt.Fprintln(os.Stderr, "native replacement timed out:", diagnostic.Load()); os.Exit(2) })
	defer watchdog.Stop()
	phase, waiting, nextFrame := 0, false, uint64(7)
	var failure error
	paintToken := ""
	paintPhase := -1
	var heldX, heldY float32
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(root), Width: 1280, Height: 820, Background: ui.RGB(editor), CustomTitlebar: true, Input: func(cx *ui.Context, e ui.InputEvent) bool {
		if phase >= 19 && e.Kind == ui.KeyPressed {
			fmt.Printf("Native replacement key phase=%d key=%d mods=%d editing=%t current=%t groups=%d undo=%d redo=%d\n", phase, e.Key, e.Modifiers, m.editing, m.current() == d, len(m.history.groups), d.buffer.HistorySnapshot().UndoRevision(), d.buffer.HistorySnapshot().RedoRevision())
		}
		handled := m.input(cx, e)
		if phase >= 19 && e.Kind == ui.KeyPressed {
			fmt.Printf("Native replacement key result handled=%t prompt=%t busy=%t message=%q\n", handled, m.history.prompt != nil, m.history.busy, m.message)
		}
		return handled
	}}, func(cx *ui.Context) *ui.Element {
		m.native = cx
		if stopSearch == nil {
			stopSearch = m.startSearch(ctx, cx.Dispatch, nil)
			stopHistory = m.startHistory(ctx, cx.Dispatch)
			stopSave = m.startSaveActor(ctx, cx.Dispatch)
			stopIcon = applyAppIcon("gocode — " + filepath.Base(root))
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
		click := func(key string, done func(error)) {
			bounds, ok := cx.ElementBounds(key)
			if !ok {
				done(fmt.Errorf("replacement native control missing: %s", key))
				return
			}
			x, y := bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2
			tabsNativePointer(cx, root, x, y, true, func(err error) {
				if err != nil {
					done(err)
					return
				}
				tabsNativePointer(cx, root, x, y, false, done)
			})
		}
		key := func(key, mod int, done func(error)) { tabsNativeKey(cx, root, key, mod, true, done) }
		replaceValue := func(value string, done func(error)) {
			key('H', ui.ModifierControl|ui.ModifierShift, func(err error) {
				if err != nil {
					done(err)
					return
				}
				key('A', ui.ModifierControl, func(err error) {
					if err != nil {
						done(err)
						return
					}
					searchNativeText(cx, root, value, func(err error) {
						if err != nil {
							done(err)
							return
						}
						key(13, 0, done)
					})
				})
			})
		}
		ioDone := func(work func() error, done func(error)) {
			go func() { err := work(); cx.Dispatch(func() { done(err) }) }()
		}
		token := fmt.Sprintf("%d/%d/%s/%s/%s", m.search.generation, m.search.replaceGeneration, m.search.status, m.search.replaceStatus, m.message)
		if token != paintToken {
			paintToken = token
			nextFrame = max(nextFrame, cx.RenderedFrames()+5)
		}
		ready := !m.search.busy && m.search.timer == nil && !m.search.replaceBusy && !m.search.replaceSaving && !m.history.busy
		diagnostic.Store(fmt.Sprintf("phase=%d results=%d focus=%d busy=%t editing=%t current=%t groups=%d undo=%d redo=%d status=%q replacement=%q message=%q", phase, len(m.search.report.Matches), m.search.focus, !ready, m.editing, m.current() == d, len(m.history.groups), d.buffer.HistorySnapshot().UndoRevision(), d.buffer.HistorySnapshot().RedoRevision(), m.search.status, m.search.replaceStatus, m.message))
		if phase != paintPhase {
			paintPhase = phase
			fmt.Println("Native replacement", diagnostic.Load())
		}
		if !waiting && failure == nil && cx.RenderedFrames() >= nextFrame {
			switch phase {
			case 0:
				waiting = true
				searchNativeText(cx, root, "unsaved needle ", advance)
			case 1:
				waiting = true
				key('F', ui.ModifierControl|ui.ModifierShift, advance)
			case 2:
				waiting = true
				searchNativeText(cx, root, "(n)(eedle)", func(err error) {
					if err != nil {
						advance(err)
						return
					}
					key(13, 0, advance)
				})
			case 3:
				if ready {
					waiting = true
					click("search-regex", advance)
				}
			case 4:
				if ready && len(m.search.report.Matches) == 4 {
					waiting = true
					key('H', ui.ModifierControl|ui.ModifierShift, advance)
				}
			case 5:
				waiting = true
				searchNativeText(cx, root, "$2-$1界", func(err error) {
					if err != nil {
						advance(err)
						return
					}
					key(13, 0, advance)
				})
			case 6:
				if ready && m.search.replacePlan != nil {
					if m.search.replacePlan.Count != 4 || d.buffer.Text() != "unsaved needle "+a || len(m.docs) != 1 {
						fail(errors.New("native preview mutated source or missed complete unsaved results"))
						break
					}
					if err := captureReplacementPixels(cx, m, "preview", true); err != nil {
						fail(err)
						break
					}
					waiting = true
					ioDone(func() error {
						data, err := os.ReadFile(filepath.Join(root, "a.txt"))
						if err != nil {
							return err
						}
						if string(data) != a {
							return errors.New("preview wrote original source")
						}
						return os.WriteFile(filepath.Join(root, "b.txt"), []byte("external needle"), 0600)
					}, advance)
				}
			case 7:
				waiting = true
				click("search-replace-all", advance)
			case 8:
				if ready && strings.Contains(m.search.replaceStatus, "Replace aborted") {
					if d.buffer.Text() != "unsaved needle "+a || len(m.docs) != 1 {
						fail(errors.New("disk-stale native preview changed a live document"))
						break
					}
					if err := captureReplacementPixels(cx, m, "stale-preview", true); err != nil {
						fail(err)
						break
					}
					waiting = true
					ioDone(func() error { return os.WriteFile(filepath.Join(root, "b.txt"), []byte(b), 0600) }, advance)
				}
			case 9:
				waiting = true
				click("search-replace-preview", advance)
			case 10:
				if ready && m.search.replacePlan != nil {
					waiting = true
					bounds, ok := cx.ElementBounds("search-replace-all")
					if !ok {
						fail(errors.New("captured replacement button missing"))
						break
					}
					heldX, heldY = bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2
					tabsNativePointer(cx, root, heldX, heldY, true, func(err error) {
						if err != nil {
							advance(err)
							return
						}
						replaceValue("updated", advance)
					})
				}
			case 11:
				if ready && m.search.replacePlan != nil {
					waiting = true
					tabsNativePointer(cx, root, heldX, heldY, false, advance)
				}
			case 12:
				if ready && strings.Contains(m.search.replaceStatus, "changed during click") {
					if d.buffer.Text() != "unsaved needle "+a || len(m.docs) != 1 {
						fail(errors.New("captured old click applied a different preview"))
						break
					}
					if err := captureReplacementPixels(cx, m, "changed-review", true); err != nil {
						fail(err)
						break
					}
					waiting = true
					replaceValue("$2-$1界", advance)
				}
			case 13:
				if ready && m.search.replacePlan != nil {
					waiting = true
					click("search-replace-all", advance)
				}
			case 14:
				if ready && strings.Contains(m.message, "Replaced 4 matches") {
					if len(m.docs) != 2 || d.buffer.Dirty() || m.docs[1].buffer.Dirty() || !strings.Contains(d.buffer.Text(), "unsaved eedle-n界") {
						fail(errors.New("native replacement did not save both actual documents"))
						break
					}
					if err := captureReplacementPixels(cx, m, "saved", false); err != nil {
						fail(err)
						break
					}
					waiting = true
					click("editor-content", func(err error) {
						if err != nil {
							advance(err)
							return
						}
						key('Z', ui.ModifierControl, advance)
					})
				}
			case 15:
				if m.history.prompt != nil {
					if err := captureReplacementPixels(cx, m, "undo-confirmation", false); err != nil {
						fail(err)
						break
					}
					waiting = true
					click("history-cancel", advance)
				}
			case 16:
				if m.history.prompt == nil && !d.buffer.Dirty() && !m.docs[1].buffer.Dirty() {
					waiting = true
					key('Z', ui.ModifierControl, advance)
				}
			case 17:
				if m.history.prompt != nil {
					waiting = true
					click("history-all", advance)
				}
			case 18:
				if ready && d.buffer.Dirty() && d.buffer.Text() == "unsaved needle "+a && m.docs[1].buffer.Dirty() && m.docs[1].buffer.Text() == b {
					if err := captureReplacementPixels(cx, m, "undo", false); err != nil {
						fail(err)
						break
					}
					waiting = true
					key('Z', ui.ModifierControl|ui.ModifierShift, advance)
				}
			case 19:
				if ready && !d.buffer.Dirty() && !m.docs[1].buffer.Dirty() && m.docs[1].buffer.Text() == "eedle-n界 😀\n" {
					if err := captureReplacementPixels(cx, m, "redo", false); err != nil {
						fail(err)
						break
					}
					waiting = true
					key('Z', ui.ModifierControl, advance)
				}
			case 20:
				if m.history.prompt != nil {
					waiting = true
					click("history-one", advance)
				}
			case 21:
				if ready && d.buffer.Dirty() && d.buffer.Text() == "unsaved needle "+a && !m.docs[1].buffer.Dirty() && m.docs[1].buffer.Text() == "eedle-n界 😀\n" && len(m.history.groups) == 0 {
					if err := captureReplacementPixels(cx, m, "undo-one", false); err != nil {
						fail(err)
						break
					}
					phase = 22
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
	if phase != 22 {
		return fmt.Errorf("native replacement incomplete phase=%d status=%s message=%s", phase, m.search.replaceStatus, m.message)
	}
	wantA := strings.ReplaceAll(strings.ReplaceAll("unsaved needle "+a, "needle", "eedle-n界"), "NEEDLE", "EEDLE-N界")
	for name, want := range map[string]string{"a.txt": wantA, "b.txt": "eedle-n界 😀\n"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return err
		}
		if string(data) != want {
			return fmt.Errorf("actual replacement bytes/EOL differ in %s: %q", name, data)
		}
	}
	fmt.Println("Native replacement passed: actual unsaved/closed files, regex captures, preview pixels, disk-stale/captured-review rejection, background saves, UTF-16/CRLF and native grouped undo/cancel/redo/current-file split")
	return nil
}

func captureReplacementPixels(cx *ui.Context, m *model, stage string, preview bool) error {
	pixels, scale, err := captureWorkbenchGPU(cx, m.workspace)
	if err != nil {
		return err
	}
	red, green, ink, accentPixels := 0, 0, 0, 0
	var validationErr error
	if stage == "undo-confirmation" {
		bounds, ok := cx.ElementBounds("history-all")
		if !ok {
			return errors.New("workspace undo confirmation missing")
		}
		for y := int(float64(bounds.Y) * scale); y < min(pixels.Bounds().Dy(), int(float64(bounds.Y+bounds.Height)*scale)); y++ {
			for x := int(float64(bounds.X) * scale); x < min(pixels.Bounds().Dx(), int(float64(bounds.X+bounds.Width)*scale)); x++ {
				p := pixels.RGBAAt(x, y)
				// At 100% DPI, hinted 13px gray text blends with the blue fill:
				// high coverage in all RGB channels is not guaranteed. Require
				// visible gray-on-blue glyph contrast; the fill alone has R=0.
				if p.R >= 48 && p.G >= 135 && p.B >= 190 && p.B <= 220 {
					ink++
				}
				if p.R == uint8(accent>>16) && p.G == uint8((accent>>8)&255) && p.B == uint8(accent&255) {
					accentPixels++
				}
			}
		}
		if ink < 24 || accentPixels < 24 {
			validationErr = fmt.Errorf("undo confirmation lacks completed GPU text/button: ink=%d accent=%d", ink, accentPixels)
		}
	}
	if preview {
		bounds, ok := cx.ElementBounds("search-results")
		if !ok {
			return errors.New("replace preview viewport missing")
		}
		for y := int(float64(bounds.Y) * scale); y < min(pixels.Bounds().Dy(), int(float64(bounds.Y+bounds.Height)*scale)); y++ {
			for x := int(float64(bounds.X) * scale); x < min(pixels.Bounds().Dx(), int(float64(bounds.X+bounds.Width)*scale)); x++ {
				p := pixels.RGBAAt(x, y)
				if p.R == 0x52 && p.G == 0x22 && p.B == 0x22 {
					red++
				}
				if p.R == 0x24 && p.G == 0x4b && p.B == 0x2a {
					green++
				}
			}
		}
		if red < 24 || green < 24 {
			validationErr = fmt.Errorf("replace preview lacks completed before/after GPU pixels: red=%d green=%d", red, green)
		}
	}
	directory := os.Getenv("GOCODE_REPLACE_SCREENSHOTS")
	if directory == "" {
		directory = filepath.Join(".cache", "replace-native")
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(directory, stage+".png"))
	if err != nil {
		return err
	}
	if err := errors.Join(png.Encode(f, pixels), f.Close()); err != nil {
		return err
	}
	data, err := json.MarshalIndent(struct {
		Stage                                  string
		Width, Height, Red, Green, Ink, Accent int
		Scale                                  float64
		Status, Message                        string
	}{stage, pixels.Bounds().Dx(), pixels.Bounds().Dy(), red, green, ink, accentPixels, scale, m.search.replaceStatus, m.message}, "", "  ")
	if err != nil {
		return err
	}
	return errors.Join(os.WriteFile(filepath.Join(directory, stage+".json"), data, 0600), validationErr)
}
