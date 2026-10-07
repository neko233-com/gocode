package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/neko233-com/gocode/internal/uidispatch"
)

const (
	maxWorkspaceFiles       = 250
	maxWorkspaceEntries     = 32768
	maxWorkspaceDirectories = 256
	maxWorkspaceDepth       = 64
)

type workspaceScan struct {
	files   []string
	limited bool
	skipped int
	err     error
}

func (r workspaceScan) status() string {
	if r.err != nil {
		return "Explorer: " + r.err.Error()
	}
	s := fmt.Sprintf("Explorer: %d files", len(r.files))
	if r.limited {
		s += " · scan limit reached"
	}
	if r.skipped > 0 {
		s += fmt.Sprintf(" · %d unreadable directories", r.skipped)
	}
	return s
}

func preferredFile(files []string) string {
	for _, path := range files {
		if filepath.Base(path) == "main.go" {
			return path
		}
	}
	for _, path := range files {
		if strings.HasSuffix(path, ".go") {
			return path
		}
	}
	if len(files) > 0 {
		return files[0]
	}
	return ""
}

// ReadDir with a positive count bounds allocation even when a directory has
// millions of entries. The queue, traversal depth, entries and returned paths
// have separate limits; symlink directories are never recursively followed.
func scanWorkspace(ctx context.Context, root string) (result workspaceScan) {
	type directory struct {
		relative string
		depth    int
	}
	queue := []directory{{}}
	entries := 0
	defer func() { sort.Strings(result.files) }()
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			result.err = err
			return
		}
		dir := queue[0]
		queue[0] = directory{}
		queue = queue[1:]
		f, err := os.Open(filepath.Join(root, dir.relative))
		if err != nil {
			if dir.relative == "" {
				result.err = err
				return
			}
			result.skipped++
			continue
		}
		stop := false
		for !stop {
			if err := ctx.Err(); err != nil {
				result.err = err
				f.Close()
				return
			}
			batch, readErr := f.ReadDir(128)
			for _, item := range batch {
				entries++
				if entries > maxWorkspaceEntries {
					result.limited = true
					f.Close()
					return
				}
				if item.Type()&os.ModeSymlink != 0 {
					continue
				}
				rel := filepath.Join(dir.relative, item.Name())
				if item.IsDir() {
					switch item.Name() {
					case ".git", ".cache", "node_modules", "bin", "vendor":
						continue
					}
					if len(queue) >= maxWorkspaceDirectories || dir.depth >= maxWorkspaceDepth {
						result.limited = true
						continue
					}
					queue = append(queue, directory{rel, dir.depth + 1})
					continue
				}
				if item.Type()&os.ModeType != 0 {
					continue
				}
				result.files = append(result.files, filepath.ToSlash(rel))
				if len(result.files) >= maxWorkspaceFiles {
					result.limited = true
					f.Close()
					return
				}
			}
			if readErr != nil {
				stop = true
				if !errors.Is(readErr, io.EOF) {
					if dir.relative == "" {
						result.err = readErr
					} else {
						result.skipped++
					}
				}
			}
		}
		f.Close()
	}
	return
}

// A late startup scan populates Explorer but may not steal newer navigation.
// Explicit CLI files use the same open actor independently of directory I/O.
func (m *model) startWorkspace(parent context.Context, dispatch func(func()) bool, paths []string, gotoQuery string, scan func(context.Context, string) workspaceScan, ready func(error)) func() {
	lifetime, stopLifetime := context.WithCancel(parent)
	ctx, cancel := context.WithTimeout(lifetime, 30*time.Second)
	var stopped atomic.Bool
	m.cancelWorkspace = cancel
	m.workspaceBusy = true
	m.workspaceStatus = "Scanning workspace…"
	if scan == nil {
		scan = scanWorkspace
	}
	sequence, root := m.openSequence, m.workspace
	if len(paths) > 0 {
		remaining := len(paths)
		var failures error
		for i, path := range paths {
			m.openThen(parent, path, func(d *document, err error) {
				if i == len(paths)-1 && err == nil && d != nil && gotoQuery != "" {
					m.goTo(gotoQuery)
				}
				if err != nil && !errors.Is(err, errOpenSuperseded) {
					failures = errors.Join(failures, err)
				}
				remaining--
				if remaining == 0 {
					ready(failures)
				}
			})
		}
	}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		result := scan(ctx, root)
		uidispatch.Retry(lifetime, dispatch, func() {
			if parent.Err() != nil || stopped.Load() {
				return
			}
			m.workspaceBusy = false
			m.cancelWorkspace = nil
			if ctx.Err() != nil {
				result.err = ctx.Err()
			}
			m.files, m.workspaceStatus = result.files, result.status()
			cancel()
			if len(paths) != 0 {
				return
			}
			preferred := preferredFile(result.files)
			if result.err != nil || preferred == "" || m.openSequence != sequence {
				ready(result.err)
				return
			}
			m.openThen(parent, preferred, func(d *document, err error) {
				if err == nil && d == m.current() && gotoQuery != "" {
					m.goTo(gotoQuery)
				}
				if errors.Is(err, errOpenSuperseded) {
					err = nil
				}
				ready(err)
			})
		})
	}()
	return func() {
		stopped.Store(true)
		stopLifetime()
		cancel()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
		}
	}
}
