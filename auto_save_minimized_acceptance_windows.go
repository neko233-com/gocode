//go:build windows && cgo

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

type minimizedAutoSaveState struct {
	Text       string `json:"text"`
	Version    int    `json:"version"`
	SaveID     uint64 `json:"save_id"`
	DidSave    int    `json:"did_save"`
	Dirty      bool   `json:"dirty"`
	SaveBusy   bool   `json:"save_busy"`
	WindowBlur bool   `json:"window_blur"`
	DueNanos   int64  `json:"auto_save_due_unix_nanos,omitempty"`
}

type minimizedAutoSavePhase struct {
	Mode        string                 `json:"mode"`
	Iconic      bool                   `json:"iconic"`
	State       minimizedAutoSaveState `json:"state"`
	ViewsBefore uint64                 `json:"views_before"`
	ViewsAfter  uint64                 `json:"views_after"`
	Before      winprobe.RenderStats   `json:"renderer_before"`
	After       winprobe.RenderStats   `json:"renderer_after"`
}

type minimizedAutoSaveReport struct {
	PID           uint32                   `json:"pid"`
	Phases        []minimizedAutoSavePhase `json:"phases"`
	Restored      bool                     `json:"restored_after_verified_saves"`
	ViewsRestored uint64                   `json:"views_restored"`
	RestoredStats winprobe.RenderStats     `json:"renderer_restored"`
}

