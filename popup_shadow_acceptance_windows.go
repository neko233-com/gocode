//go:build windows && cgo

package main

import (
	"context"
	"crypto/sha256"
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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/neko233-com/gocode/internal/uidispatch"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

type popupShadowStageReport struct {
	Run            int                  `json:"run"`
	Stage          string               `json:"stage"`
	Body           ui.Bounds            `json:"body"`
	Controls       map[string]ui.Bounds `json:"controls"`
	DPI            uint32               `json:"actual_window_dpi"`
	DrawableScale  float32              `json:"actual_drawable_scale"`
	Viewport       ui.Bounds            `json:"viewport_dip"`
	Width, Height  int
	CompletedFloor uint64               `json:"completed_floor"`
	Renderer       winprobe.RenderStats `json:"renderer"`
	Presentation   string               `json:"presentation"`
	Darkening      int                  `json:"halo_darkening"`
	Samples        []popupShadowPixel   `json:"pixels"`
}

type popupShadowRunReport struct {
	Run                int                  `json:"run"`
	Views              uint64               `json:"views"`
	HWND               uintptr              `json:"hwnd"`
	HWNDGone           bool                 `json:"hwnd_gone"`
	OldContextRejected bool                 `json:"old_context_rejected"`
	WorkerJoined       bool                 `json:"worker_joined"`
	Renderer           winprobe.RenderStats `json:"renderer"`
}

type popupShadowReport struct {
	Version        string                   `json:"version"`
	Commit         string                   `json:"commit"`
	PID            int                      `json:"pid"`
	ElapsedMS      int64                    `json:"elapsed_ms"`
	Stages         []popupShadowStageReport `json:"stages"`
	Runs           []popupShadowRunReport   `json:"runs"`
	SourceSHA256   string                   `json:"source_sha256"`
	ScratchRemoved bool                     `json:"scratch_removed"`
	Error          string                   `json:"error,omitempty"`
}

var popupShadowStages = []string{"base", "file", "submenu", "quick", "revert", "close", "history", "ancestor"}

// This CLI owns all native input and artifacts. It exercises the actual product
// builders/callbacks; it does not start Node, LSP, a terminal or update services.
func runPopupShadowAcceptance() (failure error) {
	started := time.Now()
	report := popupShadowReport{PID: os.Getpid(), Version: appVersion(), Commit: buildCommit()}
	output, scratch, err := preparePopupShadowOutput()
	if err != nil {
		return err
	}
	defer func() {
		report.ElapsedMS = time.Since(started).Milliseconds()
		if failure != nil {
			report.Error = failure.Error()
			_ = writePopupShadowJSON(filepath.Join(output, "failed.json"), report)
		}
	}()
	defer func() {
		if err := os.RemoveAll(scratch); err != nil {
			failure = errors.Join(failure, fmt.Errorf("cleanup owned popup scratch: %w", err))
		} else if _, err = os.Lstat(scratch); os.IsNotExist(err) {
			report.ScratchRemoved = true
		}
	}() // Normal success also requires explicit cleanup before current.json.
	previousScale, hadScale := os.LookupEnv("GODESKTOP_TEST_DRAWABLE_SCALE")
	if err = os.Unsetenv("GODESKTOP_TEST_DRAWABLE_SCALE"); err != nil {
		return err
	}
	defer func() {
		if hadScale {
			_ = os.Setenv("GODESKTOP_TEST_DRAWABLE_SCALE", previousScale)
		}
	}()
	for _, name := range []string{"GODESKTOP_READBACK", "GODESKTOP_TEST_INPUT_ISOLATION"} {
		previous, existed := os.LookupEnv(name)
		if err := os.Setenv(name, "1"); err != nil {
			return err
		}
		defer func() {
			if existed {
				_ = os.Setenv(name, previous)
			} else {
				_ = os.Unsetenv(name)
			}
		}()
	}
	workspace := filepath.Join(scratch, "workspace")
	if err = os.Mkdir(workspace, 0700); err != nil {
		return err
	}
	path := filepath.Join(workspace, "native 世界😀.go")
	const source = "package main\r\n// 原始 世界 😀\r\n"
	if err = os.WriteFile(path, []byte(source), 0600); err != nil {
		return err
	}
	expectedHash := sha256.Sum256([]byte(source))
	report.SourceSHA256 = fmt.Sprintf("%x", expectedHash)
	for run := 0; run < 3; run++ {
		if err = runPopupShadowWindow(workspace, path, output, run, &report); err != nil {
			return err
		}
	}
	body, err := os.ReadFile(path)
	if err != nil || sha256.Sum256(body) != expectedHash {
		return errors.Join(err, errors.New("popup callbacks changed the private source"))
	}
	if err = os.RemoveAll(scratch); err != nil {
		return fmt.Errorf("remove owned popup scratch: %w", err)
	}
	if _, err = os.Lstat(scratch); !os.IsNotExist(err) {
		return fmt.Errorf("popup scratch remains: %v", err)
	}
	report.ScratchRemoved = true
	report.ElapsedMS = time.Since(started).Milliseconds()
	if err = writePopupShadowJSON(filepath.Join(output, "current.json"), report); err != nil {
		return err
	}
	fmt.Printf("native popup shadows passed: %d captures, three closed Runs, actualDPI/path recorded, source unchanged\n", len(report.Stages))
	return nil
}

func runPopupShadowWindow(workspace, path, output string, run int, report *popupShadowReport) (failure error) {
	m, err := newWorkspaceModel(workspace)
	if err != nil {
		return err
	}
	d, err := loadDocument(context.Background(), path)
	if err != nil {
		return err
	}
	_, err = d.buffer.Apply([]textbuffer.Edit{{Range: textbuffer.Range{}, Text: "X"}}, nil)
	if err != nil {
		return err
	}
	m.docs, m.active, m.files, m.showPanel, m.editing = []*document{d}, 0, []string{filepath.Base(path)}, false, true
	m.rememberDocument(path, d)
	m.autoSave.config = defaultAutoSaveConfig() // No user configuration or auto writer.
	m.keyboard.profile = "vscode"
	originalText, originalVersion := d.buffer.Text(), d.buffer.Version()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var current atomic.Pointer[ui.Context]
	var expired atomic.Bool
	var views atomic.Uint64
	type job struct {
		work func() error
		done func(error)
	}
	jobs, workerDone := make(chan job, 1), make(chan struct{})
	title := fmt.Sprintf("gocode popup shadows — run%d", run)
	var window winprobe.Window
	var stopWatch func()
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		restore := winprobe.Awareness()
		defer restore()
		defer close(workerDone)
		for {
			select {
			case <-ctx.Done():
				return
			case request := <-jobs:
				if ctx.Err() != nil {
					return
				}
				_, _, result := waitPopupShadowQuiescence(ctx, &views)
				if result == nil {
					result = request.work()
				}
				cx := current.Load()
				if cx == nil || !uidispatch.Retry(ctx, cx.Dispatch, func() { request.done(result) }) {
					return
				}
			}
		}
	}()
	defer func() {
		if stopWatch != nil {
			stopWatch()
		}
		cancel()
		select {
		case <-workerDone:
		case <-time.After(3 * time.Second):
			failure = errors.Join(failure, errors.New("popup native worker did not join within three seconds"))
		}
		m.closeDocuments()
	}()
	watchdog := time.AfterFunc(35*time.Second, func() {
		expired.Store(true)
		if cx := current.Load(); cx != nil {
			cx.Quit()
		}
	})
	defer watchdog.Stop()
	phase, cancelStep := 0, false
	busy := false
	floor := uint64(3)
	var baseline *image.RGBA
	var fileBaseline *image.RGBA
	rootClicks := 0
	width, height := float32(900), float32(680)
	if run == 1 {
		width, height = 640, 480
	}
	err = ui.Run(ui.WindowOptions{Title: title, Width: width, Height: height, CustomTitlebar: true, Background: ui.RGB(outer), Input: m.input}, func(cx *ui.Context) *ui.Element {
		views.Add(1)
		if current.Load() == nil {
			current.Store(cx)
			m.native = cx
			stopWatch = m.startDocumentWatch(ctx, cx.Dispatch)
		}
		if expired.Load() {
			cx.Quit()
			return ui.Column()
		}
		fail := func(err error) {
			failure = fmt.Errorf("popup Run%d phase%d cancel=%t: %w", run, phase, cancelStep, err)
			cx.Quit()
		}
		next := func() { floor = winprobe.RendererStats().Submitted + 3; cx.Invalidate() }
		var tree *ui.Element
		if phase == 7 || phase == 8 {
			// A real ancestor must still clip its positioned child's halo and hits.
			parent := ui.Stack(ui.Column().Background(ui.RGB(0xeeeeee)), m.menuPanel("Preferences", 0).Position(0, 0)).Width(326).Height(132).ClipRounded(24).Position(30, 40).Key("popup-ancestor")
			tree = ui.Stack(ui.Column().Background(ui.RGB(0x808080)).OnClick(func(*ui.Context) { rootClicks++ }), parent)
		} else {
			tree = m.view(cx)
		}
		if busy {
			return tree
		}
		stats := winprobe.RendererStats()
		if stats.Completed < floor {
			cx.Invalidate()
			return tree
		}
		queue := func(work func() error, done func(error)) {
			busy = true
			jobs <- job{work, func(err error) {
				busy = false
				if err != nil {
					fail(err)
					return
				}
				done(nil)
			}}
		}
		find := func() error { var err error; window, err = winprobe.Find(title, uint32(os.Getpid())); return err }
		click := func(b ui.Bounds) error {
			x, y := int(b.X+b.Width/2), int(b.Y+b.Height/2)
			if err := window.Pointer(0x201, x, y); err != nil {
				return err
			}
			return window.Pointer(0x202, x, y)
		}
		bounds := func(key string) (ui.Bounds, bool) {
			b, ok := cx.ElementBounds(key)
			if !ok {
				fail(fmt.Errorf("actual key %s unavailable", key))
			}
			return b, ok
		}
		w, h := cx.WindowSize()
		if phase == 0 {
			file, ok := bounds("menu-File")
			if !ok {
				return tree
			}
			floorCopy := floor
			queue(func() error {
				if err := find(); err != nil {
					return err
				}
				var err error
				baseline, err = capturePopupShadowStage(output, run, "base", window, floorCopy, ui.Bounds{Width: w, Height: h}, ui.Bounds{}, nil, nil, nil, nil, report)
				if err != nil {
					return err
				}
				return click(file)
			}, func(error) {
				if m.menu.name != "File" {
					fail(errors.New("actual File header pointer did not open File"))
					return
				}
				phase = 1
				next()
			})
			return tree
		}
		if phase == 8 {
			queue(func() error {
				// Every job has already settled the current View off UI.
				// Start the unchanged zero-increment interval from that real
				// completed snapshot, admitting no extra frame during the gate.
				before, beforeViews := winprobe.RendererStats(), views.Load()
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(250 * time.Millisecond):
				}
				after := winprobe.RendererStats()
				if before.Submitted != after.Submitted || before.Completed != after.Completed || before.FrameTicks != after.FrameTicks || after.InFlight != 0 || views.Load() != beforeViews {
					return fmt.Errorf("popup idle rendered: before%+v after%+v views%d→%d", before, after, beforeViews, views.Load())
				}
				return nil
			}, func(error) { phase = 9; cx.Quit() })
			return tree
		}
		keys := []string{"", "menu-panel-0", "menu-panel-1", "quick-popup", "reload-dialog", "close-dialog", "history-dialog", "menu-panel-0"}
		body, ok := bounds(keys[phase])
		if !ok {
			return tree
		}
		style, shade := menuPopupShadow(), float32(0)
		controls := map[string]ui.Bounds{}
		otherBodies := []ui.Bounds{}
		var cancelKey string
		switch phase {
		case 1:
			if body != (ui.Bounds{X: m.menu.x, Y: m.menu.y, Width: 324, Height: menuHeight(m.menuEntries("File"))}) {
				fail(fmt.Errorf("File body moved: %+v", body))
				return tree
			}
			for i, entry := range m.menuEntries("File") {
				if entry.Submenu == "Preferences" {
					b, ok := bounds(menuRowKey(0, i))
					if !ok {
						return tree
					}
					controls["preferences"] = b
				}
			}
		case 2:
			mainBody, ok := bounds("menu-panel-0")
			if !ok {
				return tree
			}
			otherBodies = append(otherBodies, mainBody)
			x := m.menu.subx
			if x+324 > w {
				x = max(0, m.menu.x-324)
			}
			y := min(m.menu.suby, max(36, h-menuHeight(m.menuEntries("Preferences"))))
			if body != (ui.Bounds{X: x, Y: max(0, y), Width: 324, Height: menuHeight(m.menuEntries("Preferences"))}) {
				fail(fmt.Errorf("submenu body moved: %+v", body))
				return tree
			}
		case 3:
			style = modalPopupShadow()
			wantedW := min(float32(600), max(200, w-32))
			if body.X != max(0, (w-wantedW)/2) || body.Y != 6 || body.Width != wantedW || body.Height != 70 {
				fail(fmt.Errorf("Quick Input body moved: %+v", body))
				return tree
			}
		case 4, 5, 6:
			style, shade = modalPopupShadow(), .65
			wantedW, wantedH := float32(460), float32(140)
			statusLines := 0
			cancelKey = "reload-cancel"
			if phase == 4 {
				if m.reloadError != "" || m.reloadBusy {
					fail(fmt.Errorf("dirty Revert confirmation unexpectedly has status %q busy=%t", m.reloadError, m.reloadBusy))
					return tree
				}
				// The original dialog keeps one 20-DIP blank status row:
				// wrapChatText("") returns []string{""}, rather than nil.
				statusLines = len(wrapChatText(m.reloadError, 390))
				wantedH += float32(20 * statusLines)
			}
			if phase == 5 {
				plans := m.closePlans()
				if m.closeError != "" || m.closeBusy || len(plans) != 1 || plans[0].document != d {
					fail(fmt.Errorf("dirty close confirmation changed: status %q busy=%t plans=%d", m.closeError, m.closeBusy, len(plans)))
					return tree
				}
				statusLines = len(wrapChatText(m.closeError, 390))
				wantedH, cancelKey = float32(114+22+20*statusLines), "close-cancel"
			}
			if phase == 6 {
				wantedW, wantedH, cancelKey = 440, 138, "history-cancel"
			}
			if body != (ui.Bounds{X: max(0, (w-wantedW)/2), Y: max(0, (h-wantedH)/2), Width: wantedW, Height: wantedH}) {
				fail(fmt.Errorf("modal body moved: %+v", body))
				return tree
			}
			b, ok := bounds(cancelKey)
			if !ok {
				return tree
			}
			controls[cancelKey] = b
			wantedCancel := ui.Bounds{X: body.X + body.Width - 112, Y: body.Y + 76 + float32(20*statusLines), Width: 90, Height: 30}
			if phase == 5 {
				wantedCancel.Y = body.Y + 74 + float32(20*statusLines)
			}
			if phase == 6 {
				wantedCancel.X, wantedCancel.Width = body.X+313, 80
			}
			if b != wantedCancel {
				fail(fmt.Errorf("actual %s bounds moved: got%+v want%+v", cancelKey, b, wantedCancel))
				return tree
			}
		case 7:
			if body != (ui.Bounds{X: 30, Y: 40, Width: 324, Height: 130}) {
				fail(fmt.Errorf("clipped positioned body moved: %+v", body))
				return tree
			}
			b, ok := bounds(menuRowKey(0, 0))
			if !ok {
				return tree
			}
			controls["settings"] = b
			ancestor, ok := bounds("popup-ancestor")
			if !ok {
				return tree
			}
			controls["ancestor"] = ancestor
			if ancestor != (ui.Bounds{X: 30, Y: 40, Width: 326, Height: 132}) {
				fail(fmt.Errorf("real ancestor bounds moved: %+v", ancestor))
				return tree
			}
		}
		if cancelStep {
			queue(func() error { return click(controls[cancelKey]) }, func(error) {
				if d.buffer.Text() != originalText || d.buffer.Version() != originalVersion || !d.dirty() {
					fail(errors.New("popup Cancel changed the dirty buffer"))
					return
				}
				if phase == 4 {
					if m.reloadPrompt != nil {
						fail(errors.New("actual Revert Cancel did not dismiss"))
						return
					}
					m.beginClose(d)
				}
				if phase == 5 {
					if m.closePrompt {
						fail(errors.New("actual close Cancel did not dismiss"))
						return
					}
					m.history.prompt = &historyPrompt{group: &historyGroup{label: "Native popup confirmation", members: []historyMember{{path: d.path}, {path: d.path}}}}
				}
				if phase == 6 {
					if m.history.prompt != nil {
						fail(errors.New("actual history Cancel did not dismiss"))
						return
					}
					m.menu.name = "Preferences"
				}
				phase++
				cancelStep = false
				next()
			})
			return tree
		}
		stage := popupShadowStages[phase]
		phaseCopy := phase
		floorCopy := floor
		background := baseline
		if phase == 2 {
			background = fileBaseline
		}
		ancestor := phase == 7
		dismissAt, canDismiss := popupHaloClickPoint(body, otherBodies, w, h)
		if !ancestor && !canDismiss {
			fail(errors.New("no real outside-all-bodies halo point"))
			return tree
		}
		queue(func() error {
			image, err := capturePopupShadowStage(output, run, stage, window, floorCopy, ui.Bounds{Width: w, Height: h}, body, controls, &style, background, &shade, report)
			if err != nil {
				return err
			}
			if phaseCopy == 1 {
				fileBaseline = image
				b := controls["preferences"]
				return window.Pointer(0x200, int(b.X+b.Width/2), int(b.Y+b.Height/2))
			}
			if ancestor {
				if err := checkPopupAncestorPixels(image, window, body); err != nil {
					return err
				}
				// Inside the row's own 4-DIP corners and menu's 8-DIP body,
				// but outside the real ancestor's 24-DIP corner.
				return click(ui.Bounds{X: 36, Y: 47, Width: 1, Height: 1})
			}
			// This is outside the body and inside the halo, never a global click.
			return click(dismissAt)
		}, func(error) {
			switch phase {
			case 1:
				if m.menu.submenu != "Preferences" {
					fail(errors.New("actual menu hover did not open submenu"))
					return
				}
				phase = 2
			case 2:
				if m.menu.name != "" || m.closePrompt || m.reloadPrompt != nil || m.activity != "files" {
					fail(errors.New("submenu halo stole the dismiss hit"))
					return
				}
				m.openQuickInput(false)
				phase = 3
			case 3:
				if m.palette {
					fail(errors.New("Quick Input halo stole the dismiss hit"))
					return
				}
				m.beginReload(d)
				if m.reloadPrompt != d {
					fail(errors.New("real dirty Revert prompt unavailable"))
					return
				}
				phase = 4
			case 4:
				if m.reloadPrompt != d {
					fail(errors.New("Revert halo accepted/dismissed the modal"))
					return
				}
				cancelStep = true
			case 5:
				if !m.closePrompt {
					fail(errors.New("close halo accepted/dismissed the modal"))
					return
				}
				cancelStep = true
			case 6:
				if m.history.prompt == nil {
					fail(errors.New("history halo accepted/dismissed the modal"))
					return
				}
				cancelStep = true
			case 7:
				if rootClicks != 1 || m.menu.name != "Preferences" {
					fail(errors.New("rounded ancestor corner expanded the menu hit"))
					return
				}
				queue(func() error { return click(controls["settings"]) }, func(error) {
					if m.activity != "settings" || m.menu.name != "" {
						fail(errors.New("actual clipped menu row did not activate"))
						return
					}
					phase = 8
					next()
				})
				return
			}
			next()
		})
		return tree
	})
	old := current.Swap(nil)
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	if expired.Load() || phase != 9 {
		return fmt.Errorf("popup acceptance expired/incomplete at phase%d", phase)
	}
	if stopWatch != nil {
		stopWatch()
		stopWatch = nil
	}
	cancel()
	select {
	case <-workerDone:
	case <-time.After(3 * time.Second):
		return errors.New("popup worker failed to join")
	}
	stats := winprobe.RendererStats()
	rejected := old != nil && !old.Dispatch(func() {})
	isWindow, _, _ := syscall.NewLazyDLL("user32.dll").NewProc("IsWindow").Call(uintptr(window))
	if !rejected || isWindow != 0 || stats.InFlight != 0 || stats.DroppedFrames != 0 || stats.Submitted != stats.Completed {
		return fmt.Errorf("popup shutdown contextRejected=%t HWNDlive=%d stats%+v", rejected, isWindow, stats)
	}
	report.Runs = append(report.Runs, popupShadowRunReport{Run: run, Views: views.Load(), HWND: uintptr(window), HWNDGone: true, OldContextRejected: rejected, WorkerJoined: true, Renderer: stats})
	return nil
}

