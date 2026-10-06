package main

import (
	"context"
	"errors"
	"time"
)

const maxExtensionSyncJobs = 128

var errExtensionSyncStopped = errors.New("extension document synchronization stopped")
var errExtensionSyncFull = errors.New("extension document synchronization queue is full")

type extensionSyncJob struct {
	params map[string]any
	ack    chan error
}
type extensionDocumentSync struct {
	ctx               context.Context
	jobs              chan extensionSyncJob
	initialized, done chan struct{}
}

// Notifications and receipt barriers share one FIFO. A command/provider can
// await all earlier native document events without blocking the UI or holding
// this worker across the extension's nested native RPCs.
func startExtensionDocumentSync(ctx context.Context, initialize func(context.Context) error, synchronize func(context.Context, map[string]any) error, failed func(error)) *extensionDocumentSync {
	q := &extensionDocumentSync{ctx: ctx, jobs: make(chan extensionSyncJob, maxExtensionSyncJobs), initialized: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(q.done)
		request, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := initialize(request)
		cancel()
		close(q.initialized)
		if err != nil {
			failed(err)
			return
		}
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-q.jobs:
				if job.ack != nil {
					job.ack <- nil
					continue
				}
				request, cancel := context.WithTimeout(ctx, 10*time.Second)
				err := synchronize(request, job.params)
				cancel()
				if err != nil {
					failed(err)
					return
				}
			}
		}
	}()
	return q
}
func (q *extensionDocumentSync) push(job extensionSyncJob) error {
	if err := q.ctx.Err(); err != nil {
		return err
	}
	select {
	case <-q.done:
		return errExtensionSyncStopped
	default:
	}
	select {
	case q.jobs <- job:
		return nil
	case <-q.done:
		return errExtensionSyncStopped
	case <-q.ctx.Done():
		return q.ctx.Err()
	default:
		return errExtensionSyncFull
	}
}
func (q *extensionDocumentSync) await(ctx context.Context, dispatch func(func()) bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	reply := make(chan error, 1)
	if !dispatch(func() {
		if err := ctx.Err(); err != nil {
			reply <- err
			return
		}
		if err := q.push(extensionSyncJob{ack: reply}); err != nil {
			reply <- err
		}
	}) {
		return errors.New("native workbench closed before extension synchronization")
	}
	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-q.ctx.Done():
		return q.ctx.Err()
	case <-q.done:
		return errExtensionSyncStopped
	}
}
