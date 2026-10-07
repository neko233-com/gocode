package main

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestSaveWriterRetriesRejectedUIReceiptWithFrozenSnapshot(t *testing.T) {
	m := testModel(t)
	t.Cleanup(m.closeDocuments)
	d := m.current()
	mailbox := make(chan func(), 1)
	rejected := make(chan struct{}, 1)
	var attempts atomic.Int32
	stop := m.startSaveActor(context.Background(), func(fn func()) bool {
		if attempts.Add(1) == 1 {
			rejected <- struct{}{}
			return false
		}
		mailbox <- fn
		return true
	})
	t.Cleanup(stop)
	text(m, "first rejected receipt😀")
	first := d.buffer.Text()
	var firstResult error
	firstDone := false
	m.requestSave(context.Background(), []*document{d}, func(err error) { firstResult, firstDone = err, true })
	select {
	case <-rejected:
	case <-time.After(5 * time.Second):
		t.Fatal("real disk writer did not reach rejected UI dispatch")
	}
	if autoSaveDisk(t, d.path) != first || !m.saveBusy || !d.dirty() || firstDone {
		t.Fatal("rejected receipt did not preserve the completed disk/UI boundary")
	}
	text(m, "newer edit 世界")
	newer := d.buffer.Text()
	secondDone := false
	m.requestSave(context.Background(), []*document{d}, func(err error) {
		if err != nil {
			t.Error(err)
		}
		secondDone = true
	})
	saveAck(t, mailbox)()
	if !firstDone || firstResult == nil || !d.dirty() || d.saveID != 0 || d.buffer.Text() != newer {
		t.Fatal("retried old snapshot acknowledged newer unsaved edits", firstResult)
	}
	saveAck(t, mailbox)()
	if !secondDone || d.dirty() || d.saveID != 1 || m.saveBusy || len(m.saveJobs) != 0 || autoSaveDisk(t, d.path) != newer || attempts.Load() != 3 {
		t.Fatal("successful disk receipt was lost after temporary overload", attempts.Load())
	}
}

func TestAutoSaveTimerRetriesRejectedDispatchAndRemainsLive(t *testing.T) {
	m, diskMailbox, _ := autoSaveModel(t, "afterDelay")
	m.autoSave.config.DelayMS = 100
	d := m.current()
	text(m, "automatic after rejected tick😀")
	mailbox := make(chan func(), 1)
	rejected := make(chan struct{}, 1)
	var attempts atomic.Int32
	stop := m.startAutoSaveActor(context.Background(), func(fn func()) bool {
		if attempts.Add(1) == 1 {
			rejected <- struct{}{}
			return false
		}
		mailbox <- fn
		return true
	})
	defer stop()
	select {
	case <-rejected:
	case <-time.After(5 * time.Second):
		t.Fatal("automatic timer did not reach dispatch overload")
	}
	saveAck(t, mailbox)()
	saveAck(t, diskMailbox)()
	if d.dirty() || d.saveID != 1 || !m.autoSave.running || autoSaveDisk(t, d.path) != d.buffer.Text() {
		t.Fatal("automatic save stopped at the first rejected dispatch")
	}
	text(m, "second scheduled version世界")
	saveAck(t, mailbox)()
	saveAck(t, diskMailbox)()
	if d.dirty() || d.saveID != 2 || attempts.Load() != 3 || autoSaveDisk(t, d.path) != d.buffer.Text() {
		t.Fatal("automatic actor did not remain live after overload", attempts.Load())
	}
}

