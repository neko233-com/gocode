// Package uidispatch keeps worker acknowledgements bounded when the native UI
// dispatch queue temporarily rejects a callback because it is full.
package uidispatch

import (
	"context"
	"time"
)

const retryDelay = 10 * time.Millisecond

// Retry retains one callback in its calling worker until Dispatch accepts it or
// the worker's lifetime context is cancelled. A false Dispatch result can mean
// overload or shutdown, so cancellation determines when the worker should stop.
// Use the actor's context, not a short I/O request context that expires before
// its result can be acknowledged. Retry creates no additional goroutines.
//
// Accepted means queued, not executed. The callback also checks cancellation
// before execution so an accepted but delayed receipt cannot mutate closed UI.
func Retry(ctx context.Context, dispatch func(func()) bool, fn func()) bool {
	if dispatch == nil || fn == nil || ctx.Err() != nil {
		return false
	}
	callback := func() {
		if ctx.Err() == nil {
			fn()
		}
	}
	if dispatch(callback) {
		return true
	}
	timer := time.NewTimer(retryDelay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			if ctx.Err() != nil {
				return false
			}
			if dispatch(callback) {
				return true
			}
			timer.Reset(retryDelay)
		}
	}
}
