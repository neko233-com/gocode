package languageserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const maxRestarts = 4
const restartWindow = 3 * time.Minute

var ErrUnavailable = errors.New("language server is unavailable")
var ErrBacklog = errors.New("language server document queue is full; restarting")

// Event is immutable. At most 16 events wait for the consumer; diagnostic text
// in each event is limited to 256 KiB. Consumers must check Valid at delivery.
type Event struct {
	State       string
	Revision    uint64
	Session     *Session
	Err         error
	Diagnostics *PublishDiagnostics
}

type Supervisor struct {
	config    Config
	workspace string
	ctx       context.Context
	cancel    context.CancelFunc
	restart   chan struct{}
	events    chan Event
	done      chan struct{}
	active    atomic.Pointer[Session]
	revision  atomic.Uint64
	mu        sync.Mutex
	attempt   context.CancelFunc
}

// Session owns one process generation. Jobs retain at most 32 immutable source
// snapshots; requests have eight slots. A replacement never reuses these queues.
type Session struct {
	Client  *Client
	owner   *Supervisor
	ctx     context.Context
	cancel  context.CancelFunc
	jobs    chan func(context.Context, *Client) error
	slots   chan struct{}
	mu      sync.Mutex
	stopped bool
	workers sync.WaitGroup
}

func Supervise(parent context.Context, workspace string, config Config) *Supervisor {
	ctx, cancel := context.WithCancel(parent)
	s := &Supervisor{config: config, workspace: workspace, ctx: ctx, cancel: cancel,
		restart: make(chan struct{}, 1), events: make(chan Event, 16), done: make(chan struct{})}
	go s.run()
	return s
}
func (s *Supervisor) Events() <-chan Event { return s.events }
func (s *Supervisor) Current() *Session    { return s.active.Load() }
func (s *Supervisor) Valid(e Event) bool {
	if s.ctx.Err() != nil {
		return false
	}
	if e.Session != nil {
		return e.Session.Valid()
	}
	return e.Revision == s.revision.Load()
}
func (s *Session) Valid() bool { return s.ctx.Err() == nil && s.owner.Current() == s }
func (s *Supervisor) Restart() {
	if s.ctx.Err() != nil {
		return
	}
	select {
	case s.restart <- struct{}{}:
	default:
	}
	s.mu.Lock()
	if s.attempt != nil {
		s.attempt()
	}
	s.mu.Unlock()
}
func (s *Supervisor) Close() error {
	s.cancel()
	select {
	case <-s.done:
		return nil
	case <-time.After(3 * time.Second):
		return errors.New("language server shutdown exceeded three seconds")
	}
}
func (s *Session) Submit(job func(context.Context, *Client) error) error {
	if !s.Valid() {
		return ErrUnavailable
	}
	select {
	case s.jobs <- job:
		return nil
	default:
		s.cancel()
		return ErrBacklog
	}
}

// Request runs bounded work outside the document/notification loop. Its callback
// must also check Valid before publishing UI results after cancellation/restart.
func (s *Session) Request(work func(context.Context, *Client)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped || !s.Valid() {
		return false
	}
	select {
	case s.slots <- struct{}{}:
	default:
		return false
	}
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		defer func() { <-s.slots }()
		ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
		defer cancel()
		work(ctx, s.Client)
	}()
	return true
}
func (s *Session) stop() {
	s.mu.Lock()
	s.stopped = true
	s.cancel()
	s.mu.Unlock()
	_ = s.Client.Close()
	s.workers.Wait()
}
func (s *Supervisor) state(state string, err error, session *Session) bool {
	e := Event{State: state, Err: err, Session: session, Revision: s.revision.Add(1)}
	select {
	case s.events <- e:
		return true
	case <-s.ctx.Done():
		return false
	}
}
func (s *Supervisor) run() {
	defer close(s.done)
	defer close(s.events)
	var failures []time.Time
	for s.ctx.Err() == nil {
		s.state("starting", nil, nil)
		ctx, cancel := context.WithCancel(s.ctx)
		s.mu.Lock()
		s.attempt = cancel
		manual := false
		select {
		case <-s.restart:
			manual = true
			cancel()
		default:
		}
		s.mu.Unlock()
		client, err := Start(ctx, s.workspace, s.config)
		if err == nil && ctx.Err() == nil {
			session := &Session{Client: client, owner: s, ctx: ctx, cancel: cancel,
				jobs: make(chan func(context.Context, *Client) error, 32), slots: make(chan struct{}, 8)}
			s.active.Store(session)
			s.state("ready", nil, session)
			err = s.serve(session)
			s.active.Store(nil)
			select {
			case <-s.restart:
				manual = true
				err = nil
			default:
			}
			s.state("restarting", err, nil)
			session.stop()
		} else {
			cancel()
			if client != nil {
				_ = client.RPC.Close()
			}
		}
		cancel()
		s.mu.Lock()
		s.attempt = nil
		s.mu.Unlock()
		if s.ctx.Err() != nil {
			return
		}
		if manual {
			failures = nil
			continue
		}
		select {
		case <-s.restart:
			failures = nil
			continue
		default:
		}
		now := time.Now()
		for len(failures) > 0 && now.Sub(failures[0]) > restartWindow {
			failures = failures[1:]
		}
		failures = append(failures, now)
		if len(failures) > maxRestarts {
			s.state("failed", fmt.Errorf("stopped after repeated failures; use Restart Language Servers: %w", err), nil)
			select {
			case <-s.ctx.Done():
				return
			case <-s.restart:
				failures = nil
				continue
			}
		}
		s.state("restarting", err, nil)
		timer := time.NewTimer(time.Duration(1<<(len(failures)-1)) * 250 * time.Millisecond)
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return
		case <-s.restart:
			timer.Stop()
			failures = nil
		case <-timer.C:
		}
	}
}
func (s *Supervisor) serve(session *Session) error {
	for {
		select {
		case <-session.ctx.Done():
			return session.ctx.Err()
		case job := <-session.jobs:
			ctx, cancel := context.WithTimeout(session.ctx, 10*time.Second)
			err := job(ctx, session.Client)
			cancel()
			if err != nil && session.Valid() {
				select {
				case s.events <- Event{State: "error", Err: err, Session: session}:
				default:
					return errors.New("language event queue overflow")
				}
			}
		case notification, ok := <-session.Client.RPC.Notifications():
			if !ok {
				return errors.New("language server connection closed")
			}
			if session.Client.RPC.DroppedNotifications() != 0 {
				return errors.New("language transport notification queue overflow")
			}
			if notification.Method != "textDocument/publishDiagnostics" {
				continue
			}
			var p PublishDiagnostics
			if json.Unmarshal(notification.Params, &p) != nil {
				continue
			}
			if len(p.URI) > 64<<10 {
				continue
			}
			items := make([]Diagnostic, min(2000, len(p.Diagnostics)))
			budget := 256 << 10
			for i := range items {
				items[i] = p.Diagnostics[i]
				text := items[i].Message
				if len(text) > min(2048, budget) {
					text = text[:min(2048, budget)]
				}
				items[i].Message = strings.Clone(strings.ToValidUTF8(text, ""))
				budget -= len(text)
			}
			p.Diagnostics = items
			select {
			case s.events <- Event{State: "diagnostics", Session: session, Diagnostics: &p}:
			default:
				return errors.New("language diagnostic queue overflow")
			}
		}
	}
}
