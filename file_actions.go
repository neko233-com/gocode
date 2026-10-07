package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/neko233-com/gocode/internal/filewatch"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

type fileActionState struct {
	chordUntil time.Time
	busy       bool
	choose     func(string, string, func(string, error))
	writeAs    func(context.Context, *document, string, func(error))
}

var errDialogCancelled = errors.New("file dialog cancelled")

func (m *model) chooseFileAction(kind string, d *document, done func(error)) {
	if m.fileActions.choose == nil {
		if done != nil {
			done(errors.New("native file dialogs unavailable"))
		}
		return
	}
	initial := m.workspace
	if d != nil {
		initial = d.path
	}
	m.fileActions.choose(kind, initial, func(path string, err error) {
		if err != nil {
			if !errors.Is(err, errDialogCancelled) {
				m.message = err.Error()
			}
			if done != nil {
				done(err)
			}
			return
		}
		switch kind {
		case "open":
			m.open(path)
		case "folder":
			m.requestRelaunch(path)
		case "vsix":
			if m.extensionsView.manage != nil {
				m.extensionsView.manage("install", path)
			}
		case "save":
			if m.fileActions.writeAs != nil {
				m.fileActions.writeAs(context.Background(), d, path, done)
			}
		}
	})
}
func (m *model) saveAsDocument(d *document, done func(error)) {
	if d == nil || d.buffer == nil {
		if done != nil {
			done(errors.New("no editable document"))
		}
		return
	}
	m.chooseFileAction("save", d, done)
}
func (m *model) requestRelaunch(workspace string) {
	if m.relaunch == nil {
		return
	}
	m.pendingWindowAction = func() { m.relaunch(workspace, true) }
	if m.requestWindowClose(m.native) {
		action := m.pendingWindowAction
		m.pendingWindowAction = nil
		action()
	}
}

// Save As writes a bounded immutable snapshot. A new target uses exclusive
// creation; an existing target uses the normal hash-checked atomic replacement.
func writeSaveAs(ctx context.Context, path string, snapshot textbuffer.Snapshot) ([32]byte, error) {
	var zero [32]byte
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Size() > editableFileLimit {
			return zero, errors.New("Save As target must be a regular file within the editable limit")
		}
		f, err := filewatch.OpenRead(path)
		if err != nil {
			return zero, err
		}
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(f, editableFileLimit+1))
		err = errors.Join(err, f.Close())
		if err != nil {
			return zero, err
		}
		if n > editableFileLimit {
			return zero, errors.New("target grew beyond save limit")
		}
		var expected [32]byte
		copy(expected[:], h.Sum(nil))
		return writeDocumentSnapshot(ctx, path, snapshot, &expected)
	} else if !os.IsNotExist(err) {
		return zero, err
	}
	bytes := 0
	for i := range snapshot.LineCount() {
		bytes += len(snapshot.Line(i))
		if i > 0 {
			bytes += len(snapshot.EOL)
		}
		if bytes > editableFileLimit {
			return zero, errors.New("snapshot exceeds save limit")
		}
	}
	text := snapshot.Text()
	hash := sha256.Sum256([]byte(text))
	f, err := os.CreateTemp(filepath.Dir(path), ".gocode-save-as-")
	if err != nil {
		return zero, err
	}
	name := f.Name()
	defer os.Remove(name)
	_, err = io.Copy(f, &cancelledTextReader{ctx: ctx, text: []byte(text)})
	err = errors.Join(err, f.Sync(), f.Close())
	if err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if err := commitNewSave(name, path); err != nil {
		return zero, err
	}
	return hash, nil
}

type cancelledTextReader struct {
	ctx  context.Context
	text []byte
}

func (r *cancelledTextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if len(r.text) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.text[:min(len(r.text), 128<<10)])
	r.text = r.text[n:]
	return n, nil
}
func (m *model) startFileActions(parent context.Context, cx *ui.Context) func() {
	ctx, cancel := context.WithCancel(parent)
	var workers sync.WaitGroup
	m.fileActions.choose = func(kind, initial string, done func(string, error)) {
		if m.fileActions.busy {
			done("", errors.New("a file operation is already in progress"))
			return
		}
		m.fileActions.busy = true
		workspace := m.workspace
		workers.Go(func() {
			path, err := nativeChooseFile(ctx, workspace, kind, initial)
			cx.Dispatch(func() {
				m.fileActions.busy = false
				if ctx.Err() == nil {
					done(path, err)
				}
			})
		})
	}
	m.fileActions.writeAs = func(request context.Context, d *document, path string, done func(error)) {
		finish := func(err error) {
			if err != nil {
				m.message = err.Error()
			}
			if done != nil {
				done(err)
			}
		}
		if m.fileActions.busy || m.saveBusy || d == nil || d.buffer == nil || !m.ownsDocument(d) {
			finish(errors.New("document is unavailable or saving"))
			return
		}
		for _, other := range m.docs {
			if other != d && pathKey(other.path) == pathKey(path) {
				finish(errors.New("target is already open; close it before Save As"))
				return
			}
		}
		if !d.untitled && pathKey(d.path) == pathKey(path) {
			m.requestSave(request, []*document{d}, finish)
			return
		}
		snapshot, oldPath := d.buffer.Snapshot(), d.path
		m.fileActions.busy = true
		workers.Go(func() {
			c, stop := context.WithTimeout(ctx, 30*time.Second)
			defer stop()
			stopRequest := context.AfterFunc(request, stop)
			defer stopRequest()
			hash, err := writeSaveAs(c, path, snapshot)
			cx.Dispatch(func() {
				m.fileActions.busy = false
				if ctx.Err() != nil {
					return
				}
				if err == nil && m.ownsDocument(d) && d.path == oldPath {
					m.documentEvent("close", d, textbuffer.ChangeEvent{})
					for alias, doc := range m.pathAliases {
						if doc == d {
							delete(m.pathAliases, alias)
						}
					}
					d.path, d.untitled, d.diskHash, d.diskKnown = path, false, hash, true
					d.tabWidth = 0
					m.activeTabs().reveal = true
					d.serviceVersion = 0
					d.watchID = 0
					d.instance = ""
					d.diskConflict = nil
					m.rememberDocument(path, d)
					if d.buffer.Version() == snapshot.Version {
						d.buffer.MarkSaved()
						d.saveID++
					} else {
						err = errSaveChanged
					}
					m.documentEvent("open", d, textbuffer.ChangeEvent{})
					m.documentEvent("save", d, textbuffer.ChangeEvent{})
					if m.current() == d {
						m.documentEvent("focus", d, textbuffer.ChangeEvent{})
					}
					m.message = "Saved " + filepath.Base(path)
				} else if err == nil {
					err = fmt.Errorf("saved copy; original editor changed before acknowledgement")
				}
				finish(err)
			})
		})
	}
	// Save/Save All/close confirmation route every untitled document through
	// Save As before the shared save queue. Cancel preserves all unsaved buffers.
	queuedSave := m.requestSave
	var save func(context.Context, []*document, func(error))
	save = func(request context.Context, documents []*document, done func(error)) {
		for _, d := range documents {
			if d != nil && d.untitled {
				m.saveAsDocument(d, func(err error) {
					if err != nil {
						if done != nil {
							done(err)
						}
						return
					}
					save(request, documents, done)
				})
				return
			}
		}
		queuedSave(request, documents, done)
	}
	m.requestSave = save
	return func() {
		cancel()
		finished := make(chan struct{})
		go func() { workers.Wait(); close(finished) }()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
		}
	}
}
