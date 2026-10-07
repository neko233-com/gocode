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
	textbuffer "github.com/neko233-com/godesktop/editor"
)

func runEditorGroupsAcceptance() error {
	if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
		return err
	}
	root, err := os.MkdirTemp("", "gocode-groups-native-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	var source strings.Builder
	source.WriteString("package main\r\n")
	for i := range 140 {
		fmt.Fprintf(&source, "var item%d = \"shared 😀界\"\r\n", i)
	}
	for _, name := range []string{"main.go", "other.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source.String()), 0600); err != nil {
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
	m.showPanel = false
	m.editing = true
	d, err := loadDocument(context.Background(), filepath.Join(root, "main.go"))
	if err != nil {
		return err
	}
	m.docs = []*document{d}
	m.active = 0
	m.files = []string{"main.go", "other.go"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stopOpen, stopSave, stopIcon func()
	defer func() {
		for _, stop := range []func(){stopOpen, stopSave, stopIcon} {
			if stop != nil {
				stop()
			}
		}
	}()
	var diagnostic atomic.Value
	diagnostic.Store("starting")
	watchdog := time.AfterFunc(60*time.Second, func() { fmt.Fprintln(os.Stderr, "native groups timeout:", diagnostic.Load()); os.Exit(2) })
	defer watchdog.Stop()
	phase, waiting, nextFrame := 0, false, uint64(8)
	var failure error
	var primarySelection textbuffer.Selection
	var heldX, heldY float32
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(root), Width: 1280, Height: 820, CustomTitlebar: true, Background: ui.RGB(editor), Input: m.input}, func(cx *ui.Context) *ui.Element {
		m.native = cx
		if stopOpen == nil {
			stopOpen = m.startFileOpens(ctx, cx.Dispatch, nil)
			stopSave = m.startDocumentSaves(ctx, cx)
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
			nextFrame = cx.RenderedFrames() + 6
			cx.Invalidate()
		}
		click := func(key string, done func(error)) {
			b, ok := cx.ElementBounds(key)
			if !ok {
				done(fmt.Errorf("group control missing: %s", key))
				return
			}
			x, y := b.X+b.Width/2, b.Y+b.Height/2
			tabsNativePointer(cx, root, x, y, true, func(err error) {
				if err != nil {
					done(err)
					return
				}
				tabsNativePointer(cx, root, x, y, false, done)
			})
		}
		key := func(k, mods int) { waiting = true; tabsNativeKey(cx, root, k, mods, true, advance) }
		diagnostic.Store(fmt.Sprintf("phase=%d groups=%d active=%d current=%s close=%t open=%t", phase, len(m.allGroups()), m.groups.active, m.groupBreadcrumb(m.current()), m.closePrompt, m.openBusy))
		if !waiting && failure == nil && cx.RenderedFrames() >= nextFrame {
			fmt.Println("Native groups", diagnostic.Load())
			switch phase {
			case 0:
				waiting = true
				primarySelection = d.buffer.Selection()
				click("split", advance)
			case 1:
				if len(m.allGroups()) != 2 || len(m.docs) != 1 || m.current() != d {
					fail(errors.New("native split duplicated document or lost focus"))
					break
				}
				if err := captureGroupsPixels(cx, m, "shared"); err != nil {
					fail(err)
					break
				}
				waiting = true
				click("group-2-code-line-1", advance)
			case 2:
				waiting = true
				searchNativeText(cx, root, "right界", advance)
			case 3:
				if !strings.Contains(d.buffer.Text(), "right界") {
					fail(errors.New("right view native edit missing"))
					break
				}
				key('1', ui.ModifierControl)
			case 4:
				if d.buffer.Selection() != primarySelection {
					fail(errors.New("switch restored another view caret"))
					break
				}
				waiting = true
				click("code-line-10", advance)
			case 5:
				waiting = true
				searchNativeText(cx, root, "left😀", advance)
			case 6:
				key('2', ui.ModifierControl)
			case 7:
				if d.buffer.Selection().Active.Line != 1 || !strings.Contains(d.buffer.Text(), "left😀") {
					fail(errors.New("shared edit or independent second caret lost"))
					break
				}
				key(35, ui.ModifierControl)
			case 8:
				if d.scroll == 0 || m.findGroup(1).views[d].scroll != 0 {
					fail(errors.New("large editor scroll leaked between groups"))
					break
				}
				if err := captureGroupsPixels(cx, m, "independent"); err != nil {
					fail(err)
					break
				}
				b, ok := cx.ElementBounds("editor-sash-1")
				outer, rootOK := cx.ElementBounds("editor-split-1")
				if !ok || !rootOK {
					fail(errors.New("native split sash missing"))
					break
				}
				waiting = true
				tabsNativeDrag(cx, root, b, ui.Bounds{X: outer.X + outer.Width*.68}, advance)
			case 9:
				if m.groups.root.ratio < .6 {
					fail(errors.New("native sash drag did not resize groups"))
					break
				}
				if err := captureGroupsPixels(cx, m, "dragged"); err != nil {
					fail(err)
					break
				}
				waiting = true
				click("group-2-split-down", advance)
			case 10:
				if len(m.allGroups()) != 3 || !m.groups.root.right.down {
					fail(errors.New("native nested down split missing"))
					break
				}
				if err := captureGroupsPixels(cx, m, "nested"); err != nil {
					fail(err)
					break
				}
				waiting = true
				click("file-other.go", advance)
			case 11:
				if m.openBusy || m.current() == nil || filepath.Base(m.current().path) != "other.go" {
					cx.Invalidate()
					break
				}
				waiting = true
				searchNativeText(cx, root, "dirty ", advance)
			case 12:
				waiting = true
				b, ok := cx.ElementBounds("group-3-close-group")
				if !ok {
					fail(errors.New("captured group close missing"))
					break
				}
				heldX, heldY = b.X+b.Width/2, b.Y+b.Height/2
				tabsNativePointer(cx, root, heldX, heldY, true, func(err error) {
					if err != nil {
						advance(err)
						return
					}
					tabsNativeKey(cx, root, 33, ui.ModifierControl, true, advance)
				})
			case 13:
				waiting = true
				tabsNativePointer(cx, root, heldX, heldY, false, advance)
			case 14:
				if m.closePrompt || len(m.allGroups()) != 3 || !strings.Contains(m.message, "changed during click") {
					fail(errors.New("old captured group action accepted changed review"))
					break
				}
				if err := captureGroupsPixels(cx, m, "captured"); err != nil {
					fail(err)
					break
				}
				key(34, ui.ModifierControl)
			case 15:
				waiting = true
				click("group-3-close-group", advance)
			case 16:
				if !m.closePrompt || m.closeGroupID != 3 || len(m.closePlans()) != 1 || m.closePlans()[0].document == d {
					fail(errors.New("native group close targets unrelated shared edit"))
					break
				}
				waiting = true
				click("close-cancel", advance)
			case 17:
				if m.closePrompt || len(m.allGroups()) != 3 {
					fail(errors.New("native Cancel lost group"))
					break
				}
				waiting = true
				click("group-3-close-group", advance)
			case 18:
				if m.closePrompt {
					waiting = true
					click("close-save", advance)
				}
			case 19:
				if m.closeBusy || m.closePrompt {
					cx.Invalidate()
					break
				}
				if len(m.allGroups()) != 2 || len(m.docs) != 1 || !d.dirty() {
					fail(errors.New("group save removed shared dirty resource"))
					break
				}
				if err := captureGroupsPixels(cx, m, "closed"); err != nil {
					fail(err)
					break
				}
				phase = 20
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
	if phase != 20 {
		return fmt.Errorf("native groups incomplete: %v", diagnostic.Load())
	}
	mainBytes, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(mainBytes) != source.String() {
		return errors.New("group save wrote unrelated shared dirty file")
	}
	otherBytes, err := os.ReadFile(filepath.Join(root, "other.go"))
	if err != nil || !strings.HasPrefix(string(otherBytes), "dirty ") {
		return errors.New("group Save did not persist actual unique source")
	}
	fmt.Println("Native editor groups passed: shared UTF-16/CRLF edits, independent caret/scroll, native focus/sash/nested splits, scoped Cancel/Save and untouched shared disk bytes")
	return nil
}

func captureGroupsPixels(cx *ui.Context, m *model, stage string) error {
	pixels, scale, err := captureWorkbenchGPU(cx, m.workspace)
	if err != nil {
		return err
	}
	inks := []int{}
	bounds := []ui.Bounds{}
	for _, g := range m.allGroups() {
		b, ok := cx.ElementBounds(groupKey(g, "editor-content"))
		if !ok {
			return fmt.Errorf("group %d content missing", g.id)
		}
		bounds = append(bounds, b)
		ink := 0
		large := g.current != nil && g.current.large != nil
		left, right, top := float32(68), float32(94), float32(0)
		if large {
			left, right, top = 10, 16, 48
		}
		for y := int(float64(b.Y+top) * scale); y < min(pixels.Bounds().Dy(), int(float64(b.Y+b.Height)*scale)); y++ {
			for x := int(float64(b.X+left) * scale); x < min(pixels.Bounds().Dx(), int(float64(b.X+b.Width-right)*scale)); x++ {
				p := pixels.RGBAAt(x, y)
				if (!large && int(p.B) > int(p.R)+15 && p.G > 90) || (large && p.R >= 100 && p.G >= 100 && p.B >= 100) {
					ink++
				}
			}
		}
		inks = append(inks, ink)
	}
	dir := os.Getenv("GOCODE_GROUPS_SCREENSHOTS")
	if dir == "" {
		dir = filepath.Join(".cache", "editor-groups")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, stage+".png"))
	if err != nil {
		return err
	}
	if err := errors.Join(png.Encode(f, pixels), f.Close()); err != nil {
		return err
	}
	data, err := json.MarshalIndent(struct {
		Stage         string
		Width, Height int
		Scale         float64
		Bounds        []ui.Bounds
		Ink           []int
	}{stage, pixels.Bounds().Dx(), pixels.Bounds().Dy(), scale, bounds, inks}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, stage+".json"), data, 0600); err != nil {
		return err
	}
	for i, ink := range inks {
		if ink < 16 {
			return fmt.Errorf("group %d lacks completed syntax GPU ink: %d", i, ink)
		}
	}
	return nil
}