// The only native worker observes completion without requesting more frames.
// It starts during View, so it must also wait for that View's final submission.
func waitPopupShadowQuiescence(ctx context.Context, views *atomic.Uint64) (winprobe.RenderStats, uint64, error) {
	before, beforeViews := winprobe.RendererStats(), views.Load()
	settle := time.NewTimer(2 * time.Second)
	defer settle.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	stableAt := time.Now()
	for before.InFlight != 0 || time.Since(stableAt) < 50*time.Millisecond {
		select {
		case <-ctx.Done():
			return before, beforeViews, ctx.Err()
		case <-settle.C:
			return before, beforeViews, errors.New("popup current View did not become quiescent within two seconds")
		case <-tick.C:
		}
		sample, sampleViews := winprobe.RendererStats(), views.Load()
		if sample.InFlight != 0 || before.Submitted != sample.Submitted || before.Completed != sample.Completed || before.FrameTicks != sample.FrameTicks || beforeViews != sampleViews {
			stableAt = time.Now()
		}
		before, beforeViews = sample, sampleViews
	}
	return before, beforeViews, nil
}

func capturePopupShadowStage(output string, run int, stage string, window winprobe.Window, floor uint64, viewport, body ui.Bounds, controls map[string]ui.Bounds, style *ui.ShadowStyle, baseline *image.RGBA, shade *float32, report *popupShadowReport) (*image.RGBA, error) {
	stats := winprobe.RendererStats()
	presentation, err := winprobe.NativePresentation(window, uint32(os.Getpid()))
	if err != nil {
		return nil, err
	}
	if err = winprobe.ValidateWindowsPresentation(stats, presentation); err != nil {
		return nil, err
	}
	maxInFlight := uint32(3)
	if presentation == "committed-dib" {
		maxInFlight = 2
	}
	if stats.Completed < floor || stats.InFlight != 0 || stats.FrameSlots != 3 || stats.UsedSlotsMask & ^uint32(7) != 0 || stats.MaxInFlight > maxInFlight || stats.DroppedFrames != 0 {
		return nil, fmt.Errorf("popup real frame/slots floor%d: %+v", floor, stats)
	}
	pixels, err := window.Capture()
	if err != nil {
		return nil, err
	}
	entry := popupShadowStageReport{Run: run, Stage: stage, Body: body, Controls: controls, DPI: window.DPI(), Viewport: viewport, Width: pixels.Bounds().Dx(), Height: pixels.Bounds().Dy(), CompletedFloor: floor, Renderer: stats, Presentation: presentation}
	if viewport.Width <= 0 || viewport.Height <= 0 {
		return nil, errors.New("popup real viewport unavailable")
	}
	entry.DrawableScale = float32(entry.Width) / viewport.Width
	dpiScale := float32(entry.DPI) / 96
	if math.Abs(float64(entry.DrawableScale-dpiScale)) > 1e-6 || entry.Height != int(math.Round(float64(viewport.Height*dpiScale))) {
		return nil, fmt.Errorf("popup real drawable/viewport/DPI mismatch: %+v", entry)
	}
	if style != nil {
		scale := entry.DrawableScale
		if stage == "ancestor" {
			entry.Samples = popupHaloSamples(pixels, body, *style, scale, 0, nil, 0xeeeeee)
		} else {
			entry.Samples = popupHaloSamples(pixels, body, *style, scale, *shade, baseline, 0)
		}
		entry.Darkening, err = checkPopupShadowPixels(pixels, entry.Samples)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", stage, err)
		}
		if stage != "ancestor" {
			if err = checkPopupCornerPixel(pixels, body, *style, scale, *shade, baseline); err != nil {
				return nil, fmt.Errorf("%s: %w", stage, err)
			}
		}
	}
	f, err := os.Create(filepath.Join(output, fmt.Sprintf("run-%d-%s.png", run, stage)))
	if err != nil {
		return nil, err
	}
	err = errors.Join(png.Encode(f, pixels), f.Close())
	if err != nil {
		return nil, err
	}
	report.Stages = append(report.Stages, entry)
	return pixels, nil
}

