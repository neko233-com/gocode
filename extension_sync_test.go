package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestExtensionBarrierWaitsForInitializeAndFocusAcknowledgement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initialize := make(chan struct{})
	release := make(chan struct{})
	started := make(chan string, 2)
	q := startExtensionDocumentSync(ctx, func(ctx context.Context) error {
		select {
		case <-initialize:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}, func(ctx context.Context, params map[string]any) error {
		kind := params["kind"].(string)
		started <- kind
		if kind == "open" {
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}, func(error) {})
	defer func() { cancel(); <-q.done }()
	for _, kind := range []string{"open", "focus"} {
		if err := q.push(extensionSyncJob{params: map[string]any{"kind": kind}}); err != nil {
			t.Fatal(err)
		}
	}
	mailbox := make(chan func(), 2)
	result := make(chan error, 1)
	go func() { result <- q.await(ctx, func(fn func()) bool { mailbox <- fn; return true }) }()
	saveAck(t, mailbox)()
	select {
	case <-result:
		t.Fatal("barrier ran before initialization")
	default:
	}
	close(initialize)
	select {
	case kind := <-started:
		if kind != "open" {
			t.Fatal("focus overtook open")
		}
	case <-time.After(time.Second):
		t.Fatal("sync did not start")
	}
	select {
	case <-result:
		t.Fatal("barrier ignored delayed document acknowledgement")
	default:
	}
	close(release)
	select {
	case kind := <-started:
		if kind != "focus" {
			t.Fatal("focus missing")
		}
	case <-time.After(time.Second):
		t.Fatal("focus never synchronized")
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("receipt missing")
	}
}

func TestExtensionBarrierBoundsCancellationAndFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	q := startExtensionDocumentSync(ctx, func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }, func(context.Context, map[string]any) error { return nil }, func(error) {})
	defer func() { cancel(); <-q.done }()
	for range maxExtensionSyncJobs {
		if err := q.push(extensionSyncJob{params: map[string]any{}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.push(extensionSyncJob{params: map[string]any{}}); !errors.Is(err, errExtensionSyncFull) {
		t.Fatal("queue not bounded", err)
	}
	request, stop := context.WithCancel(context.Background())
	mailbox := make(chan func(), 2)
	result := make(chan error, 1)
	go func() { result <- q.await(request, func(fn func()) bool { mailbox <- fn; return true }) }()
	ack := saveAck(t, mailbox)
	stop()
	ack()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatal("barrier ignored cancellation", err)
	}
	cancel()
	<-q.done
	if err := q.push(extensionSyncJob{}); !errors.Is(err, context.Canceled) {
		t.Fatal("stopped queue accepted a new job", err)
	}
	if err := q.await(context.Background(), func(func()) bool { return false }); err == nil {
		t.Fatal("closed UI accepted receipt")
	}
	failed := make(chan error, 1)
	q2 := startExtensionDocumentSync(context.Background(), func(context.Context) error { return errors.New("initialize failed") }, func(context.Context, map[string]any) error {
		t.Error("failed initialize started synchronization")
		return nil
	}, func(err error) { failed <- err })
	<-q2.done
	if err := <-failed; err == nil {
		t.Fatal("initialization failure hidden")
	}
	if err := q2.await(context.Background(), func(fn func()) bool { fn(); return true }); !errors.Is(err, errExtensionSyncStopped) {
		t.Fatal("failed synchronization acknowledged receipt", err)
	}
}
