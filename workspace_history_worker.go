package main

import (
	"context"
	"sync/atomic"
	"time"
)

type historyWork struct {
	ctx    context.Context
	cancel context.CancelFunc
	run    func(context.Context) error
	done   func(error)
}

// One dedicated worker and one latest queue avoid cancelling search jobs.
// All request/state methods and receipts belong to the UI thread.
func (m *model) startHistory(parent context.Context, dispatch func(func()) bool) func() {
	ctx, stop := context.WithCancel(parent)
	queue := make(chan historyWork, 1)
	finished := make(chan struct{})
	var closed atomic.Bool
	var current context.CancelFunc
	m.history.submit = func(run func(context.Context) error, done func(error)) {
		if ctx.Err() != nil {
			return
		}
		if current != nil {
			current()
		}
		jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		current = cancel
		job := historyWork{jobCtx, cancel, run, done}
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
				err := job.run(job.ctx)
				if err == nil {
					err = job.ctx.Err()
				}
				job.cancel()
				if !dispatch(func() {
					if !closed.Load() && ctx.Err() == nil {
						job.done(err)
					}
				}) {
					return
				}
			}
		}
	}()
	return func() {
		closed.Store(true)
		stop()
		if current != nil {
			current()
		}
		m.history.generation++
		m.history.busy = false
		m.history.prompt = nil
		m.history.submit = nil
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
		}
	}
}