func popupHaloSamples(pixels *image.RGBA, body ui.Bounds, style ui.ShadowStyle, scale, shade float32, baseline *image.RGBA, fixedBackground uint32) []popupShadowPixel {
	samples := []popupShadowPixel{}
	for _, dy := range []int{-8, 0, 8} {
		for _, dx := range []int{0, 1, 2} {
			point := image.Pt(int(math.Ceil(float64(body.X+body.Width)*float64(scale)))+dx, int(float64(body.Y+body.Height/2)*float64(scale))+dy)
			if !point.In(pixels.Bounds()) {
				continue
			}
			if baseline == nil && float32(point.X)+.5 >= (body.X+326)*scale {
				continue
			}
			background := [3]uint8{uint8(fixedBackground >> 16), uint8(fixedBackground >> 8), uint8(fixedBackground)}
			if baseline != nil {
				r, g, b, _ := baseline.At(point.X, point.Y).RGBA()
				background = [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
			}
			for i, value := range background {
				background[i] = uint8(math.Round(float64(value) * float64(1-shade)))
			}
			samples = append(samples, popupShadowPixelAt(body, style, scale, point, background))
		}
	}
	return samples
}

func checkPopupCornerPixel(pixels *image.RGBA, body ui.Bounds, style ui.ShadowStyle, scale, shade float32, baseline *image.RGBA) error {
	point := image.Pt(int(math.Floor(float64(body.X)*float64(scale))), int(math.Floor(float64(body.Y)*float64(scale))))
	if !point.In(pixels.Bounds()) || baseline == nil {
		return errors.New("rounded popup corner unavailable")
	}
	r, g, b, _ := baseline.At(point.X, point.Y).RGBA()
	background := [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
	for i, value := range background {
		background[i] = uint8(math.Round(float64(value) * float64(1-shade)))
	}
	sample := popupShadowPixelAt(body, style, scale, point, background)
	r, g, b, _ = pixels.At(point.X, point.Y).RGBA()
	actual := [3]int{int(r >> 8), int(g >> 8), int(b >> 8)}
	for i, wanted := range sample.Expected {
		difference := actual[i] - int(wanted)
		if difference < -1 || difference > 1 {
			return fmt.Errorf("rounded corner %v RGB%v want%v", point, actual, sample.Expected)
		}
	}
	return nil
}

func checkPopupAncestorPixels(pixels *image.RGBA, window winprobe.Window, body ui.Bounds) error {
	scale := float64(window.DPI()) / 96
	for _, point := range []image.Point{image.Pt(int(31*scale), int(41*scale)), image.Pt(int((body.X+body.Width+4)*float32(scale)), int((body.Y+body.Height/2)*float32(scale)))} {
		r, g, b, _ := pixels.At(point.X, point.Y).RGBA()
		if r>>8 != 128 || g>>8 != 128 || b>>8 != 128 {
			return fmt.Errorf("real rounded/rect ancestor leaked at%v RGB%d/%d/%d", point, r>>8, g>>8, b>>8)
		}
	}
	return nil
}

func preparePopupShadowOutput() (string, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	output := os.Getenv("GOCODE_POPUP_SHADOW_OUTPUT")
	if output == "" {
		output = filepath.Join(cwd, ".cache", "popup-shadow-native")
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return "", "", err
	}
	if err = validatePopupShadowDirectory(output); err != nil {
		return "", "", err
	}
	if err = os.MkdirAll(output, 0700); err != nil {
		return "", "", err
	}
	if _, err = os.Lstat(filepath.Join(output, "failed.json")); err == nil {
		return "", "", errors.New("preserved popup failed.json must be archived before reusing this owned output")
	} else if !os.IsNotExist(err) {
		return "", "", err
	}
	for run := 0; run < 3; run++ {
		for _, stage := range popupShadowStages {
			path := filepath.Join(output, fmt.Sprintf("run-%d-%s.png", run, stage))
			if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
				return "", "", err
			}
		}
	}
	for _, name := range []string{"current.json", "failed.json"} {
		if err = os.Remove(filepath.Join(output, name)); err != nil && !os.IsNotExist(err) {
			return "", "", err
		}
	}
	scratch := filepath.Join(output, ".scratch")
	if err = validatePopupShadowDirectory(scratch); err != nil {
		return "", "", err
	}
	if err = os.RemoveAll(scratch); err != nil {
		return "", "", err
	}
	if err = os.Mkdir(scratch, 0700); err != nil {
		return "", "", err
	}
	return output, scratch, nil
}

func validatePopupShadowDirectory(output string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	cache := filepath.Join(cwd, ".cache")
	rel, err := filepath.Rel(cache, output)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return errors.New("popup output must be an owned subdirectory of workspace .cache")
	}
	for path := output; ; path = filepath.Dir(path) {
		if info, err := os.Lstat(path); err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return fmt.Errorf("unsafe popup output ancestor%s", path)
			}
			if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok && data.FileAttributes&0x400 != 0 {
				return fmt.Errorf("reparsed popup output ancestor%s", path)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if strings.EqualFold(path, cwd) {
			break
		}
		if filepath.Dir(path) == path {
			return errors.New("popup output escaped workspace")
		}
	}
	return nil
}

func writePopupShadowJSON(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0600)
}
