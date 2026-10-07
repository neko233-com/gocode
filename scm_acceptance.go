package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	gitrepo "github.com/neko233-com/gocode/internal/git"
	ui "github.com/neko233-com/godesktop"
)

func runSCMAcceptance() error {
	if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
		return err
	}
	root, err := os.MkdirTemp("", "gocode-scm-native-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repository, err := gitrepo.Init(ctx, root)
	if err != nil {
		return err
	}
	if err := repository.SetIdentity(ctx, "Gocode acceptance", "acceptance@example.invalid"); err != nil {
		return err
	}
	config := filepath.Join(root, ".git", "config")
	originalConfig, err := os.ReadFile(config)
	if err != nil {
		return err
	}
	fixtureConfig := "\n[commit]\n gpgsign = false\n[core]\n autocrlf = false\n hooksPath = " + strconv.Quote(filepath.ToSlash(filepath.Join(root, "no-hooks"))) + "\n"
	if err := os.WriteFile(config, append(originalConfig, []byte(fixtureConfig)...), 0600); err != nil {
		return err
	}
	path := "main 世界😀.go"
	before := "package main\r\n// ORIGINAL 世界 😀\r\nfunc main() {}\r\n"
	after := "package main\r\n// UPDATED 世界 😀\r\nfunc main() { println(\"native git\") }\r\n"
	file := filepath.Join(root, path)
	if err := os.WriteFile(file, []byte(before), 0600); err != nil {
		return err
	}
	state, err := repository.Status(ctx)
	if err != nil {
		return err
	}
	state, err = repository.ChangeIndex(ctx, state, []string{path}, true)
	if err != nil {
		return err
	}
	state, err = repository.Commit(ctx, state, "Native fixture baseline")
	if err != nil {
		return err
	}
	initialHead := state.Head
	if err := os.WriteFile(file, []byte(after), 0600); err != nil {
		return err
	}
	m, err := newModel(root)
	if err != nil {
		return err
	}
	defer m.closeDocuments()
	m.logo, err = loadWorkbenchLogo()
	if err != nil {
		return err
	}
	var stop, stopIcon func()
	defer func() {
		cancel()
		if stop != nil {
			stop()
		}
		if stopIcon != nil {
			stopIcon()
		}
	}()
	phase, waiting, nextFrame := 0, false, uint64(8)
	awaitedControl := ""
	var awaitingAction bool
	var actionGeneration uint64
	var actionReceipts uint64
	var failure error
	var diagnostic atomic.Value
	diagnostic.Store("starting")
	watchdog := time.AfterFunc(60*time.Second, func() { fmt.Fprintln(os.Stderr, "native SCM timeout:", diagnostic.Load()); os.Exit(2) })
	defer watchdog.Stop()
	var ticking bool
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(root), Width: 1280, Height: 820, CustomTitlebar: true, Background: ui.RGB(editor), Input: m.input}, func(cx *ui.Context) *ui.Element {
		m.native = cx
		if stop == nil {
			stop = m.startSCM(ctx, cx.Dispatch)
			request := m.scm.request
			m.scm.request = func(operation string, paths []string, staged bool) {
				before := m.scm.generation
				request(operation, paths, staged)
				if operation != "refresh" && m.scm.generation > before {
					actionReceipts++
				}
			}
			stopIcon = applyAppIcon("gocode — " + filepath.Base(root))
			m.showSCM()
			m.hidePanel()
		}
		fail := func(err error) { failure = err; cx.Quit() }
		advance := func(err error) {
			if err != nil {
				fail(err)
				return
			}
			waiting = false
			phase++
			nextFrame = cx.RenderedFrames() + 5
			cx.Invalidate()
		}
		click := func(key string) {
			awaitedControl = key
			bounds, ok := cx.ElementBounds(key)
			if !ok {
				// Worker completion updates the model before the new tree has
				// rendered. Wait for that tree's actual control geometry.
				cx.Invalidate()
				return
			}
			awaitedControl = ""
			waiting = true
			// Posting a native pointer event does not acknowledge its callback.
			// Require the real Git worker request before checking its result.
			awaitingAction = key != "scm-message"
			actionGeneration = actionReceipts
			x, y := bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2
			tabsNativePointer(cx, root, x, y, true, func(err error) {
				if err != nil {
					fail(err)
					return
				}
				tabsNativePointer(cx, root, x, y, false, advance)
			})
		}
		if awaitingAction && actionReceipts > actionGeneration {
			awaitingAction = false
		}
		diagnostic.Store(fmt.Sprintf("phase=%d waiting=%t awaiting-action=%t generation=%d action-receipts=%d/%d busy=%t status=%s entries=%d diff=%t control=%s", phase, waiting, awaitingAction, m.scm.generation, actionReceipts, actionGeneration, m.scm.busy, m.scm.status, len(m.scm.snapshot.Entries), m.scm.diff != nil, awaitedControl))
		if !waiting && !awaitingAction && !m.scm.busy && cx.RenderedFrames() >= nextFrame && failure == nil {
			switch phase {
			case 0:
				if len(m.scm.snapshot.Entries) == 1 {
					click("scm-resource-false-" + path)
				}
			case 1:
				if m.scm.diff != nil {
					if m.scm.diff.Before != before || m.scm.diff.After != after {
						fail(errors.New("native diff does not match HEAD/index/disk"))
						break
					}
					if err := captureSCMPixels(cx, m, "working-diff"); err != nil {
						if !errors.Is(err, errSCMPixelsPending) {
							fail(err)
						}
						break
					}
					click("scm-resource-false-" + path + "-index")
				}
			case 2:
				if len(m.scm.snapshot.Entries) != 1 || !m.scm.snapshot.Entries[0].Staged() {
					fail(errors.New("native stage did not change real index"))
					break
				}
				click("scm-resource-true-" + path)
			case 3:
				if m.scm.diff != nil {
					if !m.scm.diff.Staged || m.scm.diff.Before != before || m.scm.diff.After != after {
						fail(errors.New("staged native diff mismatch"))
						break
					}
					if err := captureSCMPixels(cx, m, "staged-diff"); err != nil {
						if !errors.Is(err, errSCMPixelsPending) {
							fail(err)
						}
						break
					}
					click("scm-resource-true-" + path + "-index")
				}
			case 4:
				if len(m.scm.snapshot.Entries) != 1 || m.scm.snapshot.Entries[0].Staged() {
					fail(errors.New("native unstage retained index changes"))
					break
				}
				click("scm-resource-false-" + path + "-index")
			case 5:
				if m.scm.snapshot.Entries[0].Staged() {
					click("scm-message")
				}
			case 6:
				if !m.scm.inputFocused {
					break
				}
				for _, r := range "Native commit 世界 😀" {
					m.input(cx, ui.InputEvent{Kind: ui.Character, Key: int(r)})
				}
				if m.scm.message != "Native commit 世界 😀" {
					fail(errors.New("native commit text did not reach message input"))
					break
				}
				phase++
				nextFrame = cx.RenderedFrames() + 5
			case 7:
				click("scm-commit")
			case 8:
				if m.scm.snapshot.Head == initialHead || len(m.scm.snapshot.Entries) != 0 || m.scm.message != "" {
					fail(fmt.Errorf("native commit did not update actual HEAD/index: generation=%d action-receipts=%d/%d status=%q head-changed=%t entries=%d message=%q", m.scm.generation, actionReceipts, actionGeneration, m.scm.status, m.scm.snapshot.Head != initialHead, len(m.scm.snapshot.Entries), m.scm.message))
					break
				}
				if m.current().buffer.Dirty() || m.current().buffer.Text() != after {
					fail(errors.New("Git action changed editor source"))
					break
				}
				cx.Quit()
			}
		}
		if !ticking {
			ticking = true
			time.AfterFunc(25*time.Millisecond, func() { cx.Dispatch(func() { ticking = false }) })
		}
		return m.view(cx)
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != after {
		return errors.New("native Git actions changed worktree bytes")
	}
	state, err = repository.Status(ctx)
	if err != nil || len(state.Entries) != 0 || state.Head == initialHead {
		return errors.New("final repository not actually committed")
	}
	fmt.Println("Native Git passed: actual HEAD/index/worktree, Unicode/CRLF side-by-side GPU diff, owned pointer stage/unstage/commit, real message input and unchanged source bytes")
	return nil
}

