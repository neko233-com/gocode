//go:build windows && cgo

package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf16"

	"github.com/neko233-com/gocode/internal/uidispatch"
	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/extensions"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

type extensionDetailNativeReport struct {
	PID             uint32   `json:"pid"`
	Version         string   `json:"version"`
	Source          string   `json:"source"`
	Gates           []string `json:"gates"`
	DescriptionRows int      `json:"description_rows"`
	Commands        int      `json:"manifest_commands"`
	NativeFontWidth float32  `json:"native_font_width"`
	MaxMeasuredLine float32  `json:"max_measured_line"`
	ViewportWidth   float32  `json:"viewport_width"`
	BottomScroll    float32  `json:"bottom_scroll"`
	ExecutedCommand string   `json:"executed_command"`
	DiskSHA256      string   `json:"unchanged_disk_sha256"`
	Backend         string   `json:"renderer_backend"`
	Submitted       uint64   `json:"submitted_frames"`
}

type extensionDetailNativeState struct {
	frame                                uint64
	detail, tab, profile, activity, text string
	scroll, maxScroll                    float32
	sidebarScroll                        int
	rows                                 int
	maxLine, fontWidth                   float32
	dirty, busy, disabled, focused       bool
	bounds                               map[string]ui.Bounds
	keyTrace                             string
}

func installExtensionDetailFixture(root string) (extensions.Extension, error) {
	commands := make([]map[string]string, 2000)
	for i := range commands {
		commands[i] = map[string]string{"command": fmt.Sprintf("details.command.%d", i), "title": fmt.Sprintf("Native contribution %d · 中文 é 👩‍👩‍👧‍👦", i)}
	}
	manifest, err := json.Marshal(map[string]any{"publisher": "fixture", "name": "native-details", "displayName": "Native Details Fixture", "version": "0.1.0", "description": "START " + strings.Repeat("Native description with Unicode 世界 é 👩‍👩‍👧‍👦 and proportional font words. ", 100) + " END", "main": "extension.cjs", "engines": map[string]string{"vscode": "^1.140.0"}, "activationEvents": []string{"*"}, "contributes": map[string]any{"commands": commands}})
	if err != nil {
		return extensions.Extension{}, err
	}
	file, err := os.Create(filepath.Join(root, "native-details.vsix"))
	if err != nil {
		return extensions.Extension{}, err
	}
	w := zip.NewWriter(file)
	for _, entry := range []struct {
		name string
		data []byte
	}{{"extension/package.json", manifest}, {"extension/extension.cjs", []byte(`const v=require('vscode');exports.activate=c=>{for(let i=0;i<2000;i++)c.subscriptions.push(v.commands.registerCommand('details.command.'+i,()=> 'executed:'+i));};`)}} {
		part, e := w.Create(entry.name)
		if e == nil {
			_, e = part.Write(entry.data)
		}
		if e != nil {
			_ = w.Close()
			_ = file.Close()
			return extensions.Extension{}, e
		}
	}
	if err = errors.Join(w.Close(), file.Close()); err != nil {
		return extensions.Extension{}, err
	}
	return extensions.Install(filepath.Join(root, "extensions"), filepath.Join(root, "native-details.vsix"))
}

