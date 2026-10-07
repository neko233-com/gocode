package main

import (
	"context"
	"time"

	"github.com/neko233-com/gocode/internal/uidispatch"
)

// Settings selection and shutdown belong to the UI thread. The single writer
// drains the latest immutable value without waiting for error UI admission; one
// separate publisher retains the latest failure under temporary queue pressure.
func startSettingsWriter[T any](parent context.Context, dispatch func(func()) bool, write func(T) error, report func(T, error)) (func(T), func()) {
	writerCtx, stopWriter := context.WithCancel(parent)
	receiptCtx, stopReceipts := context.WithCancel(parent)
	type request struct {
		value    T
		delivery context.Context
	}
	type failure struct {
		request
		err error
	}
	requests := make(chan request, 1)
	failures := make(chan failure, 1)
	writerDone, publisherDone := make(chan struct{}), make(chan struct{})
	var supersede context.CancelFunc
	stopped := false
	persist := func(value T) {
		if stopped || writerCtx.Err() != nil {
			return
		}
		if supersede != nil {
			supersede()
		}
		var delivery context.Context
		delivery, supersede = context.WithCancel(receiptCtx)
		replacePendingSetting(requests, request{value, delivery})
	}
	go func() {
		defer close(writerDone)
		for {
			select {
			case <-writerCtx.Done():
				return
			case job, ok := <-requests:
				if !ok {
					return
				}
				if err := write(job.value); err != nil && job.delivery.Err() == nil {
					replacePendingSetting(failures, failure{job, err})
				}
			}
		}
	}()
	go func() {
		defer close(publisherDone)
		for {
			select {
			case <-receiptCtx.Done():
				return
			case result := <-failures:
				// A new selection cancels rejected or already admitted failures.
				// Keep this generation alive after admission until supersession or
				// shutdown; Retry's callback guard protects delayed execution.
				uidispatch.Retry(result.delivery, dispatch, func() { report(result.value, result.err) })
			}
		}
	}()
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		stopReceipts() // Delayed UI callbacks are obsolete as soon as stop begins.
		close(requests)
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		defer stopWriter()
		for _, done := range []<-chan struct{}{writerDone, publisherDone} {
			select {
			case <-done:
			case <-timer.C:
				return // A blocked OS write owns only its immutable value.
			}
		}
	}
	return persist, stop
}

func replacePendingSetting[T any](queue chan T, value T) {
	select {
	case queue <- value:
	default:
		select {
		case <-queue:
		default:
		}
		select {
		case queue <- value:
		default:
		}
	}
}