var errSCMPixelsPending = errors.New("SCM pixels pending")

func captureSCMPixels(cx *ui.Context, m *model, stage string) error {
	pixels, scale, err := captureWorkbenchGPU(cx, m.workspace)
	if err != nil {
		return err
	}
	bounds, ok := cx.ElementBounds("scm-diff")
	if !ok {
		return errSCMPixelsPending
	}
	removed, added, ink := 0, 0, 0
	for y := max(0, int(float64(bounds.Y)*scale)); y < min(pixels.Bounds().Dy(), int(float64(bounds.Y+bounds.Height)*scale)); y++ {
		for x := max(0, int(float64(bounds.X)*scale)); x < min(pixels.Bounds().Dx(), int(float64(bounds.X+bounds.Width)*scale)); x++ {
			p := pixels.RGBAAt(x, y)
			if p.R == 0x3e && p.G == 0x20 && p.B == 0x20 {
				removed++
			}
			if p.R == 0x20 && p.G == 0x3a && p.B == 0x26 {
				added++
			}
			if int(p.B) > int(p.R)+15 && p.G > 90 {
				ink++
			}
		}
	}
	if removed < 100 || added < 100 || ink < 20 {
		return errSCMPixelsPending
	}
	directory := os.Getenv("GOCODE_SCM_SCREENSHOTS")
	if directory == "" {
		directory = filepath.Join(".cache", "scm-native")
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(directory, stage+".png"))
	if err != nil {
		return err
	}
	if err := errors.Join(png.Encode(file, pixels), file.Close()); err != nil {
		return err
	}
	metadata, err := json.MarshalIndent(struct {
		Stage               string
		Width, Height       int
		Scale               float64
		Removed, Added, Ink int
		Path, Head          string
	}{stage, pixels.Bounds().Dx(), pixels.Bounds().Dy(), scale, removed, added, ink, m.scm.diff.Path, m.scm.snapshot.Head}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, strings.TrimSuffix(stage, ".png")+".json"), metadata, 0600)
}
