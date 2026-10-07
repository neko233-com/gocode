package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/neko233-com/gocode/internal/uidispatch"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

const maxSaveJobs = 32

var errSaveChanged = errors.New("newer unsaved edits arrived while saving; review and save again")
var errSaveFileOperation = errors.New("a file operation is in progress; save again after it finishes")

type savePlan struct {
	document *document
	path     string
	snapshot textbuffer.Snapshot
	expected *[32]byte
}
type saveJob struct {
	ctx   context.Context
	plans []savePlan
	done  func(error)
}
type savedPlan struct {
	plan savePlan
	hash [32]byte
}

func (m *model) ownsDocument(document *document) bool {
	for _, d := range m.docs {
		if d == document {
			return true
		}
	}
	return false
}

func (m *model) savePlans(documents []*document) ([]savePlan, error) {
	// Save As adopts a new URI only after its immutable disk receipt reaches
	// the UI. Saving the old URI meanwhile could clean a version that the new
	// target has never received, regardless of acknowledgement order.
	if len(documents) != 0 && m.fileActions.busy {
		return nil, errSaveFileOperation
	}
	seen := map[*document]bool{}
	plans := make([]savePlan, 0, len(documents))
	for _, d := range documents {
		if d == nil || d.buffer == nil || !m.ownsDocument(d) {
			return nil, errors.New("no editable document; large-file browsing stays read-only")
		}
		if seen[d] {
			continue
		}
		seen[d] = true
		plans = append(plans, savePlan{document: d, path: d.path, snapshot: d.buffer.Snapshot()})
	}
	return plans, nil
}
func (m *model) saveActive() {
	if m.requestSave == nil {
		m.message = "Save service is unavailable"
		return
	}
	m.requestSave(context.Background(), []*document{m.current()}, func(err error) {
		if err != nil {
			m.message = err.Error()
		}
	})
}

// The UI owns this bounded queue and all acknowledgement/state changes. A single
// worker at a time writes frozen snapshots; successive writes rebase their disk
// expectation only after the previous result has been acknowledged on the UI.
func (m *model) startSaveActor(parent context.Context, dispatch func(func()) bool) func() {
	ctx, cancel := context.WithCancel(parent)
	var workers sync.WaitGroup
	var next func()
	finish := func(job saveJob, saved []savedPlan, failure error) {
		m.saveBusy = false
		for _, result := range saved {
			d := result.plan.document
			if !m.ownsDocument(d) || d.buffer == nil || pathKey(d.path) != pathKey(result.plan.path) {
				failure = errors.Join(failure, errSaveChanged)
				continue
			}
			d.diskHash = result.hash
			d.diskKnown = true
			d.diskConflict = nil
			if d.buffer.Version() == result.plan.snapshot.Version {
				d.buffer.MarkSaved()
				d.saveID++
				m.documentEvent("save", d, textbuffer.ChangeEvent{})
				m.message = "Saved " + filepath.Base(d.path)
			} else {
				failure = errors.Join(failure, errSaveChanged)
			}
		}
		if job.done != nil {
			job.done(failure)
		}
		next()
		if m.publishWatches != nil {
			m.publishWatches()
		}
	}
	next = func() {
		if m.saveBusy || len(m.saveJobs) == 0 {
			return
		}
		job := m.saveJobs[0]
		m.saveJobs = m.saveJobs[1:]
		if err := errors.Join(ctx.Err(), job.ctx.Err()); err != nil {
			if job.done != nil {
				job.done(err)
			}
			next()
			return
		}
		// A completion callback can begin Save As before next drains older
		// accepted jobs. Do not start an original-path writer during that action.
		if len(job.plans) != 0 && m.fileActions.busy {
			if job.done != nil {
				job.done(errSaveFileOperation)
			}
			next()
			return
		}
		for i := range job.plans {
			plan := &job.plans[i]
			if !m.ownsDocument(plan.document) || plan.document.buffer == nil || pathKey(plan.document.path) != pathKey(plan.path) {
				if job.done != nil {
					job.done(errSaveChanged)
				}
				next()
				return
			}
			if plan.document.diskKnown {
				value := plan.document.diskHash
				plan.expected = &value
			}
		}
		m.saveBusy = true
		if m.publishWatches != nil {
			m.publishWatches()
		}
		workers.Go(func() {
			c, stop := context.WithTimeout(ctx, 30*time.Second)
			defer stop()
			stopRequest := context.AfterFunc(job.ctx, stop)
			defer stopRequest()
			saved := make([]savedPlan, 0, len(job.plans))
			var failure error
			for _, plan := range job.plans {
				hash, err := writeDocumentSnapshot(c, plan.path, plan.snapshot, plan.expected)
				if err != nil {
					failure = fmt.Errorf("save %s: %w", filepath.Base(plan.path), err)
					break
				}
				saved = append(saved, savedPlan{plan, hash})
			}
			// A full native UI queue rejects Dispatch without closing the window.
			// Retain this one immutable disk receipt in the existing writer until
			// the UI accepts it; actor shutdown cancels the bounded retry.
			uidispatch.Retry(ctx, dispatch, func() { finish(job, saved, failure) })
		})
	}
	m.requestSave = func(request context.Context, documents []*document, done func(error)) {
		plans, err := m.savePlans(documents)
		if err == nil {
			err = errors.Join(ctx.Err(), request.Err())
		}
		if err == nil && len(m.saveJobs) >= maxSaveJobs {
			err = errors.New("save queue is full; unsaved buffers were preserved")
		}
		if err != nil {
			if done != nil {
				done(err)
			}
			return
		}
		m.saveJobs = append(m.saveJobs, saveJob{ctx: request, plans: plans, done: done})
		next()
	}
	return func() {
		cancel()
		finished := make(chan struct{})
		go func() { workers.Wait(); close(finished) }()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			// A blocked OS write/fsync cannot be interrupted by context. It only
			// owns its immutable snapshot; UI dispatch rejects after window exit.
		}
	}
}
func (m *model) startDocumentSaves(parent context.Context, cx *ui.Context) func() {
	stop := m.startSaveActor(parent, cx.Dispatch)
	m.discardForClose = func() {
		if m.closeBusy {
			return
		}
		m.closeBusy = true
		m.closeError = "Waiting for the active save…"
		// A zero-document job is a barrier behind already accepted disk writes.
		m.requestSave(parent, nil, func(err error) {
			m.closeBusy = false
			if err != nil {
				m.closeError = err.Error()
				return
			}
			m.finishClose(cx)
		})
	}
	m.saveForClose = func() {
		if m.closeBusy {
			return
		}
		documents := []*document{}
		for _, d := range m.docs {
			if d.dirty() && m.closeScope(d) {
				documents = append(documents, d)
			}
		}
		m.closeBusy = true
		m.closeError = "Saving…"
		m.requestSave(parent, documents, func(err error) {
			m.closeBusy = false
			if err != nil {
				m.closeError = err.Error()
				return
			}
			if len(m.closePlans()) != 0 {
				m.closeError = "New unsaved edits arrived; review and retry"
				return
			}
			m.finishClose(cx)
		})
	}
	return stop
}