func runExtensionDetailAcceptance() error {
	for _, name := range []string{"GODESKTOP_TEST_INPUT_ISOLATION", "GODESKTOP_READBACK"} {
		old, exists := os.LookupEnv(name)
		if err := os.Setenv(name, "1"); err != nil {
			return err
		}
		defer func() {
			if exists {
				_ = os.Setenv(name, old)
			} else {
				_ = os.Unsetenv(name)
			}
		}()
	}
	root, err := os.MkdirTemp("", "gocode-extension-detail-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	const original = "package main\r\n// original source 世界😀\r\n"
	source := filepath.Join(root, "main.go")
	if err = os.WriteFile(source, []byte(original), 0600); err != nil {
		return err
	}
	fixture, err := installExtensionDetailFixture(root)
	if err != nil {
		return err
	}
	m, err := newWorkspaceModel(root)
	if err != nil {
		return err
	}
	defer m.closeDocuments()
	d, err := loadDocument(context.Background(), source)
	if err != nil {
		return err
	}
	m.docs = []*document{d}
	m.rememberDocument(source, d)
	m.focusTab(d)
	m.files = []string{"main.go"}
	m.showPanel = false
	m.installed = extensionInfos([]extensions.Extension{fixture})
	m.extensionsView.contributions = extensionContributions([]extensions.Extension{fixture})
	m.extensionsView.running = map[string]bool{fixture.ID(): true}
	m.showExtensions()
	m.focusExtensionDetails(fixture.ID(), "Details")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	host, err := extensions.Start(ctx, root, []extensions.Extension{fixture})
	if err != nil {
		return err
	}
	eventsDone := make(chan struct{})
	hostErrors := make(chan string, 1)
	go func() {
		defer close(eventsDone)
		for event := range host.Events {
			if event.Type == "error" {
				select {
				case hostErrors <- event.Text:
				default:
				}
			}
		}
	}()
	defer func() { cancel(); _ = host.Close(); <-eventsDone }()
	if err = host.Call(ctx, "initialize", map[string]any{"storageRoot": filepath.Join(root, "host-state")}, nil); err != nil {
		return err
	}
	executed := make(chan string, 1)
	m.execute = func(command string) {
		select {
		case executed <- command:
		default:
			m.message = "extension detail command queue overflow"
		}
	}
	reportPath := os.Getenv("GOCODE_EXTENSION_DETAIL_REPORT")
	if reportPath == "" {
		reportPath = filepath.Join(".cache", "extension-detail", "current.json")
	}
	reportPath, err = filepath.Abs(reportPath)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(reportPath), 0700); err != nil {
		return err
	}
	ready := make(chan *ui.Context, 1)
	var active atomic.Pointer[ui.Context]
	var stopManager func()
	keyTrace := "no native key received" // UI-owned, frozen in readUI receipts.
	result := make(chan error, 1)
	workerDone := make(chan struct{})
	watchdog := time.AfterFunc(60*time.Second, func() { fmt.Fprintln(os.Stderr, "native extension detail acceptance timed out"); os.Exit(2) })
	defer watchdog.Stop()
	go func() {
		<-ctx.Done()
		if cx := active.Load(); cx != nil {
			cx.Quit()
		}
	}()
	go func() {
		defer close(workerDone)
		var cx *ui.Context
		select {
		case cx = <-ready:
		case <-ctx.Done():
			result <- ctx.Err()
			return
		}
		report := extensionDetailNativeReport{PID: uint32(os.Getpid()), Version: appVersion(), Source: buildCommit(), Commands: 2000}
		stage := "initial-layout"
		err := func() error {
			readUI := func(change func()) (extensionDetailNativeState, error) {
				reply := make(chan extensionDetailNativeState, 1)
				if !uidispatch.Retry(ctx, cx.Dispatch, func() {
					if ctx.Err() != nil {
						return
					}
					if change != nil {
						change()
					}
					s := &m.extensionsView.detailUI
					state := extensionDetailNativeState{frame: cx.RenderedFrames(), detail: m.extensionsView.detail, tab: m.extensionsView.tab, profile: m.keymapProfile(), activity: m.activity, text: d.buffer.Text(), scroll: s.scroll, maxScroll: s.maxScroll(), sidebarScroll: m.extensionsView.scroll, rows: len(s.plan.rows), fontWidth: nativeExtensionTextWidth("Wiii 世界", 13), dirty: d.dirty(), busy: m.extensionsView.busy, disabled: containsExtension(m.extensionsView.settings.Disabled, fixture.ID()), focused: s.focused, bounds: make(map[string]ui.Bounds)}
					state.keyTrace = keyTrace
					for _, row := range s.plan.rows {
						if row.command < 0 {
							state.maxLine = max(state.maxLine, nativeExtensionTextWidth(row.text, row.size))
						}
					}
					for _, key := range []string{"extension-detail-editor", "extension-detail-header", "extension-detail-viewport", "extension-detail-content", "extension-toggle", "extension-tab-Details", "extension-tab-Feature Contributions", "extension-detail-close", "extensions-sidebar", "extension-detail-command-details.command.1999"} {
						if b, ok := cx.ElementBounds(key); ok {
							state.bounds[key] = b
						}
					}
					reply <- state
				}) {
					return extensionDetailNativeState{}, ctx.Err()
				}
				select {
				case state := <-reply:
					return state, nil
				case <-ctx.Done():
					return extensionDetailNativeState{}, ctx.Err()
				}
			}
			waitUI := func(predicate func(extensionDetailNativeState) bool) (extensionDetailNativeState, error) {
				var last extensionDetailNativeState
				nextDiagnostic := time.Now().Add(5 * time.Second)
				for {
					state, e := readUI(nil)
					if e != nil {
						return state, fmt.Errorf("%w; last native state: %+v", e, last)
					}
					last = state
					if predicate(state) {
						return state, nil
					}
					if time.Now().After(nextDiagnostic) {
						fmt.Fprintf(os.Stderr, "extension detail pending %s: frame=%d detail=%q tab=%q scroll=%g max=%g font=%g rows=%d viewport=%+v\n", stage, state.frame, state.detail, state.tab, state.scroll, state.maxScroll, state.fontWidth, state.rows, state.bounds["extension-detail-viewport"])
						nextDiagnostic = time.Now().Add(5 * time.Second)
					}
					timer := time.NewTimer(20 * time.Millisecond)
					select {
					case <-timer.C:
					case <-ctx.Done():
						timer.Stop()
						return state, fmt.Errorf("%w; last native state: %+v", ctx.Err(), state)
					}
				}
			}
			state, err := waitUI(func(s extensionDetailNativeState) bool {
				return s.frame >= 3 && s.fontWidth > 0 && s.rows > 10 && s.bounds["extension-detail-viewport"].Height > 0
			})
			if err != nil {
				return err
			}
			window, err := winprobe.Find("gocode — "+filepath.Base(root), uint32(os.Getpid()))
			if err != nil {
				return err
			}
			viewport := state.bounds["extension-detail-viewport"]
			if state.dirty || state.text != original || state.maxLine > viewport.Width-2*extensionDetailPadding-12+.1 {
				return errors.New("native measured metadata overflows its actual viewport or changed source")
			}
			report.DescriptionRows, report.NativeFontWidth, report.MaxMeasuredLine, report.ViewportWidth = state.rows, state.fontWidth, state.maxLine, viewport.Width
			report.Gates = append(report.Gates, "native-font-and-bounded-description")
			stage = "pointer-wheel"
			capture := func(name string) error {
				pixels, e := window.Capture()
				if e != nil {
					return e
				}
				file, e := os.Create(filepath.Join(filepath.Dir(reportPath), name+".png"))
				if e != nil {
					return e
				}
				return errors.Join(png.Encode(file, pixels), file.Close())
			}
			if err = capture("details"); err != nil {
				return err
			}
			click := func(b ui.Bounds) error {
				if b.Width <= 0 || b.Height <= 0 {
					return errors.New("native detail control is not visible")
				}
				if e := window.Pointer(0x201, int(b.X+b.Width/2), int(b.Y+b.Height/2)); e != nil {
					return e
				}
				return window.Pointer(0x202, int(b.X+b.Width/2), int(b.Y+b.Height/2))
			}
			key := func(k, mods int) error {
				done := make(chan error, 1)
				tabsNativeKey(cx, root, k, mods, true, func(e error) { done <- e })
				select {
				case e := <-done:
					if e != nil {
						return e
					}
				case <-ctx.Done():
					return ctx.Err()
				}
				tabsNativeKey(cx, root, k, mods, false, func(e error) { done <- e })
				select {
				case e := <-done:
					return e
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			if err = click(viewport); err != nil {
				return err
			}
			header := state.bounds["extension-detail-header"]
			if err = window.Wheel(false, int(viewport.X+viewport.Width/2), int(viewport.Y+viewport.Height/2), -120, 0); err != nil {
				return err
			}
			state, err = waitUI(func(s extensionDetailNativeState) bool { return s.scroll >= extensionDetailLine })
			if err != nil {
				return err
			}
			if state.bounds["extension-detail-header"] != header {
				return errors.New("extension header moved with content wheel")
			}
			report.Gates = append(report.Gates, "owned-pointer-wheel-fixed-header")
			stage = "outside-wheel-other-activity"
			before := state.scroll
			outside := state.bounds["extensions-sidebar"]
			if err = window.Wheel(false, int(outside.X+40), int(outside.Y+120), -120, 0); err != nil {
				return err
			}
			state, err = readUI(nil)
			if err != nil {
				return err
			}
			if state.scroll != before {
				return errors.New("sidebar wheel scrolled extension detail")
			}
			if err = window.Wheel(false, int(header.X+20), int(header.Y+20), -120, 0); err != nil {
				return err
			}
			state, err = readUI(func() { m.activity = "files" })
			if err != nil {
				return err
			}
			if state.scroll != before {
				return errors.New("header wheel scrolled extension content")
			}
			if err = window.Wheel(false, int(viewport.X+100), int(viewport.Y+100), -120, 0); err != nil {
				return err
			}
			state, err = waitUI(func(s extensionDetailNativeState) bool { return s.scroll > before && s.activity == "files" })
			if err != nil {
				return err
			}
			report.Gates = append(report.Gates, "outside-wheel-isolation-and-other-activity")
			stage = "resize-description"
			for _, character := range utf16.Encode([]rune("No document edit 世界😀")) {
				if err = window.Send(0x102, uintptr(character), 0); err != nil {
					return err
				}
			}
			state, err = readUI(nil)
			if err != nil {
				return err
			}
			if state.text != original || state.dirty {
				return errors.New("focused native extension detail edited source")
			}
			if err = window.Resize(900, 760); err != nil {
				return err
			}
			state, err = waitUI(func(s extensionDetailNativeState) bool {
				return s.bounds["extension-detail-viewport"].Width < viewport.Width-50 && s.maxLine <= s.bounds["extension-detail-viewport"].Width-2*extensionDetailPadding-12+.1
			})
			if err != nil {
				return err
			}
			if err = capture("narrow"); err != nil {
				return err
			}
			report.Gates = append(report.Gates, "native-resize-rewrap-no-document-edit")
			stage = "contribution-tab"
			if err = click(state.bounds["extension-tab-Feature Contributions"]); err != nil {
				return err
			}
			state, err = waitUI(func(s extensionDetailNativeState) bool {
				return s.tab == "Feature Contributions" && s.scroll == 0 && s.frame > state.frame
			})
			if err != nil {
				return err
			}
			if err = key(35, 0); err != nil {
				return err
			}
			endState, e := readUI(nil)
			if e != nil {
				return e
			}
			if endState.scroll == 0 {
				return fmt.Errorf("native End did not scroll detail: %s", endState.keyTrace)
			}
			stage = "last-contribution"
			state, err = waitUI(func(s extensionDetailNativeState) bool {
				return s.scroll > 1000 && s.bounds["extension-detail-command-details.command.1999"].Height > 0
			})
			if err != nil {
				return err
			}
			report.BottomScroll = state.scroll
			if err = click(state.bounds["extension-detail-command-details.command.1999"]); err != nil {
				return err
			}
			select {
			case command := <-executed:
				var response string
				if err = host.Call(ctx, "execute", map[string]string{"command": command}, &response); err != nil {
					return err
				}
				if command != "details.command.1999" || response != "executed:1999" {
					return fmt.Errorf("native callback/real VSIX command mismatch %q %q", command, response)
				}
				report.ExecutedCommand = command
			case <-ctx.Done():
				return ctx.Err()
			}
			if err = capture("contributions-bottom"); err != nil {
				return err
			}
			report.Gates = append(report.Gates, "last-of-2000-contributions-real-node-callback")
			stage = "disable-enable"
			if err = click(state.bounds["extension-toggle"]); err != nil {
				return err
			}
			state, err = waitUI(func(s extensionDetailNativeState) bool { return s.disabled && !s.busy })
			if err != nil {
				return err
			}
			// A worker receipt can precede the new native tree. Two old GPU
			// submissions may still complete, so three subsequent completions
			// prove that the Enable button and its callback were rebuilt.
			disabledReceiptFrame := state.frame
			state, err = waitUI(func(s extensionDetailNativeState) bool {
				return s.disabled && !s.busy && s.frame >= disabledReceiptFrame+3
			})
			if err != nil {
				return err
			}
			settings, err := readExtensionSettings(filepath.Join(root, "extensions"))
			if err != nil || !containsExtension(settings.Disabled, fixture.ID()) {
				return errors.New("native disable did not persist real private extension settings")
			}
			if err = click(state.bounds["extension-toggle"]); err != nil {
				return err
			}
			state, err = waitUI(func(s extensionDetailNativeState) bool { return !s.disabled && !s.busy })
			if err != nil {
				return err
			}
			enabledReceiptFrame := state.frame
			state, err = waitUI(func(s extensionDetailNativeState) bool {
				return !s.disabled && !s.busy && s.frame >= enabledReceiptFrame+3
			})
			if err != nil {
				return err
			}
			settings, err = readExtensionSettings(filepath.Join(root, "extensions"))
			if err != nil || containsExtension(settings.Disabled, fixture.ID()) {
				return errors.New("native enable did not persist real private extension settings")
			}
			report.Gates = append(report.Gates, "actual-disable-enable-worker-callbacks")
			stage = "resize-bottom"
			if err = window.Resize(1120, 980); err != nil {
				return err
			}
			state, err = waitUI(func(s extensionDetailNativeState) bool {
				return s.bounds["extension-detail-viewport"].Width > 650 && s.scroll <= s.maxScroll+.1
			})
			if err != nil {
				return err
			}
			report.Gates = append(report.Gates, "native-resize-clamps-bottom")
			for _, profile := range []string{"vscode", "jetbrains"} {
				stage = "keymap-" + profile
				state, err = readUI(func() { m.keyboard.profile = profile; m.focusExtensionDetails(fixture.ID(), "Details") })
				if err != nil {
					return err
				}
				state, err = waitUI(func(s extensionDetailNativeState) bool {
					return s.profile == profile && s.tab == "Details" && s.scroll == 0 && s.frame > state.frame
				})
				if err != nil {
					return err
				}
				if err = key(34, 0); err != nil {
					return err
				}
				state, err = waitUI(func(s extensionDetailNativeState) bool { return s.scroll > 0 })
				if err != nil {
					return err
				}
				closeKey := int('W')
				if profile == "jetbrains" {
					closeKey = 115
				}
				if err = key(closeKey, ui.ModifierControl); err != nil {
					return err
				}
				state, err = waitUI(func(s extensionDetailNativeState) bool { return s.detail == "" && s.scroll == 0 && !s.focused })
				if err != nil {
					return err
				}
				if state.text != original || state.dirty {
					return errors.New("native keymap close changed source")
				}
				report.Gates = append(report.Gates, "native-"+profile+"-page-close-reset")
			}
			select {
			case message := <-hostErrors:
				return fmt.Errorf("real VSIX error: %s", message)
			default:
			}
			body, err := os.ReadFile(source)
			if err != nil || string(body) != original {
				return errors.New("native detail fixture changed real disk source")
			}
			digest := sha256.Sum256(body)
			report.DiskSHA256 = hex.EncodeToString(digest[:])
			stats := winprobe.RendererStats()
			report.Backend, report.Submitted = stats.Backend, stats.Submitted
			if stats.Backend != "direct3d12" || stats.Submitted < 3 {
				return errors.New("native detail did not submit actual D3D12 frames")
			}
			report.Gates = append(report.Gates, "unchanged-real-disk-and-owned-d3d12")
			encoded, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			if len(encoded) > 4096 {
				return errors.New("native detail report exceeds bound")
			}
			file, err := os.CreateTemp(filepath.Dir(reportPath), ".extension-detail-report-")
			if err != nil {
				return err
			}
			defer os.Remove(file.Name())
			_, err = file.Write(append(encoded, '\n'))
			err = errors.Join(err, file.Sync(), file.Close())
			if err != nil {
				return err
			}
			return os.Rename(file.Name(), reportPath)
		}()
		if err != nil {
			err = fmt.Errorf("native extension detail stage %s: %w", stage, err)
		}
		result <- err
		cx.Quit()
	}()
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(root), Width: 1280, Height: 820, CustomTitlebar: true, Background: ui.RGB(editor), Input: func(cx *ui.Context, e ui.InputEvent) bool {
		if e.Kind == ui.KeyPressed {
			keyTrace = fmt.Sprintf("key=%d mods=%d beforeFocused=%t editing=%t terminal=%t chat=%t searchFocused=%t menu=%q palette=%t beforeScroll=%g content=%g rows=%d", e.Key, e.Modifiers, m.extensionsView.detailUI.focused, m.editing, m.terminalFocused, m.chatFocused, m.extensionsView.focused, m.menu.name, m.palette, m.extensionsView.detailUI.scroll, m.extensionsView.detailUI.contentHeight, len(m.extensionsView.detailUI.plan.rows))
		}
		consumed := m.input(cx, e)
		if e.Kind == ui.KeyPressed {
			keyTrace += fmt.Sprintf(" consumed=%t afterFocused=%t afterScroll=%g", consumed, m.extensionsView.detailUI.focused, m.extensionsView.detailUI.scroll)
		}
		return consumed
	}}, func(cx *ui.Context) *ui.Element {
		if active.Load() == nil {
			active.Store(cx)
			m.native = cx
			stopManager = m.startExtensionManager(ctx, cx, filepath.Join(root, "extensions"))
			ready <- cx
		}
		return m.view(cx)
	})
	cancel()
	<-workerDone
	if stopManager != nil {
		stopManager()
	}
	if err != nil {
		return err
	}
	if err = <-result; err != nil {
		return err
	}
	fmt.Println("native extension detail acceptance passed: actual fonts/independent viewport/2000 contributions/real VSIX callback/two keymaps/unchanged disk")
	return nil
}
