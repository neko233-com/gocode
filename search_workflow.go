package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	workspaceSearch "github.com/neko233-com/gocode/internal/search"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

type searchRow struct {
	file   bool
	result int
}
type searchState struct {
	query                  workspaceSearch.Query
	report                 workspaceSearch.Report
	rows                   []searchRow
	resultRows             []int
	status                 string
	busy                   bool
	generation             uint64
	navigation             uint64
	focus, caret, selected int
	selectAll, initialized bool
	scroll                 float32
	timer                  *time.Timer
	submit                 func()
	validate               func(func(context.Context) error, func(error))
	cancel                 func()
}
type searchWork struct {
	ctx    context.Context
	cancel context.CancelFunc
	run    func(context.Context) workspaceSearch.Report
	done   func(workspaceSearch.Report)
}

func (m *model) startSearch(parent context.Context, dispatch func(func()) bool, run func(context.Context, string, workspaceSearch.Query, map[string]workspaceSearch.Overlay) workspaceSearch.Report) func() {
	if run == nil {
		run = workspaceSearch.Run
	}
	if !m.search.initialized {
		m.search.initialized = true
		m.search.focus = -2
		m.search.selected = -1
		m.search.query.UseIgnore = true
	}
	ctx, stop := context.WithCancel(parent)
	queue := make(chan searchWork, 1)
	finished := make(chan struct{})
	var closed atomic.Bool
	var current context.CancelFunc
	request := func(work func(context.Context) workspaceSearch.Report, done func(workspaceSearch.Report)) {
		if ctx.Err() != nil {
			return
		}
		if current != nil {
			current()
		}
		jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		current = cancel
		job := searchWork{jobCtx, cancel, work, done}
		select {
		case old := <-queue:
			old.cancel()
		default:
		}
		select {
		case queue <- job:
		case <-ctx.Done():
			cancel()
		}
	}
	go func() {
		defer close(finished)
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-queue:
				result := job.run(job.ctx)
				job.cancel()
				if !dispatch(func() {
					if !closed.Load() && ctx.Err() == nil {
						job.done(result)
					}
				}) {
					return
				}
			}
		}
	}()
	m.search.cancel = func() {
		if current != nil {
			current()
		}
	}
	m.search.submit = func() {
		if m.search.timer != nil {
			m.search.timer.Stop()
			m.search.timer = nil
		}
		m.search.generation++
		generation := m.search.generation
		m.search.report = workspaceSearch.Report{}
		m.search.rows = nil
		m.search.resultRows = nil
		m.search.selected = -1
		m.search.scroll = 0
		if m.search.query.Text == "" {
			m.search.busy = false
			m.search.status = ""
			if current != nil {
				current()
			}
			return
		}
		overlays := make(map[string]workspaceSearch.Overlay)
		for _, d := range m.docs {
			if d.buffer == nil {
				continue
			}
			relative, err := filepath.Rel(m.workspace, d.path)
			if err != nil || !filepath.IsLocal(relative) {
				continue
			}
			if len(overlays) >= 128 {
				m.search.status = "Search exceeds 128 open-document snapshots"
				m.search.busy = false
				return
			}
			m.tabKey(d)
			overlays[filepath.ToSlash(relative)] = workspaceSearch.Overlay{Snapshot: d.buffer.Snapshot(), Identity: d.tabID}
		}
		root, query := m.workspace, m.search.query
		m.search.busy = true
		m.search.status = "Searching…"
		request(func(c context.Context) workspaceSearch.Report { return run(c, root, query, overlays) }, func(result workspaceSearch.Report) {
			if generation != m.search.generation {
				return
			}
			m.search.busy = false
			m.search.report = result
			m.search.status = searchStatus(result)
			previous := ""
			for i, match := range result.Matches {
				if match.Path != previous {
					m.search.rows = append(m.search.rows, searchRow{true, i})
					previous = match.Path
				}
				m.search.resultRows = append(m.search.resultRows, len(m.search.rows))
				m.search.rows = append(m.search.rows, searchRow{false, i})
			}
		})
	}
	m.search.validate = func(work func(context.Context) error, done func(error)) {
		request(func(c context.Context) workspaceSearch.Report { return workspaceSearch.Report{Err: work(c)} }, func(r workspaceSearch.Report) { done(r.Err) })
	}
	return func() {
		closed.Store(true)
		if m.search.timer != nil {
			m.search.timer.Stop()
			m.search.timer = nil
		}
		stop()
		select {
		case old := <-queue:
			old.cancel()
		default:
		}
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
		}
		m.search.submit = nil
		m.search.validate = nil
		m.search.cancel = nil
	}
}

