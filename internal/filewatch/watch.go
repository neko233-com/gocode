// Package filewatch observes open editable files on a cancellable disk worker.
// Parent-directory OS watches survive atomic file replacement. Periodic metadata
// and content reconciliation compensate for overflow or unavailable OS watches.
package filewatch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/fsnotify/fsnotify"
)

const MaxFiles = 128
const MaxBytes = 8 << 20

type Entry struct {
	ID       uint64
	Path     string
	Version  int
	ReloadID uint64
	Hash     [32]byte
	Known    bool
}
type State struct {
	Entries []Entry
	Paused  bool
}
type Result struct {
	Entry Entry
	Kind  string // text, missing, unavailable
	Text  string
	Hash  [32]byte
	Err   error
}
type Watcher struct {
	updates chan State
	cancel  context.CancelFunc
	done    chan struct{}
	mu      sync.Mutex
}
type observation struct {
	info          os.FileInfo
	hash          [32]byte
	kind, failure string
	version       int
	reload        uint64
}

// Start queues at most one UI callback and one immutable file body at a time.
// apply returns false when its read became stale and should be reconciled again.
func Start(parent context.Context, dispatch func(func()) bool, apply func(Result) bool) *Watcher {
	ctx, cancel := context.WithCancel(parent)
	w := &Watcher{updates: make(chan State, 1), cancel: cancel, done: make(chan struct{})}
	go w.run(ctx, dispatch, apply)
	return w
}
func (w *Watcher) Update(state State) error {
	if len(state.Entries) > MaxFiles {
		return errors.New("open-file watch limit is 128 documents")
	}
	state.Entries = append([]Entry(nil), state.Entries...)
	for _, entry := range state.Entries {
		if !filepath.IsAbs(entry.Path) || len(entry.Path) > 32768 {
			return errors.New("watch path must be bounded and absolute")
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	select {
	case <-w.done:
		return context.Canceled
	default:
	}
	select {
	case w.updates <- state:
	default:
		select {
		case <-w.updates:
		default:
		}
		select {
		case w.updates <- state:
		default:
		}
	}
	return nil
}
func (w *Watcher) Close() {
	w.cancel()
	// An OS read may block despite context cancellation; it owns no live model.
	select {
	case <-w.done:
	case <-time.After(3 * time.Second):
	}
}
func key(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}
func sameInfo(a, b os.FileInfo) bool {
	return a != nil && b != nil && os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime()) && a.Mode() == b.Mode()
}

var errUnstable = errors.New("file changed during disk read")

// Read validates both the open descriptor and the final pathname identity; a
// replacement/read race must never publish the stale descriptor's bytes.
func Read(ctx context.Context, entry Entry) (Result, os.FileInfo) {
	result := Result{Entry: entry}
	info, err := os.Lstat(entry.Path)
	if errors.Is(err, os.ErrNotExist) {
		result.Kind = "missing"
		return result, nil
	}
	if err == nil && !info.Mode().IsRegular() {
		err = errors.New("disk path is no longer a regular text file")
	}
	if err == nil && info.Size() > MaxBytes {
		err = errors.New("disk file exceeds the 8 MiB editable limit; buffer was preserved")
	}
	if err != nil {
		result.Kind, result.Err = "unavailable", err
		return result, info
	}
	f, err := OpenRead(entry.Path)
	if err != nil {
		result.Kind, result.Err = "unavailable", err
		return result, info
	}
	defer f.Close()
	data := make([]byte, 0, int(info.Size()))
	block := make([]byte, 128<<10)
	for {
		if err = ctx.Err(); err != nil {
			break
		}
		var n int
		n, err = f.Read(block)
		if len(data)+n > MaxBytes {
			err = errors.New("disk file grew beyond 8 MiB while reading")
			break
		}
		data = append(data, block[:n]...)
		if err != nil {
			break
		}
	}
	if errors.Is(err, io.EOF) {
		err = nil
	}
	opened, openedErr := f.Stat()
	final, finalErr := os.Lstat(entry.Path)
	if err == nil {
		err = errors.Join(openedErr, finalErr)
	}
	if err == nil && (!sameInfo(info, opened) || !sameInfo(opened, final)) {
		err = errUnstable
	}
	if err == nil && (!utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0) {
		err = errors.New("Binary files are not supported: expected valid UTF-8 text without NUL")
	}
	if err != nil {
		result.Kind, result.Err = "unavailable", err
		return result, info
	}
	result.Kind, result.Text, result.Hash = "text", string(data), sha256.Sum256(data)
	return result, final
}

func (w *Watcher) run(ctx context.Context, dispatch func(func()) bool, apply func(Result) bool) {
	defer close(w.done)
	native, _ := fsnotify.NewWatcher()
	var events <-chan fsnotify.Event
	var failures <-chan error
	if native != nil {
		events, failures = native.Events, native.Errors
		defer native.Close()
	}
	dirs := map[string]bool{}
	entries := map[string]Entry{}
	observations := map[string]observation{}
	pending := map[string]bool{}
	paused := false
	debounce := time.NewTicker(100 * time.Millisecond)
	defer debounce.Stop()
	metadata := time.NewTicker(2 * time.Second)
	defer metadata.Stop()
	contents := time.NewTicker(30 * time.Second)
	defer contents.Stop()
	markAll := func(force bool) {
		for path := range entries {
			pending[path] = pending[path] || force
		}
	}
	update := func(state State) {
		wasPaused := paused
		paused = state.Paused
		next := map[string]Entry{}
		nextDirs := map[string]bool{}
		for _, entry := range state.Entries {
			path := key(entry.Path)
			next[path] = entry
			nextDirs[filepath.Dir(entry.Path)] = true
			old, exists := entries[path]
			if !exists || old.ID != entry.ID || old.ReloadID != entry.ReloadID || old.Hash != entry.Hash || wasPaused && !paused {
				pending[path] = true
				delete(observations, path)
			}
		}
		for path := range entries {
			if _, ok := next[path]; !ok {
				delete(observations, path)
				delete(pending, path)
			}
		}
		entries = next
		if native != nil {
			for dir := range dirs {
				if !nextDirs[dir] {
					_ = native.Remove(dir)
					delete(dirs, dir)
				}
			}
			// Failed registrations are retried at each reconciliation/update.
			for dir := range nextDirs {
				if !dirs[dir] && native.Add(dir) == nil {
					dirs[dir] = true
				}
			}
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case state := <-w.updates:
			update(state)
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			path := key(event.Name)
			if _, ok := entries[path]; ok {
				pending[path] = true
			}
			if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				for dir := range dirs {
					if key(dir) == path {
						delete(dirs, dir)
						markAll(true)
					}
				}
			}
		case _, ok := <-failures:
			if !ok {
				failures = nil
				continue
			}
			markAll(true)
		case <-metadata.C:
			markAll(false)
			update(State{Entries: values(entries), Paused: paused})
		case <-contents.C:
			markAll(true)
		case <-debounce.C:
			if paused {
				continue
			}
			// Only one read/delivery per tick: memory and queued UI work stay bounded,
			// including a storm of native file notifications while the UI is busy.
			for path, forced := range pending {
				delete(pending, path)
				entry, exists := entries[path]
				if !exists {
					continue
				}
				old := observations[path]
				if !forced {
					info, err := os.Lstat(entry.Path)
					if err == nil && sameInfo(old.info, info) {
						continue
					}
				}
				result, info := Read(ctx, entry)
				if ctx.Err() != nil {
					return
				}
				if errors.Is(result.Err, errUnstable) {
					pending[path] = true
					break
				}
				failure := ""
				if result.Err != nil {
					failure = result.Err.Error()
				}
				observations[path] = observation{info, result.Hash, result.Kind, failure, entry.Version, entry.ReloadID}
				unchanged := old.kind == result.Kind && old.hash == result.Hash && old.failure == failure && old.version == entry.Version && old.reload == entry.ReloadID
				baseline := result.Kind == "text" && entry.Known && result.Hash == entry.Hash && entry.ReloadID == 0 && old.kind == ""
				if unchanged || baseline {
					break
				}
				ack := make(chan bool, 1)
				if !dispatch(func() {
					if ctx.Err() != nil {
						ack <- true
						return
					}
					ack <- apply(result)
				}) {
					return
				}
				select {
				case <-ctx.Done():
					return
				case accepted := <-ack:
					if !accepted {
						delete(observations, path)
						pending[path] = true
					}
				}
				break
			}
		}
	}
}
func values(entries map[string]Entry) []Entry {
	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}
	return result
}
