package uidispatch

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryOverloadKeepsOneQueuedReceiptAndRunsOnDispatcher(t *testing.T) {
	var attempts atomic.Int32
	mailbox := make(chan func(), 1)
	result := make(chan bool, 1)
	executed := 0
	go func() {
		result <- Retry(context.Background(), func(fn func()) bool {
			if attempts.Add(1) <= 5 {
				return false
			}
			mailbox <- fn
			return true
		}, func() { executed++ })
	}()
	var callback func()
	select {
	case callback = <-mailbox:
	case <-time.After(time.Second):
		t.Fatal("temporary overload lost the pending receipt")
	}
	if executed != 0 {
		t.Fatal("worker executed the UI mutation")
	}
	select {
	case accepted := <-result:
		if !accepted || attempts.Load() != 6 || len(mailbox) != 0 {
			t.Fatal("retry queued more than one receipt", accepted, attempts.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("retry worker did not return after queue acceptance")
	}
	callback()
	if executed != 1 {
		t.Fatal("accepted receipt did not execute once")
	}
}

func TestRetryPermanentRejectionCancelsAndStaleAcceptedCallbackDoesNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var attempts atomic.Int32
	rejected := make(chan struct{}, 1)
	result := make(chan bool, 1)
	go func() {
		result <- Retry(ctx, func(func()) bool {
			attempts.Add(1)
			select {
			case rejected <- struct{}{}:
			default:
			}
			return false
		}, func() { t.Error("cancelled receipt executed") })
	}()
	select {
	case <-rejected:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("worker never attempted its first dispatch")
	}
	cancel()
	select {
	case accepted := <-result:
		if accepted {
			t.Fatal("permanently rejected callback was accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("rejection retained a worker after cancellation")
	}
	before := attempts.Load()
	time.Sleep(2 * retryDelay)
	if attempts.Load() != before {
		t.Fatal("retry worker remained live after cancellation")
	}
	ctx, cancel = context.WithCancel(context.Background())
	var callback func()
	executed := false
	if !Retry(ctx, func(fn func()) bool { callback = fn; return true }, func() { executed = true }) {
		t.Fatal("live queue rejected a callback")
	}
	cancel()
	callback()
	if executed {
		t.Fatal("already queued callback mutated state after shutdown")
	}
}
