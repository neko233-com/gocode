//go:build windows && cgo

package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf16"

	"github.com/neko233-com/gocode/internal/filewatch"
	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

// This acceptance owns its files/configuration and sends only PID-verified
// HWND messages. One worker serializes native probes, disk assertions, external
// replacements and PNG encoding; the UI owns all live buffers and phase state.
func runAutoSaveAcceptance() error {
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
	root, err := os.MkdirTemp("", "gocode-auto-save-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	path := filepath.Join(root, "main 世界😀.go")
	secondPath := filepath.Join(root, "second.go")
	initial := "// 原始 世界 😀\r\npackage main\r\n"
	if err = os.WriteFile(path, []byte(initial), 0600); err != nil {
		return err
	}
	if err = os.WriteFile(secondPath, []byte("package main\r\n// second tab\r\n"), 0600); err != nil {
		return err
	}
	configPath := filepath.Join(root, "autosave.json")
	if err = writeAutoSaveConfig(configPath, defaultAutoSaveConfig()); err != nil {
		return err
	}
	m, err := newWorkspaceModel(root)
	if err != nil {
		return err
	}
	m.autoSave.config = defaultAutoSaveConfig() // Never consult user settings.
	m.keyboard.profile = "vscode"
	m.terminalAcceptance = true
	m.files = []string{filepath.Base(path), filepath.Base(secondPath)}
	m.logo, err = loadWorkbenchLogo()
	if err != nil {
		return err
	}
	d, err := loadDocument(context.Background(), path)
	if err != nil {
		return err
	}
	second, err := loadDocument(context.Background(), secondPath)
	if err != nil {
		return err
	}
	m.docs = []*document{d, second}
	m.rememberDocument(path, d)
	m.rememberDocument(secondPath, second)
	m.focusTab(d)
	ctx, cancel := context.WithCancel(context.Background())
	var stopAuto, stopWatch, stopSettings, stopSaves, stopTerminals func()
	jobs, workerDone := make(chan func() error, 1), make(chan struct{})
	var current atomic.Pointer[ui.Context]
	phase, frame, capturePhase := 0, uint64(4), -1
	busy := false
	var failure error
	var quietUntil time.Time
	var expectedDisk, preservedLocal string
	var preservedVersion int
	var untitled *document
	saveRequests := 0
	var advance func(error)
	go func() {
		defer close(workerDone)
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-jobs:
				result := job()
				if cx := current.Load(); cx != nil {
					if !cx.Dispatch(func() { advance(result) }) {
						return
					}
				}
			}
		}
	}()
	defer func() {
		// Policy timers stop before watchers/configuration/writes, so shutdown
		// cannot schedule a new automatic write or produce a Save As dialog.
		for _, stop := range []func(){stopAuto, stopWatch, stopSettings, stopSaves, stopTerminals} {
			if stop != nil {
				stop()
			}
		}
		cancel()
		<-workerDone
		m.closeDocuments()
	}()
	watchdog := time.AfterFunc(60*time.Second, func() {
		fmt.Fprintln(os.Stderr, "native Auto Save acceptance timed out")
		os.Exit(2) // The process-owning test also owns/cleans the private temp root.
	})
	defer watchdog.Stop()
	dir := os.Getenv("GOCODE_AUTOSAVE_SCREENSHOTS")
	if dir == "" {
		dir = filepath.Join(".cache", "auto-save-native")
	}
	nativeWindow := func() (winprobe.Window, error) {
		return winprobe.Find("gocode — "+filepath.Base(root), uint32(os.Getpid()))
	}
	readExact := func(wanted string) error {
		body, err := os.ReadFile(path)
		if err == nil && string(body) != wanted {
			err = fmt.Errorf("actual disk bytes differ: got %q, want %q", body, wanted)
		}
		return err
	}
	readConfig := func(wanted autoSaveConfig) error {
		// The separate settings writer is asynchronous. Waiting here never
		// stalls the UI, and the three-second bound reports a real failure.
		until := time.Now().Add(3 * time.Second)
		var err error
		for time.Now().Before(until) {
			var actual autoSaveConfig
			actual, err = readAutoSaveConfig(configPath)
			if err == nil && actual == wanted {
				return nil
			}
			if err == nil {
				err = fmt.Errorf("persisted policy %+v, want %+v", actual, wanted)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(15 * time.Millisecond):
			}
		}
		return err
	}
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(root), Width: 1280, Height: 820, CustomTitlebar: true, Input: m.input, CloseRequested: m.requestWindowClose, Background: ui.RGB(editor)}, func(cx *ui.Context) *ui.Element {
		if current.Load() == nil {
			current.Store(cx)
			m.native = cx
			stopSaves = m.startDocumentSaves(ctx, cx)
			queuedSave := m.requestSave
			m.requestSave = func(request context.Context, docs []*document, done func(error)) {
				if len(docs) != 0 {
					saveRequests++ // Receipt from the real writer, never a mock save.
				}
				queuedSave(request, docs, done)
			}
			stopWatch = m.startDocumentWatch(ctx, cx.Dispatch)
			stopSettings = m.startAutoSaveSettings(ctx, cx.Dispatch, configPath)
			stopAuto = m.startAutoSaveActor(ctx, cx.Dispatch)
			stopTerminals = m.startTerminals(ctx, cx)
		}
		fail := func(err error) {
			failure = fmt.Errorf("Auto Save phase %d mode=%s delay=%d saveID=%d dirty=%t message=%q: %w", phase, m.autoSaveConfig().Mode, m.autoSaveConfig().DelayMS, d.saveID, d.dirty(), m.message, err)
			cx.Quit()
		}
		advance = func(err error) {
			busy = false
			if err != nil {
				fail(err)
				return
			}
			fmt.Fprintf(os.Stderr, "native Auto Save phase=%d passed mode=%s delay=%d saveID=%d dirty=%t\n", phase, m.autoSaveConfig().Mode, m.autoSaveConfig().DelayMS, d.saveID, d.dirty())
			phase++
			frame = cx.RenderedFrames() + 3
			quietUntil = time.Time{}
		}
		submit := func(job func() error) {
			busy = true
			select {
			case jobs <- job:
			default:
				fail(errors.New("acceptance worker queue unexpectedly full"))
			}
		}
		send := func(messages ...[3]uintptr) {
			submit(func() error {
				w, err := nativeWindow()
				for _, msg := range messages {
					if err == nil {
						err = w.Send(uint32(msg[0]), msg[1], msg[2])
					}
				}
				return err
			})
		}
		clickBounds := func(w winprobe.Window, b ui.Bounds) error {
			x, y := int(b.X+b.Width/2), int(b.Y+b.Height/2)
			if err := w.Pointer(0x201, x, y); err != nil {
				return err
			}
			return w.Pointer(0x202, x, y)
		}
		click := func(key string) {
			b, ok := cx.ElementBounds(key)
			if !ok {
				fail(fmt.Errorf("missing native control %s", key))
				return
			}
			submit(func() error {
				w, err := nativeWindow()
				if err == nil {
					err = clickBounds(w, b)
				}
				return err
			})
		}
		policyClick := func(mode, key string) {
			wanted := m.autoSaveConfig()
			if wanted.Mode != mode {
				fail(fmt.Errorf("native policy %s, want %s", wanted.Mode, mode))
				return
			}
			b, ok := cx.ElementBounds(key)
			if !ok {
				fail(fmt.Errorf("missing native control %s", key))
				return
			}
			submit(func() error {
				if err := readConfig(wanted); err != nil {
					return err
				}
				w, err := nativeWindow()
				if err == nil {
					err = clickBounds(w, b)
				}
				return err
			})
		}
		menuKey := func(id string) string {
			for i, entry := range m.menuEntries("File") {
				if entry.ID == id {
					return menuRowKey(0, i)
				}
			}
			return "missing-menu-" + id
		}
		text := func(value string) {
			submit(func() error {
				w, err := nativeWindow()
				for _, r := range utf16.Encode([]rune(value)) {
					if err == nil {
						err = w.Send(0x102, uintptr(r), 0)
					}
				}
				return err
			})
		}
		setDelay := func(backspaces int, value string) {
			submit(func() error {
				w, err := nativeWindow()
				for range backspaces {
					if err == nil {
						err = w.Send(0x100, 8, 0)
					}
				}
				for _, r := range value {
					if err == nil {
						err = w.Send(0x102, uintptr(r), 0)
					}
				}
				if err == nil {
					err = w.Send(0x100, 13, 0)
				}
				return err
			})
		}
		capture := func(stage, key string, after func(winprobe.Window) error) {
			// Model receipts can arrive after the phase's original threshold.
			// Captures therefore wait three new completed native submissions.
			if capturePhase != phase {
				capturePhase, frame = phase, cx.RenderedFrames()+3
				return
			}
			b, ok := cx.ElementBounds(key)
			if !ok {
				fail(fmt.Errorf("capture %s missing control %s", stage, key))
				return
			}
			submit(func() error {
				w, err := nativeWindow()
				if err != nil {
					return err
				}
				pixels, err := w.Capture()
				if err != nil {
					return err
				}
				scale := float64(w.DPI()) / 96
				if stage == "file-menu-checked" {
					// Sample only the native checkmark cell, excluding the title,
					// so a rendered but unchecked row cannot satisfy this gate.
					b.X += 8
					b.Width = 18
				}
				region := image.Rect(int(math.Floor(float64(b.X)*scale)), int(math.Floor(float64(b.Y)*scale)), int(math.Ceil(float64(b.X+b.Width)*scale)), int(math.Ceil(float64(b.Y+b.Height)*scale))).Intersect(pixels.Bounds())
				ink := 0
				for y := region.Min.Y; y < region.Max.Y; y++ {
					for x := region.Min.X; x < region.Max.X; x++ {
						r, g, blue, _ := pixels.At(x, y).RGBA()
						if r>>8 > 100 && g>>8 > 100 && blue>>8 > 100 {
							ink++
						}
					}
				}
				if ink < 5 {
					return fmt.Errorf("completed native %s control lacks glyph ink: %d", stage, ink)
				}
				if err = os.MkdirAll(dir, 0755); err != nil {
					return err
				}
				f, err := os.Create(filepath.Join(dir, stage+".png"))
				if err != nil {
					return err
				}
				if err = errors.Join(png.Encode(f, pixels), f.Close()); err != nil {
					return err
				}
				if after != nil {
					return after(w)
				}
				return nil
			})
		}
		quiet := func(duration time.Duration) bool {
			if quietUntil.IsZero() {
				quietUntil = time.Now().Add(duration)
			}
			return !time.Now().Before(quietUntil)
		}
		saved := func(id uint64) bool {
			if m.saveBusy || len(m.saveJobs) != 0 || d.dirty() {
				return false
			}
			if d.saveID != id {
				fail(fmt.Errorf("save receipt count %d, want %d", d.saveID, id))
				return false
			}
			return true
		}
		if !busy && cx.RenderedFrames() >= frame && failure == nil {
			switch phase {
			case 0:
				if m.autoSaveConfig() != defaultAutoSaveConfig() || d.dirty() {
					fail(errors.New("private default off/1000 configuration was not respected"))
					break
				}
				send([3]uintptr{0x6, 1, 0})
			case 1:
				click("menu-File")
			case 2:
				checked := false
				for _, e := range m.menuEntries("File") {
					if e.ID == "autoSave" {
						checked = e.Checked
					}
				}
				if m.menu.name != "File" || checked {
					fail(errors.New("File Auto Save initial checkmark is incorrect"))
					break
				}
				click(menuKey("autoSave"))
			case 3:
				if m.autoSaveConfig().Mode != "afterDelay" {
					fail(errors.New("native File menu failed to enable afterDelay"))
					break
				}
				policyClick("afterDelay", "menu-File")
			case 4:
				checked := false
				for _, e := range m.menuEntries("File") {
					if e.ID == "autoSave" {
						checked = e.Checked
					}
				}
				if m.menu.name != "File" || !checked {
					fail(errors.New("File Auto Save checkmark did not track enabled policy"))
					break
				}
				capture("file-menu-checked", menuKey("autoSave"), func(w winprobe.Window) error { return w.Send(0x100, 27, 0) })
			case 5:
				click("code-line-0")
			case 6:
				text("保存 世界😀 ")
			case 7:
				if !saved(1) {
					break
				}
				expectedDisk = d.buffer.Text()
				if !strings.Contains(expectedDisk, "保存 世界😀 ") || d.buffer.EOL() != "\r\n" {
					fail(errors.New("native Unicode input or CRLF preservation failed"))
					break
				}
				wanted := expectedDisk
				submit(func() error { return readExact(wanted) })
			case 8:
				if !quiet(1100 * time.Millisecond) {
					break
				}
				if d.saveID != 1 || d.dirty() || m.saveBusy {
					fail(errors.New("unchanged clean document was automatically saved more than once"))
					break
				}
				click("settings")
			case 9:
				if m.activity != "settings" {
					fail(errors.New("native Settings pointer did not open Settings"))
					break
				}
				for _, key := range []string{"autosave-off", "autosave-afterDelay", "autosave-onFocusChange", "autosave-onWindowChange", "autosave-delay"} {
					if _, ok := cx.ElementBounds(key); !ok {
						fail(fmt.Errorf("missing native Settings control %s", key))
					}
				}
				if failure == nil {
					capture("settings-after-delay", "autosave-afterDelay", nil)
				}
			case 10:
				click("autosave-afterDelay")
			case 11:
				click("autosave-delay")
			case 12:
				setDelay(4, "125")
			case 13:
				if m.autoSaveConfig().DelayMS != 125 || m.autoSave.delayFocused {
					fail(errors.New("native delay input did not commit 125ms"))
					break
				}
				wanted := m.autoSaveConfig()
				submit(func() error { return readConfig(wanted) })
			case 14:
				click("autosave-onFocusChange")
			case 15:
				if m.autoSaveConfig().Mode != "onFocusChange" {
					fail(errors.New("native focus-change mode selection failed"))
					break
				}
				policyClick("onFocusChange", "code-line-0")
			case 16:
				text("失焦到Settings ")
			case 17:
				if !quiet(250 * time.Millisecond) {
					break
				}
				if !d.dirty() || d.saveID != 1 {
					fail(errors.New("focus-change policy saved while the editor still retained focus"))
					break
				}
				click("settings")
			case 18:
				if !saved(2) {
					break
				}
				expectedDisk = d.buffer.Text()
				wanted := expectedDisk
				submit(func() error { return readExact(wanted) })
			case 19:
				click("terminal-create")
			case 20:
				t := m.currentTerminal()
				if t != nil && !t.pending && t.session == nil {
					fail(fmt.Errorf("owned terminal startup failed: %s", t.state))
					break
				}
				if t == nil || t.pending {
					break
				}
				click("code-line-0")
			case 21:
				text("失焦到Terminal ")
			case 22:
				if !quiet(250 * time.Millisecond) {
					break
				}
				if !d.dirty() || d.saveID != 2 {
					fail(errors.New("focus-change policy saved before the native terminal click"))
					break
				}
				click("terminal-grid")
			case 23:
				if !saved(3) {
					break
				}
				expectedDisk = d.buffer.Text()
				wanted := expectedDisk
				submit(func() error { return readExact(wanted) })
			case 24:
				click("code-line-0")
			case 25:
				text("失焦到Tab ")
			case 26:
				if !quiet(250 * time.Millisecond) {
					break
				}
				if !d.dirty() || d.saveID != 3 {
					fail(errors.New("focus-change policy saved before the native tab switch"))
					break
				}
				click(m.tabKey(second))
			case 27:
				if !saved(4) || m.current() != second {
					break
				}
				expectedDisk = d.buffer.Text()
				wanted := expectedDisk
				submit(func() error { return readExact(wanted) })
			case 28:
				click("autosave-onWindowChange")
			case 29:
				if m.autoSaveConfig().Mode != "onWindowChange" {
					fail(errors.New("native window-change mode selection failed"))
					break
				}
				wanted := m.autoSaveConfig()
				capture("settings-window-mode", "autosave-onWindowChange", func(winprobe.Window) error { return readConfig(wanted) })
			case 30:
				click(m.tabKey(d))
			case 31:
				click("code-line-0")
			case 32:
				text("窗口等待 ")
			case 33:
				send([3]uintptr{0x215, 0, 0}, [3]uintptr{0x8, 0, 0})
			case 34:
				if !quiet(350 * time.Millisecond) {
					break
				}
				if !d.dirty() || d.saveID != 4 || m.saveBusy || !m.autoSave.windowKnown || !m.autoSave.windowFocused {
					fail(errors.New("capture/keyboard cancellation incorrectly triggered window-change Auto Save"))
					break
				}
				wanted := expectedDisk
				submit(func() error { return readExact(wanted) })
			case 35:
				send([3]uintptr{0x6, 1, 0}, [3]uintptr{0x6, 0, 0})
			case 36:
				if !saved(5) {
					break
				}
				expectedDisk = d.buffer.Text()
				wanted := expectedDisk
				submit(func() error { return readExact(wanted) })
			case 37:
				send([3]uintptr{0x6, 0, 0}, [3]uintptr{0x6, 0, 0})
			case 38:
				if !quiet(350 * time.Millisecond) {
					break
				}
				if d.saveID != 5 || d.dirty() {
					fail(errors.New("repeated inactive activation caused a duplicate automatic save"))
					break
				}
				send([3]uintptr{0x6, 1, 0})
			case 39:
				click("autosave-off")
			case 40:
				if m.autoSaveConfig().Mode != "off" {
					fail(errors.New("native off mode selection failed"))
					break
				}
				policyClick("off", "code-line-0")
			case 41:
				text("关闭保护 ")
			case 42:
				if !quiet(350 * time.Millisecond) {
					break
				}
				if !d.dirty() || d.saveID != 5 {
					fail(errors.New("off policy wrote new dirty text"))
					break
				}
				preservedLocal, preservedVersion = d.buffer.Text(), d.buffer.Version()
				click(m.tabCloseKey(d))
			case 43:
				if !m.closePrompt || m.closeTarget != d {
					fail(errors.New("dirty close did not require confirmation"))
					break
				}
				click("close-cancel")
			case 44:
				if m.closePrompt || !d.dirty() || d.buffer.Text() != preservedLocal || d.buffer.Version() != preservedVersion {
					fail(errors.New("dirty close cancellation lost unsaved text/version"))
					break
				}
				click("menu-File")
			case 45:
				click(menuKey("revertFile"))
			case 46:
				if m.reloadPrompt != d {
					fail(errors.New("File Revert did not ask before discarding dirty edits"))
					break
				}
				b, ok := cx.ElementBounds("reload-cancel")
				if !ok {
					fail(errors.New("Revert cancellation control missing"))
					break
				}
				capture("revert-confirmation", "reload-confirm", func(w winprobe.Window) error { return clickBounds(w, b) })
			case 47:
				if m.reloadPrompt != nil || d.buffer.Text() != preservedLocal || d.buffer.Version() != preservedVersion || !d.dirty() {
					fail(errors.New("Revert Cancel discarded unsaved text or history"))
					break
				}
				click("menu-File")
			case 48:
				click(menuKey("revertFile"))
			case 49:
				click("reload-confirm")
			case 50:
				if m.reloadBusy || m.reloadPrompt != nil || d.dirty() {
					break
				}
				if d.buffer.Text() != expectedDisk || d.buffer.Version() <= preservedVersion || d.saveID != 5 {
					fail(errors.New("confirmed Revert did not reload current disk with a monotonic version and unchanged save receipt"))
					break
				}
				capture("revert-restored", "code-line-0", nil)
			case 51:
				click("autosave-delay")
			case 52:
				setDelay(3, "1000")
			case 53:
				if m.autoSaveConfig().DelayMS != 1000 {
					fail(errors.New("native delay could not restore 1000ms"))
					break
				}
				click("autosave-afterDelay")
			case 54:
				click("code-line-0")
			case 55:
				text("冲突保留 ")
			case 56:
				preservedLocal = d.buffer.Text()
				submit(func() error {
					f, err := os.CreateTemp(root, ".external-*")
					if err != nil {
						return err
					}
					defer os.Remove(f.Name())
					_, err = f.WriteString("// external authoritative disk 世界😀\r\npackage main\r\n")
					err = errors.Join(err, f.Close())
					if err == nil {
						err = filewatch.Replace(ctx, f.Name(), path, nil)
					}
					return err
				})
			case 57:
				if d.diskConflict == nil {
					break
				}
				if !quiet(1100 * time.Millisecond) {
					break
				}
				if d.buffer.Text() != preservedLocal || !d.dirty() || d.saveID != 5 || m.saveBusy {
					fail(errors.New("Auto Save overwrote a conflicted buffer/disk or acknowledged a fabricated save"))
					break
				}
				capture("external-conflict", "disk-reload", nil)
			case 58:
				submit(func() error { return readExact("// external authoritative disk 世界😀\r\npackage main\r\n") })
			case 59:
				click("menu-File")
			case 60:
				click(menuKey("newFile"))
			case 61:
				untitled = m.current()
				if untitled == nil || !untitled.untitled {
					fail(errors.New("native File New Text File did not create untitled editor"))
					break
				}
				click("code-line-0")
			case 62:
				text("untitled unsaved 世界😀")
			case 63:
				if !quiet(1100 * time.Millisecond) {
					break
				}
				if !untitled.untitled || !untitled.dirty() || untitled.saveID != 0 || m.fileActions.busy || m.saveBusy || m.closePrompt || m.reloadPrompt != nil || saveRequests != 5 {
					fail(errors.New("automatic save changed untitled text or opened a Save As/close dialog"))
					break
				}
				click("menu-File")
			case 64:
				click(menuKey("autoSave"))
			case 65:
				if m.autoSaveConfig().Mode != "off" || !d.dirty() || !untitled.dirty() || saveRequests != 5 {
					fail(errors.New("final native File toggle failed to preserve dirty conflict/untitled documents"))
					break
				}
				wanted := m.autoSaveConfig()
				submit(func() error {
					if err := readConfig(wanted); err != nil {
						return err
					}
					return readExact("// external authoritative disk 世界😀\r\npackage main\r\n")
				})
			case 66:
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
	if phase != 66 {
		return fmt.Errorf("native Auto Save acceptance exited at phase %d", phase)
	}
	fmt.Println("native Auto Save acceptance passed: 66 phases, File checkmark/four modes/delay, exact Unicode CRLF disk saves, editor/terminal/tab blur, owned activation/cancellation negatives, no duplicate/untitled saves, dirty close and Revert cancellation/real reload, external conflict, private persisted settings")
	return nil
}
