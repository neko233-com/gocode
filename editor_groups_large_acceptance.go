package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	ui "github.com/neko233-com/godesktop"
)

func runLargeGroupsAcceptance(mib int) error {
	if mib < 16 || mib > 10240 {
		return errors.New("large split fixture must be 16–10240 MiB")
	}
	if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
		return err
	}
	root, err := os.MkdirTemp("", "gocode-groups-large-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	path := filepath.Join(root, "large.txt")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	hash := sha256.New()
	writer := io.MultiWriter(file, hash)
	begin, tail := "BEGIN independent native group\n", "\nEND independent native group\n"
	if _, err := io.WriteString(writer, begin); err != nil {
		file.Close()
		return err
	}
	chunk := strings.Repeat("bounded native page data\n", 4096)
	remaining := int64(mib) << 20
	remaining -= int64(len(begin) + len(tail))
	for remaining > 0 {
		n := min(int64(len(chunk)), remaining)
		if _, err := io.WriteString(writer, chunk[:n]); err != nil {
			file.Close()
			return err
		}
		remaining -= n
	}
	if _, err := io.WriteString(writer, tail); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	expected := fmt.Sprintf("%x", hash.Sum(nil))
	m, err := newWorkspaceModel(root)
	if err != nil {
		return err
	}
	defer m.closeDocuments()
	d, err := loadDocument(context.Background(), path)
	if err != nil {
		return err
	}
	m.docs = []*document{d}
	m.active = 0
	m.showPanel = false
	m.editing = true
	m.logo, err = loadWorkbenchLogo()
	if err != nil {
		return err
	}
	var diagnostic atomic.Value
	diagnostic.Store("starting")
	watchdog := time.AfterFunc(60*time.Second, func() { fmt.Fprintln(os.Stderr, "native large groups timed out", diagnostic.Load()); os.Exit(2) })
	defer watchdog.Stop()
	phase, waiting, nextFrame := 0, false, uint64(8)
	var failure error
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(root), Width: 1280, Height: 820, CustomTitlebar: true, Background: ui.RGB(editor), Input: m.input}, func(cx *ui.Context) *ui.Element {
		m.native = cx
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
				done(fmt.Errorf("large group control missing %s", key))
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
		key := func(k int, done func(error)) { tabsNativeKey(cx, root, k, ui.ModifierControl, true, done) }
		diagnostic.Store(fmt.Sprintf("phase=%d groups=%d offset=%d reading=%t error=%v", phase, len(m.allGroups()), d.large.window.Offset, d.large.loading, d.large.err))
		if !waiting && failure == nil && cx.RenderedFrames() >= nextFrame {
			switch phase {
			case 0:
				if len(d.large.page) > 0 && !d.large.loading {
					waiting = true
					click("split", advance)
				}
			case 1:
				if len(m.allGroups()) != 2 || m.findGroup(1).views[d].large.file != d.large.file || m.findGroup(1).views[d].large == d.large {
					fail(errors.New("large split does not share only its index"))
					break
				}
				waiting = true
				key('G', advance)
			case 2:
				waiting = true
				searchNativeText(cx, root, fmt.Sprintf(":%d", (int64(mib)<<20)-128), func(err error) {
					if err != nil {
						advance(err)
						return
					}
					tabsNativeKey(cx, root, 13, 0, true, advance)
				})
			case 3:
				left := m.findGroup(1).views[d].large
				if !d.large.loading && strings.Contains(d.large.window.Text, "END independent native group") && len(left.page) > 0 {
					if left.byteMode || left.byteOffset != 0 {
						fail(errors.New("native byte seek moved the other view"))
						break
					}
					if err := captureGroupsPixels(cx, m, "large-independent"); err != nil {
						fail(err)
						break
					}
					waiting = true
					click("close-group", advance)
				}
			case 4:
				if len(m.allGroups()) != 1 || d.largeBase.ctx.Err() != nil || d.large.ctx.Err() != nil || !m.ownsDocument(d) {
					fail(errors.New("closing original view cancelled surviving file reader"))
					break
				}
				waiting = true
				key(36, advance)
			case 5:
				if !d.large.loading && strings.HasPrefix(d.large.window.Text, "BEGIN independent native group") {
					if err := captureGroupsPixels(cx, m, "large-survivor"); err != nil {
						fail(err)
						break
					}
					phase = 6
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
	if phase != 6 {
		return fmt.Errorf("large groups incomplete: %v", diagnostic.Load())
	}
	read, err := os.Open(path)
	if err != nil {
		return err
	}
	defer read.Close()
	hash.Reset()
	if _, err := io.Copy(hash, read); err != nil {
		return err
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != expected {
		return errors.New("large split changed actual source bytes")
	}
	fmt.Printf("Native large editor groups passed: actual %d bytes, shared bounded index, independent first/tail pages, survivor after owner-view close and identical SHA256\n", int64(mib)<<20)
	return nil
}