func TestSaveWriterAndTimerPermanentDispatchRejectionShutDown(t *testing.T) {
	t.Run("writer", func(t *testing.T) {
		m := testModel(t)
		t.Cleanup(m.closeDocuments)
		d := m.current()
		rejected := make(chan struct{}, 1)
		var attempts atomic.Int32
		stop := m.startSaveActor(context.Background(), func(func()) bool {
			attempts.Add(1)
			select {
			case rejected <- struct{}{}:
			default:
			}
			return false
		})
		text(m, "disk committed but UI unavailable😀")
		wanted := d.buffer.Text()
		m.requestSave(context.Background(), []*document{d}, nil)
		select {
		case <-rejected:
		case <-time.After(5 * time.Second):
			stop()
			t.Fatal("writer did not reach its permanent rejection")
		}
		m.requestSave(context.Background(), []*document{d}, nil)
		if len(m.saveJobs) != 1 || !m.saveBusy || !d.dirty() || autoSaveDisk(t, d.path) != wanted {
			stop()
			t.Fatal("writer overload did not retain one active snapshot and bounded pending work")
		}
		stop()
		before := attempts.Load()
		time.Sleep(30 * time.Millisecond)
		if attempts.Load() != before || d.saveID != 0 || !d.dirty() {
			t.Fatal("cancelled rejection mutated UI or retained a retry worker")
		}
		body, err := os.ReadFile(d.path)
		if err != nil || string(body) != wanted {
			t.Fatal("shutdown lost the already committed immutable disk snapshot", err)
		}
	})
	t.Run("timer", func(t *testing.T) {
		m, _, _ := autoSaveModel(t, "afterDelay")
		m.autoSave.config.DelayMS = 100
		d := m.current()
		original := autoSaveDisk(t, d.path)
		text(m, "timer never accepted")
		rejected := make(chan struct{}, 1)
		var attempts atomic.Int32
		stop := m.startAutoSaveActor(context.Background(), func(func()) bool {
			attempts.Add(1)
			select {
			case rejected <- struct{}{}:
			default:
			}
			return false
		})
		select {
		case <-rejected:
		case <-time.After(5 * time.Second):
			stop()
			t.Fatal("timer did not reach permanent rejection")
		}
		stop()
		before := attempts.Load()
		time.Sleep(30 * time.Millisecond)
		if attempts.Load() != before || m.autoSave.running || m.saveBusy || d.saveID != 0 || !d.dirty() || autoSaveDisk(t, d.path) != original {
			t.Fatal("timer shutdown retained work or changed source")
		}
	})
}

func TestAutoSaveRejectedTickCoalescesManyRealEditsIntoOneLatestWrite(t *testing.T) {
	m, diskMailbox, _ := autoSaveModel(t, "afterDelay")
	m.autoSave.config.DelayMS = 100
	d := m.current()
	text(m, "first timer")
	mailbox := make(chan func(), 1)
	rejected := make(chan struct{}, 1)
	var allow atomic.Bool
	var attempts, accepted atomic.Int32
	stop := m.startAutoSaveActor(context.Background(), func(fn func()) bool {
		attempts.Add(1)
		if !allow.Load() {
			select {
			case rejected <- struct{}{}:
			default:
			}
			return false
		}
		select {
		case mailbox <- fn:
			accepted.Add(1)
			return true
		default:
			return false
		}
	})
	defer stop()
	select {
	case <-rejected:
	case <-time.After(5 * time.Second):
		t.Fatal("timer did not enter the controlled full-queue interval")
	}
	for range 512 {
		m.replaceSelection(d, "世界😀")
	}
	wanted, version := d.buffer.Text(), d.buffer.Version()
	if len(m.autoSave.pending) != 1 || m.autoSave.pending[d].version != version || len(m.saveJobs) != 0 || m.saveBusy {
		t.Fatal("overload retained multiple scheduling snapshots or queued disk writes")
	}
	allow.Store(true)
	for callbacks := 0; !m.saveBusy; callbacks++ {
		if callbacks == 3 {
			t.Fatal("coalesced latest timer did not produce a real write")
		}
		saveAck(t, mailbox)()
	}
	saveAck(t, diskMailbox)()
	if d.dirty() || d.saveID != 1 || d.buffer.Version() != version || autoSaveDisk(t, d.path) != wanted || len(m.autoSave.pending) != 0 || len(m.saveJobs) != 0 || accepted.Load() > 2 || attempts.Load() < 2 {
		t.Fatal("full-queue interval did not coalesce into one latest immutable write", attempts.Load(), accepted.Load())
	}
}
