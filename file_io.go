package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neko233-com/gocode/internal/filewatch"
	"github.com/neko233-com/gocode/internal/languageserver"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

const maxOpenJobs = 32

var errOpenSuperseded = errors.New("opening was cancelled or superseded by newer navigation")

type openJob struct {
	ticket uint64
	group  uint64
	hidden bool
	ctx    context.Context
	path   string
	done   func(*document, error)
}

func pathKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}
func (m *model) lexicalPath(path string) string {
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.workspace, path)
	}
	return filepath.Clean(path)
}
func (m *model) rememberDocument(path string, d *document) {
	if m.pathAliases == nil {
		m.pathAliases = map[string]*document{}
	}
	m.pathAliases[pathKey(m.lexicalPath(path))] = d
	m.pathAliases[pathKey(d.path)] = d
}
func closeUnownedDocument(d *document) {
	if d != nil && d.large != nil {
		d.large.close()
	}
}

// loadDocument runs exclusively on disk workers in the native application.
// Constructed buffers are transferred to the UI once; workers never mutate
// existing live buffers. A cancelled large-file result closes its owned index.
func loadDocument(ctx context.Context, path string) (*document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	physical, err := canonicalPath(path)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := os.Stat(physical)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("Only regular UTF-8 text files can be opened")
	}
	if info.Size() > editableFileLimit {
		d, err := openLargeDocument(physical)
		if err == nil && ctx.Err() != nil {
			closeUnownedDocument(d)
			return nil, ctx.Err()
		}
		return d, err
	}
	result, _ := filewatch.Read(ctx, filewatch.Entry{Path: physical})
	if result.Kind != "text" {
		if result.Err != nil {
			return nil, result.Err
		}
		return nil, errors.New("file is unavailable or changed while opening")
	}
	buffer, err := textbuffer.New(result.Text)
	if err != nil {
		return nil, err
	}
	return &document{path: physical, buffer: buffer, diskHash: result.Hash, diskKnown: true}, nil
}

func (m *model) adoptDocument(requestPath string, loaded *document, focus bool) *document {
	return m.adoptDocumentInGroup(requestPath, loaded, focus, m.groups.active)
}
func (m *model) adoptDocumentInGroup(requestPath string, loaded *document, focus bool, groupID uint64) *document {
	return m.adoptDocumentWithGroup(requestPath, loaded, focus, m.findGroup(groupID))
}
func (m *model) adoptDocumentWithGroup(requestPath string, loaded *document, focus bool, g *editorGroup) *document {
	for _, d := range m.docs {
		if pathKey(d.path) == pathKey(loaded.path) {
			m.rememberDocument(requestPath, d)
			m.addGroupDocument(g, d)
			if focus {
				m.focusTab(d)
			}
			return d
		}
	}
	m.docs = append(m.docs, loaded)
	m.addGroupDocument(g, loaded)
	m.rememberDocument(requestPath, loaded)
	if loaded.large != nil && m.native != nil {
		m.startLargeDocument(loaded)
	}
	m.documentEvent("open", loaded, textbuffer.ChangeEvent{})
	if focus {
		m.focusTab(loaded)
	}
	return loaded
}

