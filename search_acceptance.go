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
	"sync/atomic"
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
	started := time.Now()
	var activeContext atomic.Pointer[ui.Context]
	var progress atomic.Pointer[searchAcceptanceProgress]
	var timedOut atomic.Bool
	watchdog := time.AfterFunc(90*time.Second, func() {
		timedOut.Store(true)
		data, _ := json.Marshal(progress.Load())
		fmt.Fprintf(os.Stderr, "native search acceptance timed out; last=%s\n", data)
		if cx := activeContext.Load(); cx != nil {
			cx.Quit()
		}
	})
	defer watchdog.Stop()
	phase, waiting, nextFrame := 0, false, uint64(7)
	paintGeneration, paintAfter := uint64(0), uint64(0)
	paintMessage := ""
	var failure error
	var navigation *searchAcceptanceNavigation
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(workspace), Width: 1280, Height: 820, Background: ui.RGB(editor), CustomTitlebar: true, Input: m.input}, func(cx *ui.Context) *ui.Element {
		m.native = cx
		quitBeforeWork := publishSearchAcceptanceContext(&activeContext, &timedOut, cx)
		defer func() {
			progress.Store(snapshotSearchAcceptance(m, navigation, started, phase, waiting, cx.RenderedFrames(), nextFrame, paintGeneration, paintAfter))
		}()
		if quitBeforeWork {
			cx.Quit()
			return ui.Stack()
		}
		if stopSearch == nil {
			stopOpens = m.startFileOpens(ctx, cx.Dispatch, nil)
			stopSearch = m.startSearch(ctx, cx.Dispatch, nil)
			navigation = bindSearchAcceptanceNavigation(m, started)
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
			if ready && m.search.report.Err != nil && (phase >= 3 && phase <= 7 || phase == 11) {
				fail(fmt.Errorf("native search query failed at phase %d: %w", phase, m.search.report.Err))
			}
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
					if err := verifySearchPixels(cx, m, "literal-results", started); err != nil {
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
					if err := verifySearchPixels(cx, m, "regex-results", started); err != nil {
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
					if err := verifySearchPixels(cx, m, "selected-unicode", started); err != nil {
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
					if err := verifySearchPixels(cx, m, "stale-result", started); err != nil {
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
					if err := verifySearchPixels(cx, m, "large-result", started); err != nil {
						fail(err)
						break
					}
					waiting = true
					navigation.arm(m, largePath, m.search.report.Matches[0].Start)
					click(fmt.Sprintf("search-result-%d-0", m.search.generation), advance)
				}
			case 12:
				if err := navigation.failure(m); err != nil {
					fail(err)
					break
				}
				current := m.current()
				if current != nil && current.large != nil && current.large.byteMode && !current.large.loading && current.large.requested && strings.Contains(current.large.window.Text, "needle") {
					if current.large.byteOffset != size-251 {
						fail(errors.New("native large search navigation offset incorrect"))
						break
					}
					if err := verifySearchPixels(cx, m, "large-navigated", started); err != nil {
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
	activeContext.Store(nil)
	if navigation != nil {
		navigation.current = nil
	}
	if searchAcceptanceExpired(started, time.Now(), timedOut.Load()) {
		data, _ := json.Marshal(progress.Load())
		return fmt.Errorf("native search acceptance timed out after Run; last=%s; runError=%v", data, err)
	}
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
	if searchAcceptanceExpired(started, time.Now(), timedOut.Load()) {
		data, _ := json.Marshal(progress.Load())
		return fmt.Errorf("native search acceptance timed out after final source hash; last=%s", data)
	}
	data, _ := json.Marshal(progress.Load())
	fmt.Printf("Native search progress: %s\n", data)
	fmt.Printf("Native search passed: real disk/unsaved text, case/word/regex/ignore, UTF-16 selection, stale-result protection, %d MiB single-line search/navigation and unchanged source\n", mib)
	return nil
}

func verifySearchPixels(cx *ui.Context, m *model, stage string, started time.Time) error {
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
		ElapsedMS                       int64
	}{stage, len(m.search.report.Matches), count, pixels.Bounds().Dx(), pixels.Bounds().Dy(), scale, m.search.status, time.Since(started).Milliseconds()}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, stage+".json"), append(data, '\n'), 0600)
}

// These observations belong only to this fixture's UI thread. Production work
// and callbacks are forwarded unchanged; worker timing crosses through an
// immutable atomic value, never a model/document pointer.
type searchAcceptanceNavigation struct {
	current *searchAcceptanceNavigationRequest
	started time.Time
}

type searchAcceptanceNavigationRequest struct {
	generation, navigation                        uint64
	path                                          string
	offset                                        int64
	document                                      *document
	openStarted, openDone, verifyQueued, verified bool
	openErr, verifyErr                            error
	focusSequence                                 uint64
	queuedMS, openReceiptMS, verifyReceiptMS      int64
	work                                          atomic.Pointer[searchAcceptanceVerifyProgress]
}

type searchAcceptanceVerifyProgress struct {
	StartedMS, FinishedMS int64
	Error                 string
}

func bindSearchAcceptanceNavigation(m *model, started time.Time) *searchAcceptanceNavigation {
	observer := &searchAcceptanceNavigation{started: started}
	open, validate := m.requestOpen, m.search.validate
	m.requestOpen = func(ctx context.Context, path string, done func(*document, error)) {
		request := observer.current
		tracked := request != nil && request.generation == m.search.generation && request.navigation == m.search.navigation && request.path == pathKey(m.lexicalPath(path))
		if tracked {
			request.openStarted = true
			request.queuedMS = time.Since(started).Milliseconds()
		}
		open(ctx, path, func(d *document, err error) {
			if tracked && observer.current == request {
				request.openDone, request.document, request.openErr = true, d, err
				request.openReceiptMS = time.Since(started).Milliseconds()
			}
			if done != nil {
				done(d, err)
			}
		})
	}
	m.search.validate = func(work func(context.Context) error, done func(error)) {
		request := observer.current
		tracked := request != nil && request.openDone && request.openErr == nil && request.document == m.current() && request.generation == m.search.generation && request.navigation == m.search.navigation
		if !tracked {
			validate(work, done)
			return
		}
		request.verifyQueued, request.focusSequence = true, m.openSequence
		timeline := &request.work
		validate(func(ctx context.Context) error {
			began := searchAcceptanceVerifyProgress{StartedMS: time.Since(started).Milliseconds()}
			timeline.Store(&began)
			err := work(ctx)
			finished := began
			finished.FinishedMS = time.Since(started).Milliseconds()
			if err != nil {
				finished.Error = err.Error()
			}
			timeline.Store(&finished)
			return err
		}, func(err error) {
			if observer.current == request {
				request.verified, request.verifyErr = true, err
				request.verifyReceiptMS = time.Since(started).Milliseconds()
			}
			done(err)
		})
	}
	return observer
}

func (s *searchAcceptanceNavigation) arm(m *model, path string, offset int64) {
	s.current = &searchAcceptanceNavigationRequest{generation: m.search.generation, navigation: m.search.navigation + 1, path: pathKey(m.lexicalPath(path)), offset: offset}
}

func (s *searchAcceptanceNavigation) failure(m *model) error {
	request := s.current
	if request == nil || !request.openStarted {
		return nil // A missing/pending receipt remains bounded by the real watchdog.
	}
	if request.generation != m.search.generation || request.navigation != m.search.navigation {
		return fmt.Errorf("native search navigation superseded for %s: expected query/navigation %d/%d, got %d/%d", request.path, request.generation, request.navigation, m.search.generation, m.search.navigation)
	}
	if request.verifyQueued && (request.document != m.current() || request.focusSequence != m.openSequence) {
		return fmt.Errorf("native search Verify focus superseded for %s: expected focus %d, got %d", request.path, request.focusSequence, m.openSequence)
	}
	if request.openErr != nil {
		return fmt.Errorf("native search open failed for %s: %w", request.path, request.openErr)
	}
	if request.verifyErr != nil {
		return fmt.Errorf("native search Verify failed for %s: %w", request.path, request.verifyErr)
	}
	if request.openDone && request.document == nil {
		return fmt.Errorf("native search open returned no document for %s", request.path)
	}
	if d := request.document; request.verified && d != nil && d == m.current() && d.large != nil {
		page := d.large
		// A replacement request retains its old error while loading. Only the
		// acknowledged byte page for this navigation owns a definitive error.
		if page.byteMode && page.byteOffset == request.offset && page.requested && !page.loading && page.err != nil {
			return fmt.Errorf("native search byte page failed for %s: %w", request.path, page.err)
		}
	}
	return nil
}

type searchAcceptanceProgress struct {
	ElapsedMS                                              int64
	Phase                                                  int
	Waiting                                                bool
	Rendered, NextFrame, PaintGeneration, PaintAfter       uint64
	QueryGeneration, Navigation, OpenSequence              uint64
	SearchBusy, OpenBusy                                   bool
	OpeningPath, CurrentPath, Status, Message, SearchError string
	LargeLoading, LargeRequested, ByteMode                 bool
	ByteOffset                                             int64
	LargeError                                             string
	ExpectedGeneration, ExpectedNavigation                 uint64
	TargetPath                                             string
	OpenStarted, OpenDone, VerifyQueued, Verified          bool
	OpenError, VerifyError                                 string
	OpenQueuedMS, OpenReceiptMS, VerifyReceiptMS           int64
	VerifyWork                                             searchAcceptanceVerifyProgress
}

func snapshotSearchAcceptance(m *model, navigation *searchAcceptanceNavigation, started time.Time, phase int, waiting bool, rendered, nextFrame, paintGeneration, paintAfter uint64) *searchAcceptanceProgress {
	snapshot := &searchAcceptanceProgress{ElapsedMS: time.Since(started).Milliseconds(), Phase: phase, Waiting: waiting, Rendered: rendered, NextFrame: nextFrame, PaintGeneration: paintGeneration, PaintAfter: paintAfter, QueryGeneration: m.search.generation, Navigation: m.search.navigation, OpenSequence: m.openSequence, SearchBusy: m.search.busy, OpenBusy: m.openBusy, OpeningPath: m.openingPath, Status: m.search.status, Message: m.message}
	if m.search.report.Err != nil {
		snapshot.SearchError = m.search.report.Err.Error()
	}
	if d := m.current(); d != nil {
		snapshot.CurrentPath = d.path
		if page := d.large; page != nil {
			snapshot.LargeLoading, snapshot.LargeRequested, snapshot.ByteMode, snapshot.ByteOffset = page.loading, page.requested, page.byteMode, page.byteOffset
			if page.err != nil {
				snapshot.LargeError = page.err.Error()
			}
		}
	}
	if navigation != nil && navigation.current != nil {
		request := navigation.current
		snapshot.ExpectedGeneration, snapshot.ExpectedNavigation, snapshot.TargetPath = request.generation, request.navigation, request.path
		snapshot.OpenStarted, snapshot.OpenDone, snapshot.VerifyQueued, snapshot.Verified = request.openStarted, request.openDone, request.verifyQueued, request.verified
		snapshot.OpenQueuedMS, snapshot.OpenReceiptMS, snapshot.VerifyReceiptMS = request.queuedMS, request.openReceiptMS, request.verifyReceiptMS
		if request.openErr != nil {
			snapshot.OpenError = request.openErr.Error()
		}
		if request.verifyErr != nil {
			snapshot.VerifyError = request.verifyErr.Error()
		}
		if timeline := request.work.Load(); timeline != nil {
			snapshot.VerifyWork = *timeline
		}
	}
	return snapshot
}

func searchAcceptanceExpired(started, now time.Time, watchdogReached bool) bool {
	return watchdogReached || now.Sub(started) >= 90*time.Second
}

func publishSearchAcceptanceContext(active *atomic.Pointer[ui.Context], expired *atomic.Bool, cx *ui.Context) bool {
	active.Store(cx)
	// The one-shot timer may expire before the first native View exists. With
	// timer Store->Load and publisher Store->Load, one side must observe Quit.
	return expired.Load()
}
