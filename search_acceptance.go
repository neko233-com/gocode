package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	ui "github.com/neko233-com/godesktop"
)

func runSearchAcceptance(mib int) error {
	if mib < 16 || mib > 10240 {
		return errors.New("native search fixture must be 16–10240 MiB")
	}
	if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
		return err
	}
	workspace, err := os.MkdirTemp("", "gocode-search-native-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workspace)
	fixtures := map[string]string{"a.txt": "first line\r\n😀界 needle and NEEDLE and needlex\r\n", "nested/阅读说明.md": "intro\nsecond needle word\n", "ignored/secret.txt": "needle\n", ".gitignore": "ignored/\n", "binary.dat": "\x00needle"}
	for name, body := range fixtures {
		path := filepath.Join(workspace, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			return err
		}
	}
	largePath := filepath.Join(workspace, "huge.log")
	f, err := os.Create(largePath)
	if err != nil {
		return err
	}
	size := int64(mib) << 20
	block := bytes.Repeat([]byte("x"), 128<<10)
	for off := int64(0); off < size; off += int64(len(block)) {
		if _, err := f.Write(block); err != nil {
			f.Close()
			return err
		}
	}
	if _, err := f.WriteAt([]byte(" 😀needle \n"), size-256); err != nil {
		f.Close()
		return err
	}
	f.Close()
	hashFile := func() ([32]byte, error) {
		f, err := os.Open(largePath)
		if err != nil {
			return [32]byte{}, err
		}
		defer f.Close()
		h := sha256.New()
		_, err = io.CopyBuffer(h, f, make([]byte, 128<<10))
		var sum [32]byte
		copy(sum[:], h.Sum(nil))
		return sum, err
	}
	initialHash, err := hashFile()
	if err != nil {
		return err
	}
	m, err := newWorkspaceModel(workspace)
	if err != nil {
		return err
	}
	defer m.closeDocuments()
	m.logo, err = loadWorkbenchLogo()
	if err != nil {
		return err
	}
	d, err := loadDocument(context.Background(), filepath.Join(workspace, "a.txt"))
	if err != nil {
		return err
	}
	m.docs = append(m.docs, d)
	m.active = 0
	m.editing = true
	m.showPanel = false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stopSearch, stopOpens, stopIcon func()
	defer func() {
		if stopSearch != nil {
			stopSearch()
		}
		if stopOpens != nil {
			stopOpens()
		}
		if stopIcon != nil {
			stopIcon()
		}
	}()
	watchdog := time.AfterFunc(90*time.Second, func() { fmt.Fprintln(os.Stderr, "native search acceptance timed out"); os.Exit(2) })
	defer watchdog.Stop()
	phase, waiting, nextFrame := 0, false, uint64(7)
	paintGeneration, paintAfter := uint64(0), uint64(0)
	paintMessage := ""
	var failure error
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(workspace), Width: 1280, Height: 820, Background: ui.RGB(editor), CustomTitlebar: true, Input: m.input}, func(cx *ui.Context) *ui.Element {
		m.native = cx
		if stopSearch == nil {
			stopOpens = m.startFileOpens(ctx, cx.Dispatch, nil)
			stopSearch = m.startSearch(ctx, cx.Dispatch, nil)
			stopIcon = applyAppIcon("gocode — " + filepath.Base(workspace))
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
			b, ok := cx.ElementBounds(key)
			if !ok {
				done(fmt.Errorf("native search control missing: %s", key))
				return
			}
			x, y := b.X+b.Width/2, b.Y+b.Height/2
			tabsNativePointer(cx, workspace, x, y, true, func(err error) {
				if err != nil {
					done(err)
					return
				}
				tabsNativePointer(cx, workspace, x, y, false, done)
			})
		}
		setField := func(key, value string, done func(error)) {
			click(key, func(err error) {
				if err != nil {
					done(err)
					return
				}
				tabsNativeKey(cx, workspace, 'A', ui.ModifierControl, true, func(err error) {
					if err != nil {
						done(err)
						return
					}
					searchNativeText(cx, workspace, value, done)
				})
			})
		}
		ready := !m.search.busy && m.search.timer == nil
		if ready && len(m.search.report.Matches) > 0 && paintGeneration != m.search.generation {
			paintGeneration = m.search.generation
			paintAfter = cx.RenderedFrames() + 5
		}
		ready = ready && cx.RenderedFrames() >= paintAfter
		if m.message != paintMessage {
			paintMessage = m.message
			nextFrame = max(nextFrame, cx.RenderedFrames()+5)
		}
		if !waiting && failure == nil && cx.RenderedFrames() >= nextFrame {
			switch phase {
			case 0:
				waiting = true
				searchNativeText(cx, workspace, "unsaved needle ", advance)
			case 1:
				waiting = true
				tabsNativeKey(cx, workspace, 'F', ui.ModifierControl|ui.ModifierShift, true, advance)
			case 2:
				waiting = true
				searchNativeText(cx, workspace, "needle", func(err error) {
					if err != nil {
						advance(err)
						return
					}
					tabsNativeKey(cx, workspace, 13, 0, true, advance)
				})
			case 3:
				if ready && len(m.search.report.Matches) > 0 {
					if m.search.report.Err != nil || len(m.search.report.Matches) != 6 {
						fail(fmt.Errorf("native literal search expected 6 including huge file, got %d: %v", len(m.search.report.Matches), m.search.report.Err))
						break
					}
					if err := verifySearchPixels(cx, m, "literal-results"); err != nil {
						fail(err)
						break
					}
					waiting = true
					click("search-word", advance)
				}
			case 4:
				if ready && len(m.search.report.Matches) > 0 {
					if len(m.search.report.Matches) != 5 {
						fail(fmt.Errorf("whole-word results=%d", len(m.search.report.Matches)))
						break
					}
					waiting = true
					click("search-case", advance)
				}
			case 5:
				if ready && len(m.search.report.Matches) > 0 {
					if len(m.search.report.Matches) != 4 {
						fail(fmt.Errorf("case-sensitive results=%d", len(m.search.report.Matches)))
						break
					}
					waiting = true
					click("search-regex", advance)
				}
			case 6:
				if ready && len(m.search.report.Matches) > 0 {
					waiting = true
					setField("search-query", "n[ae]edle", func(err error) {
						if err != nil {
							advance(err)
							return
						}
						tabsNativeKey(cx, workspace, 13, 0, true, advance)
					})
				}
			case 7:
				if ready && len(m.search.report.Matches) > 0 {
					if len(m.search.report.Matches) != 4 {
						fail(fmt.Errorf("regex results=%d", len(m.search.report.Matches)))
						break
					}
					if err := verifySearchPixels(cx, m, "regex-results"); err != nil {
						fail(err)
						break
					}
					waiting = true
					click(fmt.Sprintf("search-result-%d-1", m.search.generation), advance)
				}
			case 8:
				if strings.HasPrefix(m.message, "Search: a.txt") {
					if m.current() != d || d.buffer.Selection().Range() != m.search.report.Matches[1].Range || d.buffer.Selection().Range().Start.Character != 4 {
						fail(errors.New("native Unicode result did not select its UTF-16 range"))
						break
					}
					if err := verifySearchPixels(cx, m, "selected-unicode"); err != nil {
						fail(err)
						break
					}
					waiting = true
					go func() {
						err := os.WriteFile(filepath.Join(workspace, "nested", "阅读说明.md"), []byte("changed on disk\n"), 0600)
						cx.Dispatch(func() {
							if err != nil {
								advance(err)
								return
							}
							index := len(m.search.report.Matches) - 1
							click(fmt.Sprintf("search-result-%d-%d", m.search.generation, index), advance)
						})
					}()
				}
			case 9:
				if strings.Contains(m.message, "Search result is stale") {
					if m.current().buffer.Selection().Anchor.Character != 0 {
						fail(errors.New("stale disk result moved selection"))
						break
					}
					if err := verifySearchPixels(cx, m, "stale-result"); err != nil {
						fail(err)
						break
					}
					waiting = true
					go func() {
						err := os.WriteFile(filepath.Join(workspace, "nested", "阅读说明.md"), []byte(fixtures["nested/阅读说明.md"]), 0600)
						cx.Dispatch(func() {
							if err != nil {
								advance(err)
								return
							}
							m.removeTab(m.active)
							click("search-regex", advance)
						})
					}()
				}
			case 10:
				waiting = true
				setField("search-query", "needle", func(err error) {
					if err != nil {
						advance(err)
						return
					}
					setField("search-include", "huge.log", func(err error) {
						if err != nil {
							advance(err)
							return
						}
						tabsNativeKey(cx, workspace, 13, 0, true, advance)
					})
				})
			case 11:
				if ready && len(m.search.report.Matches) > 0 {
					if len(m.search.report.Matches) != 1 || m.search.report.Matches[0].Start != size-251 {
						fail(errors.New("large-file search offset incorrect"))
						break
					}
					if err := verifySearchPixels(cx, m, "large-result"); err != nil {
						fail(err)
						break
					}
					waiting = true
					click(fmt.Sprintf("search-result-%d-0", m.search.generation), advance)
				}
			case 12:
				current := m.current()
				if current != nil && current.large != nil && current.large.byteMode && !current.large.loading && current.large.requested && strings.Contains(current.large.window.Text, "needle") {
					if current.large.byteOffset != size-251 {
						fail(errors.New("native large search navigation offset incorrect"))
						break
					}
					if err := verifySearchPixels(cx, m, "large-navigated"); err != nil {
						fail(err)
						break
					}
					phase = 13
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
	if phase != 13 {
		return fmt.Errorf("native search incomplete phase=%d status=%s message=%s", phase, m.search.status, m.message)
	}
	for name, want := range fixtures {
		data, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		if string(data) != want {
			return fmt.Errorf("native search changed source %s", name)
		}
	}
	afterHash, err := hashFile()
	if err != nil {
		return err
	}
	if afterHash != initialHash {
		return errors.New("native search modified large source")
	}
	fmt.Printf("Native search passed: real disk/unsaved text, case/word/regex/ignore, UTF-16 selection, stale-result protection, %d MiB single-line search/navigation and unchanged source\n", mib)
	return nil
}

func verifySearchPixels(cx *ui.Context, m *model, stage string) error {
	pixels, scale, err := captureWorkbenchGPU(cx, m.workspace)
	if err != nil {
		return err
	}
	directory := os.Getenv("GOCODE_SEARCH_SCREENSHOTS")
	if directory == "" {
		directory = filepath.Join(".cache", "search-native")
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
	viewport, ok := cx.ElementBounds("search-results")
	if !ok {
		return errors.New("native search results viewport missing")
	}
	count, ink := 0, 0
	for y := int(float64(viewport.Y) * scale); y < min(pixels.Bounds().Dy(), int(float64(viewport.Y+viewport.Height)*scale)); y++ {
		for x := int(float64(viewport.X) * scale); x < min(pixels.Bounds().Dx(), int(float64(viewport.X+viewport.Width)*scale)); x++ {
			p := pixels.RGBAAt(x, y)
			if p.R > 70 && int(p.R) > int(p.G)+25 && p.B < 60 {
				ink++
			}
		}
	}
	for i := range m.search.report.Matches {
		if b, ok := cx.ElementBounds(fmt.Sprintf("search-result-%d-%d", m.search.generation, i)); ok {
			count++
			if b.X < viewport.X-.01 || b.X+b.Width > viewport.X+viewport.Width+.01 {
				return errors.New("search hit target escapes its native viewport")
			}
		}
	}
	if count == 0 || count > 40 || ink < 5 {
		return fmt.Errorf("%s missing completed search match pixels/targets: visible=%d ink=%d", stage, count, ink)
	}
	metadata := struct {
		Stage                           string
		Results, Visible, Width, Height int
		Scale                           float64
		Status                          string
	}{stage, len(m.search.report.Matches), count, pixels.Bounds().Dx(), pixels.Bounds().Dy(), scale, m.search.status}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, stage+".json"), append(data, '\n'), 0600)
}