func searchStatus(r workspaceSearch.Report) string {
	files := map[string]bool{}
	for _, m := range r.Matches {
		files[m.Path] = true
	}
	resultWord, fileWord := "results", "files"
	if len(r.Matches) == 1 {
		resultWord = "result"
	}
	if len(files) == 1 {
		fileWord = "file"
	}
	status := fmt.Sprintf("%d %s in %d %s", len(r.Matches), resultWord, len(files), fileWord)
	if r.Err != nil {
		status += " · " + r.Err.Error()
	}
	if r.Limited {
		status += " · limit reached"
	}
	if r.Unreadable+r.Changed > 0 {
		status += fmt.Sprintf(" · %d skipped/changed", r.Unreadable+r.Changed)
	}
	return status
}
func (m *model) searchChanged(cx *ui.Context) {
	m.search.generation++
	generation := m.search.generation
	if m.search.cancel != nil {
		m.search.cancel()
	}
	if m.search.timer != nil {
		m.search.timer.Stop()
	}
	m.search.report = workspaceSearch.Report{}
	m.search.rows = nil
	m.search.resultRows = nil
	m.search.selected = -1
	m.search.busy = false
	m.search.status = ""
	m.search.scroll = 0
	if cx != nil && m.search.query.Text != "" {
		m.search.timer = time.AfterFunc(200*time.Millisecond, func() {
			cx.Dispatch(func() {
				if generation == m.search.generation && m.search.submit != nil {
					m.search.submit()
				}
			})
		})
	}
}
func (m *model) stopSearch() {
	m.search.generation++
	if m.search.timer != nil {
		m.search.timer.Stop()
		m.search.timer = nil
	}
	if m.search.cancel != nil {
		m.search.cancel()
	}
	m.search.busy = false
	m.search.status = "Search cancelled"
}

func (m *model) activateSearchResult(cx *ui.Context, index int) {
	if index < 0 || index >= len(m.search.report.Matches) || m.search.validate == nil {
		return
	}
	result := m.search.report.Matches[index]
	generation := m.search.generation
	m.search.selected = index
	m.search.navigation++
	navigation := m.search.navigation
	m.search.focus = -2
	m.terminalFocused, m.chatFocused, m.updateFocused = false, false, false
	m.openThen(context.Background(), filepath.Join(m.workspace, filepath.FromSlash(result.Path)), func(d *document, err error) {
		if err != nil || d == nil || d != m.current() || generation != m.search.generation {
			return
		}
		if result.Identity != 0 && (d.tabID != result.Identity || d.buffer == nil || d.buffer.Version() != result.Version) {
			m.message = "Search result is stale; search again"
			return
		}
		var verify func(context.Context) error
		focusSequence := m.openSequence
		version := 0
		if d.buffer != nil {
			snapshot := d.buffer.Snapshot()
			version = snapshot.Version
			verify = func(c context.Context) error {
				text := snapshot.Text()
				return workspaceSearch.Verify(c, strings.NewReader(text), int64(len(text)), result)
			}
		} else {
			root, path := m.workspace, result.Path
			verify = func(c context.Context) error {
				fs, err := os.OpenRoot(root)
				if err != nil {
					return err
				}
				defer fs.Close()
				f, err := fs.Open(filepath.FromSlash(path))
				if err != nil {
					return err
				}
				defer f.Close()
				info, err := f.Stat()
				if err != nil {
					return err
				}
				return workspaceSearch.Verify(c, f, info.Size(), result)
			}
		}
		m.search.validate(verify, func(err error) {
			if generation != m.search.generation || navigation != m.search.navigation || d != m.current() || focusSequence != m.openSequence {
				return
			}
			if err != nil {
				m.message = "Search result is stale: " + err.Error()
				return
			}
			if d.buffer != nil {
				if d.buffer.Version() != version {
					m.message = "Document changed before search navigation"
					return
				}
				if err := d.buffer.SetSelection(textbuffer.Selection{Anchor: result.Range.Start, Active: result.Range.End}); err != nil {
					m.message = err.Error()
					return
				}
				d.followSelection()
				d.scroll = result.Range.Start.Line
				m.editing = true
				m.documentEvent("selection", d, textbuffer.ChangeEvent{})
			} else {
				m.navigateLarge(d, 0, result.Start, true)
				m.editing = true
			}
			m.message = "Search: " + result.Path
			if cx != nil {
				cx.Invalidate()
			}
		})
	})
}