// The UI owns the queue/tickets. One disk worker and one pending transfer keep
// input responsive and memory bounded even when an OS read cannot be cancelled.
func (m *model) startFileOpens(parent context.Context, dispatch func(func()) bool, read func(context.Context, string) (*document, error)) func() {
	ctx, cancel := context.WithCancel(parent)
	if read == nil {
		read = loadDocument
	}
	var workers sync.WaitGroup
	var next func()
	next = func() {
		if m.openBusy || len(m.openJobs) == 0 {
			return
		}
		job := m.openJobs[0]
		m.openJobs[0] = openJob{}
		m.openJobs = m.openJobs[1:]
		if err := errors.Join(ctx.Err(), job.ctx.Err()); err != nil {
			if job.done != nil {
				job.done(nil, err)
			}
			next()
			return
		}
		m.openBusy = true
		m.openingPath = job.path
		request, stop := context.WithTimeout(ctx, 30*time.Second)
		var abandoned atomic.Bool
		m.cancelCurrentOpen = func() { abandoned.Store(true); stop() }
		stopRequest := context.AfterFunc(job.ctx, stop)
		workers.Go(func() {
			var loaded *document
			var failure error
			if job.hidden {
				info, err := os.Stat(job.path)
				failure = err
				if err == nil && (!info.Mode().IsRegular() || info.Size() > languageserver.MaxDocumentBytes) {
					failure = errors.New("document exceeds extension UTF-8 text policy")
				}
			}
			if failure == nil {
				loaded, failure = read(request, job.path)
			}
			if failure == nil && job.hidden && !loaded.serviceEligible() {
				failure = errors.New("document exceeds extension UTF-8 text policy")
			}
			// A context timeout after a successful large-file open still owns that
			// result. Dispose it before posting; never leak a discarded index worker.
			if failure == nil {
				failure = errors.Join(request.Err(), job.ctx.Err())
			}
			stopRequest()
			stop()
			if failure != nil {
				closeUnownedDocument(loaded)
				loaded = nil
			}
			// An accepted dispatch can still be dropped during window shutdown.
			// Transfer ownership atomically so cancellation can dispose a pending
			// index without racing a UI callback that has adopted it.
			var ownership atomic.Uint32
			receipt := make(chan *document, 1)
			if !dispatch(func() {
				if !ownership.CompareAndSwap(0, 1) {
					return
				}
				unused := loaded
				defer func() { receipt <- unused }()
				if ctx.Err() != nil {
					return
				}
				err := errors.Join(failure, ctx.Err(), job.ctx.Err())
				if abandoned.Load() {
					err = errors.Join(err, errOpenSuperseded)
				}
				closed := loaded != nil && m.closedDuringOpen[pathKey(loaded.path)] > job.ticket
				if err == nil && closed {
					err = errOpenSuperseded
				}
				if err == nil && !job.hidden && m.groups.root != nil && m.findGroup(job.group) == nil {
					err = errOpenSuperseded
				}
				var d *document
				if err == nil {
					focused := !job.hidden && job.ticket == m.openSequence
					if job.hidden {
						d = m.adoptDocumentWithGroup(job.path, loaded, false, nil)
					} else {
						d = m.adoptDocumentInGroup(job.path, loaded, focused, job.group)
					}
					if d == loaded {
						unused = nil
					}
					if !focused && !job.hidden {
						err = errOpenSuperseded
					}
				}
				if err != nil && job.ticket == m.openSequence {
					m.message = err.Error()
				}
				if job.done != nil {
					job.done(d, err)
				}
			}) {
				closeUnownedDocument(loaded)
				return
			}
			var unused *document
			select {
			case unused = <-receipt:
			case <-ctx.Done():
				if ownership.CompareAndSwap(0, 2) {
					closeUnownedDocument(loaded)
					return
				}
				unused = <-receipt
			}
			// Duplicate/discarded indexes may be waiting for OS reads. Finish
			// disposing this result before starting the next worker.
			closeUnownedDocument(unused)
			dispatch(func() {
				if ctx.Err() != nil {
					return
				}
				m.openBusy = false
				m.openingPath = ""
				m.cancelCurrentOpen = nil
				next()
				if !m.openBusy && len(m.openJobs) == 0 {
					m.closedDuringOpen = nil
				}
			})
		})
	}
	enqueue := func(request context.Context, path string, hidden bool, done func(*document, error)) {
		if err := errors.Join(ctx.Err(), request.Err()); err != nil {
			if done != nil {
				done(nil, err)
			}
			return
		}
		path = m.lexicalPath(path)
		if !hidden {
			m.openSequence++
		}
		ticket := m.openSequence
		if d := m.findDocument(path); d != nil {
			if hidden && !d.serviceEligible() {
				if done != nil {
					done(nil, errors.New("document exceeds extension UTF-8 text policy"))
				}
				return
			}
			if !hidden {
				m.focusTab(d)
			}
			if done != nil {
				done(d, nil)
			}
			return
		}
		if len(m.openJobs) >= maxOpenJobs {
			err := fmt.Errorf("open queue is full (%d); existing documents are preserved", maxOpenJobs)
			m.message = err.Error()
			if done != nil {
				done(nil, err)
			}
			return
		}
		m.openJobs = append(m.openJobs, openJob{ticket: ticket, group: m.groups.active, hidden: hidden, ctx: request, path: path, done: done})
		next()
	}
	m.requestOpen = func(request context.Context, path string, done func(*document, error)) {
		enqueue(request, path, false, done)
	}
	m.requestDocumentOpen = func(request context.Context, path string, done func(*document, error)) {
		enqueue(request, path, true, done)
	}
	m.cancelPendingOpens = func() {
		m.openSequence++
		if m.cancelCurrentOpen != nil {
			m.cancelCurrentOpen()
		}
		jobs := m.openJobs
		m.openJobs = nil
		for _, job := range jobs {
			if job.done != nil {
				job.done(nil, errOpenSuperseded)
			}
		}
	}
	return func() {
		cancel()
		finished := make(chan struct{})
		go func() { workers.Wait(); close(finished) }()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
		}
	}
}

func (m *model) openThen(ctx context.Context, path string, done func(*document, error)) {
	if m.requestOpen != nil {
		m.requestOpen(ctx, path, done)
		return
	}
	if err := ctx.Err(); err != nil {
		if done != nil {
			done(nil, err)
		}
		return
	}
	path = m.lexicalPath(path)
	if d := m.findDocument(path); d != nil {
		m.focusTab(d)
		if done != nil {
			done(d, nil)
		}
		return
	}
	loaded, err := loadDocument(ctx, path)
	if err != nil {
		m.message = err.Error()
		if done != nil {
			done(nil, err)
		}
		return
	}
	d := m.adoptDocument(path, loaded, true)
	if d != loaded {
		closeUnownedDocument(loaded)
	}
	if done != nil {
		done(d, nil)
	}
}
