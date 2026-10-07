package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"time"

	gitrepo "github.com/neko233-com/gocode/internal/git"
)

type scmState struct {
	repository         *gitrepo.Repository
	snapshot           gitrepo.State
	status, message    string
	busy, inputFocused bool
	generation         uint64
	scroll, diffScroll float32
	diff               *gitrepo.Diff
	diffRows           []diffRow
	request            func(string, []string, bool)
	refresh            func()
	cancel             func()
}
type scmResult struct {
	repository *gitrepo.Repository
	state      gitrepo.State
	diff       *gitrepo.Diff
	rows       []diffRow
	err        error
}
type scmWork struct {
	ctx    context.Context
	cancel context.CancelFunc
	run    func(context.Context) scmResult
	done   func(scmResult)
}

func (m *model) startSCM(parent context.Context, dispatch func(func()) bool) func() {
	ctx, stop := context.WithCancel(parent)
	finished := make(chan struct{})
	queue := make(chan scmWork, 1)
	var closed atomic.Bool
	workspace := m.workspace
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
	m.scm.request = func(operation string, paths []string, staged bool) {
		if m.scm.busy || ctx.Err() != nil {
			return
		}
		m.scm.busy = true
		m.scm.status = "Git: " + operation + "…"
		m.scm.generation++
		generation := m.scm.generation
		expected := m.scm.snapshot
		message := m.scm.message
		focusSequence := m.openSequence
		repository := m.scm.repository
		paths = append([]string(nil), paths...)
		jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		m.scm.cancel = cancel
		job := scmWork{ctx: jobCtx, cancel: cancel, run: func(request context.Context) scmResult {
			if repository == nil {
				var err error
				if operation == "init" {
					repository, err = gitrepo.Init(request, workspace)
				} else {
					repository, err = gitrepo.Open(request, workspace)
				}
				if err != nil {
					return scmResult{err: err}
				}
			}
			result := scmResult{repository: repository}
			var err error
			switch operation {
			case "refresh", "init":
				result.state, err = repository.Status(request)
			case "stage", "unstage":
				result.state, err = repository.ChangeIndex(request, expected, paths, operation == "stage")
			case "commit":
				result.state, err = repository.Commit(request, expected, message)
			case "diff":
				if len(paths) != 1 {
					return scmResult{err: errors.New("select one file to compare")}
				}
				value, diffErr := repository.Diff(request, expected, paths[0], staged)
				err = diffErr
				if err == nil {
					result.diff = &value
					result.rows, err = buildDiffRows(value)
				}
				result.state = expected
			default:
				err = errors.New("unknown Git action")
			}
			result.err = err
			return result
		}, done: func(result scmResult) {
			if m.scm.generation != generation {
				return
			}
			m.scm.busy = false
			m.scm.cancel = nil
			if result.err != nil {
				m.scm.status = result.err.Error()
				if operation != "refresh" {
					m.message = result.err.Error()
				}
				return
			}
			m.scm.repository = result.repository
			m.scm.snapshot = result.state
			m.scm.status = fmt.Sprintf("%s · %d changed files", result.state.Branch, len(result.state.Entries))
			if operation == "commit" {
				m.scm.message = ""
			}
			if result.diff != nil && m.openSequence == focusSequence {
				m.extensionsView.detail = ""
				m.scm.diff = result.diff
				m.scm.diffRows = result.rows
				m.scm.diffScroll = 0
				m.scm.inputFocused = false
				m.editing = false
				m.terminalFocused = false
			}
			if operation == "stage" || operation == "unstage" || operation == "commit" {
				m.scm.diff = nil
				m.scm.diffRows = nil
			}
		}}
		select {
		case queue <- job:
		case <-ctx.Done():
			cancel()
			m.scm.busy = false
		}
	}
	m.scm.refresh = func() {
		if m.scm.request != nil {
			m.scm.request("refresh", nil, false)
		}
	}
	m.scm.refresh()
	var tick func()
	tick = func() {
		if ctx.Err() != nil || closed.Load() {
			return
		}
		time.AfterFunc(2*time.Second, func() {
			dispatch(func() {
				if ctx.Err() == nil && !closed.Load() {
					if m.activity == "source-control" && !m.scm.busy {
						m.scm.refresh()
					}
					tick()
				}
			})
		})
	}
	tick()
	return func() {
		closed.Store(true)
		stop()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
		}
	}
}

func (m *model) scmAction(operation string, paths []string, staged bool) {
	if m.scm.request == nil {
		m.message = "Git workers are starting"
		return
	}
	if operation == "stage" {
		for _, path := range paths {
			absolute := filepath.Join(m.scm.snapshot.Root, filepath.FromSlash(path))
			for _, doc := range m.docs {
				if pathKey(doc.path) == pathKey(absolute) && doc.dirty() {
					m.message = "Save " + filepath.Base(path) + " before staging its disk contents"
					return
				}
			}
		}
	}
	m.scm.request(operation, paths, staged)
}