// Unlike frame-driven acceptance, this worker advances exclusively through
// Dispatch receipts. No View callback is needed to schedule or acknowledge
// either save while the actual owned HWND remains minimized.
func runMinimizedAutoSaveAcceptance() error {
	previous, existed := os.LookupEnv("GODESKTOP_TEST_INPUT_ISOLATION")
	if err := os.Setenv("GODESKTOP_TEST_INPUT_ISOLATION", "1"); err != nil {
		return err
	}
	defer func() {
		if existed {
			_ = os.Setenv("GODESKTOP_TEST_INPUT_ISOLATION", previous)
		} else {
			_ = os.Unsetenv("GODESKTOP_TEST_INPUT_ISOLATION")
		}
	}()
	root, err := os.MkdirTemp("", "gocode-auto-save-minimized-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	path := filepath.Join(root, "minimized 世界😀.go")
	initial := "// 原始 世界😀\r\npackage main\r\n"
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		return err
	}
	m, err := newWorkspaceModel(root)
	if err != nil {
		return err
	}
	d, err := loadDocument(context.Background(), path)
	if err != nil {
		return err
	}
	m.docs = []*document{d}
	m.rememberDocument(path, d)
	m.focusTab(d)
	m.files = []string{filepath.Base(path)}
	m.autoSave.config = defaultAutoSaveConfig() // Private session only.
	m.keyboard.profile = "vscode"
	m.showPanel = false
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ready := make(chan *ui.Context, 1)
	var current atomic.Pointer[ui.Context]
	var viewCalls atomic.Uint64
	var stopSaves, stopAuto func()
	didSave := 0 // UI-owned; the worker obtains a frozen copy through Dispatch.
	m.onDocument = func(kind string, target *document, _ textbuffer.ChangeEvent) {
		if kind == "save" && target == d {
			didSave++
		}
	}
	type outcome struct {
		report minimizedAutoSaveReport
		err    error
	}
	result := make(chan outcome, 1)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		report := minimizedAutoSaveReport{PID: uint32(os.Getpid())}
		var cx *ui.Context
		select {
		case cx = <-ready:
		case <-ctx.Done():
			result <- outcome{report, ctx.Err()}
			return
		}
		failure := func() error {
			readUI := func(change func()) (minimizedAutoSaveState, error) {
				reply := make(chan minimizedAutoSaveState, 1)
				if !cx.Dispatch(func() {
					if ctx.Err() == nil && change != nil {
						change()
					}
					state := minimizedAutoSaveState{Text: d.buffer.Text(), Version: d.buffer.Version(), SaveID: d.saveID, DidSave: didSave, Dirty: d.dirty(), SaveBusy: m.saveBusy, WindowBlur: m.autoSave.windowKnown && !m.autoSave.windowFocused}
					if entry := m.autoSave.pending[d]; entry != nil && !entry.due.IsZero() {
						state.DueNanos = entry.due.UnixNano()
					}
					reply <- state
				}) {
					return minimizedAutoSaveState{}, errors.New("owned UI rejected minimized acceptance dispatch")
				}
				select {
				case state := <-reply:
					return state, ctx.Err()
				case <-ctx.Done():
					return minimizedAutoSaveState{}, ctx.Err()
				}
			}
			pause := func() error {
				timer := time.NewTimer(20 * time.Millisecond)
				defer timer.Stop()
				select {
				case <-timer.C:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			var window winprobe.Window
			for {
				var findErr error
				window, findErr = winprobe.Find("gocode — "+filepath.Base(root), uint32(os.Getpid()))
				if findErr == nil && winprobe.RendererStats().Submitted >= 1 {
					break
				}
				if err := pause(); err != nil {
					return fmt.Errorf("initial owned HWND/GPU: %w", err)
				}
			}
			isIconic := syscall.NewLazyDLL("user32.dll").NewProc("IsIconic")
			iconic := func() bool { value, _, _ := isIconic.Call(uintptr(window)); return value != 0 }
			if err := window.Send(0x6, 1, 0); err != nil { // Isolated WM_ACTIVATE/WA_ACTIVE.
				return err
			}
			first, err := readUI(func() {
				m.configureAutoSave(autoSaveConfig{Mode: "onWindowChange", DelayMS: 200})
				m.replaceSelection(d, "// window blur 保存😀\r\n")
			})
			if err != nil || !first.Dirty || first.SaveID != 0 {
				return fmt.Errorf("prepare window-change snapshot: %+v: %w", first, err)
			}
			cx.Minimize()
			for !iconic() {
				if err := pause(); err != nil {
					return fmt.Errorf("actual IsIconic minimization: %w", err)
				}
			}
			viewsBefore, statsBefore := viewCalls.Load(), winprobe.RendererStats()
			if err := window.Send(0x6, 0, 0); err != nil { // Explicit replay; physical focus is isolated.
				return err
			}
			waitSaved := func(wanted minimizedAutoSaveState, count uint64) (minimizedAutoSaveState, error) {
				for {
					if !iconic() {
						return minimizedAutoSaveState{}, errors.New("owned HWND restored before save verification")
					}
					body, diskErr := os.ReadFile(path)
					state, uiErr := readUI(nil)
					if uiErr != nil {
						return state, uiErr
					}
					if state.Text != wanted.Text || state.Version != wanted.Version {
						return state, errors.New("immutable expected source changed during minimized save")
					}
					if diskErr == nil && string(body) == wanted.Text && !state.Dirty && !state.SaveBusy && state.SaveID == count && state.DidSave == int(count) {
						return state, nil
					}
					if err := pause(); err != nil {
						return state, fmt.Errorf("minimized disk/UI save receipt %+v: %w", state, err)
					}
				}
			}
			saved, err := waitSaved(first, 1)
			if err != nil || !saved.WindowBlur {
				return fmt.Errorf("window-change actual minimized save: %+v: %w", saved, err)
			}
			appendPhase := func(mode string, state minimizedAutoSaveState) error {
				after, viewsAfter := winprobe.RendererStats(), viewCalls.Load()
				if !iconic() || viewsAfter != viewsBefore || after.Submitted != statsBefore.Submitted || after.Backend != "direct3d12" {
					return fmt.Errorf("minimized %s invoked View/Present: views %d→%d, submitted %d→%d, iconic=%t", mode, viewsBefore, viewsAfter, statsBefore.Submitted, after.Submitted, iconic())
				}
				report.Phases = append(report.Phases, minimizedAutoSavePhase{Mode: mode, Iconic: true, State: state, ViewsBefore: viewsBefore, ViewsAfter: viewsAfter, Before: statsBefore, After: after})
				return nil
			}
			if err := appendPhase("onWindowChange", saved); err != nil {
				return err
			}
			second, err := readUI(func() {
				m.configureAutoSave(autoSaveConfig{Mode: "afterDelay", DelayMS: 200})
				m.replaceSelection(d, "// delay while minimized 世界😀\r\n")
			})
			if err != nil || !second.Dirty || second.SaveID != 1 {
				return fmt.Errorf("prepare minimized delayed snapshot: %+v: %w", second, err)
			}
			if body, err := os.ReadFile(path); err != nil {
				return err
			} else if second.DueNanos != 0 && time.Now().Before(time.Unix(0, second.DueNanos)) && string(body) != first.Text {
				return errors.New("delayed snapshot saved before its actual UI-owned deadline")
			}
			saved, err = waitSaved(second, 2)
			if err != nil {
				return err
			}
			// Keep the window minimized after success, proving duplicate timer/
			// UI wakes do not create additional writes, Views or GPU submissions.
			quiet := time.Now().Add(300 * time.Millisecond)
			for time.Now().Before(quiet) {
				state, err := readUI(nil)
				if err != nil || !iconic() || state.Dirty || state.SaveID != 2 || state.DidSave != 2 {
					return fmt.Errorf("minimized success was not stable: %+v: %w", state, err)
				}
				if err := pause(); err != nil {
					return err
				}
			}
			if err := appendPhase("afterDelay", saved); err != nil {
				return err
			}
			window.Show(9) // SW_RESTORE only after both actual saves are proven.
			for {
				stats, views := winprobe.RendererStats(), viewCalls.Load()
				if !iconic() && views > viewsBefore && stats.Submitted > statsBefore.Submitted {
					report.Restored, report.ViewsRestored, report.RestoredStats = true, views, stats
					break
				}
				if err := pause(); err != nil {
					return fmt.Errorf("verified save restored GPU workbench: %w", err)
				}
			}
			return nil
		}()
		result <- outcome{report, failure}
		cx.Quit()
	}()
	defer func() {
		cancel()
		if stopAuto != nil {
			stopAuto()
		}
		if stopSaves != nil {
			stopSaves()
		}
		<-workerDone
		m.closeDocuments()
	}()
	// This guard asks only the owned UI to quit; the parent test also has a
	// kill-on-close process job, so a regression cannot leave a window behind.
	watchdog := time.AfterFunc(35*time.Second, func() {
		if cx := current.Load(); cx != nil {
			cx.Quit()
		}
	})
	defer watchdog.Stop()
	started := false
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(root), Width: 960, Height: 640, Background: ui.RGB(editor), CustomTitlebar: true, Input: m.input, CloseRequested: m.requestWindowClose}, func(cx *ui.Context) *ui.Element {
		viewCalls.Add(1)
		if !started {
			m.native = cx
			current.Store(cx)
			stopSaves = m.startDocumentSaves(ctx, cx)
			stopAuto = m.startAutoSaveActor(ctx, cx.Dispatch)
			started = true
			ready <- cx
		}
		return m.view(cx)
	})
	cancel()
	var finished outcome
	select {
	case finished = <-result:
	case <-time.After(3 * time.Second):
		return errors.Join(err, errors.New("minimized acceptance worker did not stop"))
	}
	if err = errors.Join(err, finished.err); err != nil {
		return err
	}
	reportPath := os.Getenv("GOCODE_AUTOSAVE_MINIMIZED_REPORT")
	if reportPath == "" {
		reportPath = filepath.Join(".cache", "auto-save-minimized", "current.json")
	}
	body, err := json.MarshalIndent(finished.report, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(reportPath), 0755); err != nil {
		return err
	}
	if err = os.WriteFile(reportPath, append(body, '\n'), 0600); err != nil {
		return err
	}
	fmt.Println("native minimized Auto Save acceptance passed: actual IsIconic + onWindowChange/afterDelay disk/didSave receipts + zero minimized View/GPU submissions + restored native workbench")
	return nil
}
