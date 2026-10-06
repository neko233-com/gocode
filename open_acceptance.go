package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
	"github.com/neko233-com/godesktop/extensions"
)

const openReadmeFixture = "// ASYNC_OPEN_SUCCESS\n# Native extension awaited file\n"

var errOpenPixelsPending = errors.New("async-open pixels are pending")

// The latency barrier delays a worker, then performs the actual regular-file
// reader/scanner. This tests UI behavior deterministically without fabricating
// a successful disk body or substituting a fake extension host.
type openAcceptance struct {
	readGate, scanGate         chan struct{}
	readEntered                chan struct{}
	phase                      int
	frame                      uint64
	acting, vsixDone, verified bool
	vsixErr                    error
	failure                    string
	original                   *document
	local                      string
}

func newOpenAcceptance() *openAcceptance {
	return &openAcceptance{readGate: make(chan struct{}), scanGate: make(chan struct{}), readEntered: make(chan struct{}, 2)}
}
func (a *openAcceptance) read(ctx context.Context, path string) (*document, error) {
	if filepath.Base(path) == "README.md" {
		select {
		case a.readEntered <- struct{}{}:
		default:
		}
		select {
		case <-a.readGate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return loadDocument(ctx, path)
}
func (a *openAcceptance) scan(ctx context.Context, path string) workspaceScan {
	select {
	case <-a.scanGate:
	case <-ctx.Done():
		return workspaceScan{err: ctx.Err()}
	}
	return scanWorkspace(ctx, path)
}
func (a *openAcceptance) execute(cx *ui.Context, ctx context.Context, host *extensions.Host, initialized <-chan struct{}, await func(context.Context) error) {
	a.vsixDone, a.vsixErr = false, nil
	go func() {
		select {
		case <-initialized:
		case <-ctx.Done():
			return
		}
		err := await(ctx)
		if err == nil {
			err = host.Call(ctx, "execute", map[string]string{"command": "gocode.readme"}, nil)
		}
		cx.Dispatch(func() { a.vsixDone, a.vsixErr = true, err })
	}()
}
func (a *openAcceptance) control(cx *ui.Context, m *model, key string, action func(), done func()) bool {
	bounds, ok := cx.ElementBounds(key)
	if !ok {
		return false
	}
	a.acting = true
	activateFileWatchControl(cx, m.workspace, bounds, action, func(err error) {
		a.acting = false
		if err != nil {
			a.failure = err.Error()
		}
		done()
	})
	return true
}
func (a *openAcceptance) step(cx *ui.Context, m *model, ctx context.Context, host *extensions.Host, initialized <-chan struct{}) {
	defer cx.Invalidate()
	if a.failure != "" {
		cx.Quit()
		return
	}
	if a.acting {
		return
	}
	if a.phase == 0 {
		if host == nil {
			a.failure = "real extension host unavailable"
			return
		}
		if cx.RenderedFrames() < 3 || m.current() == nil {
			cx.Invalidate()
			return
		}
		a.original = m.current()
		m.open("README.md")
		a.frame = cx.RenderedFrames()
		a.phase = 1
		return
	}
	switch a.phase {
	case 1:
		select {
		case <-a.readEntered:
		default:
			return
		}
		if !m.workspaceBusy || !m.openBusy || m.current() != a.original {
			a.failure = "worker barrier did not retain the live editor"
			return
		}
		a.acting = true
		inputDuringOpen(cx, m.workspace, func() {
			for _, r := range "// responsive " {
				m.input(cx, ui.InputEvent{Kind: ui.Character, Key: int(r)})
			}
		}, func(err error) {
			a.acting = false
			if err != nil {
				a.failure = err.Error()
				return
			}
			if !a.original.dirty() || !strings.HasPrefix(a.original.buffer.Text(), "// responsive ") {
				a.failure = "native typing was blocked/lost during disk read"
				return
			}
			a.local = a.original.buffer.Text()
			a.frame = cx.RenderedFrames()
			a.phase = 2
		})
	case 2:
		if cx.RenderedFrames() < a.frame+3 {
			cx.Invalidate()
			return
		}
		row, ok := cx.ElementBounds("code-line-0")
		if !ok {
			return
		}
		err := captureOpenAcceptance(m.workspace, "pending-read", row)
		if errors.Is(err, errOpenPixelsPending) {
			cx.Invalidate()
			return
		}
		if err != nil {
			a.failure = err.Error()
			return
		}
		a.control(cx, m, "cancel-open", m.cancelPendingOpens, func() { a.phase = 3 })
	case 3:
		if m.openBusy {
			return
		}
		if len(m.docs) != 1 || m.current() != a.original || a.original.buffer.Text() != a.local {
			a.failure = "cancel changed the live buffer/opened discarded result"
			return
		}
		a.execute(cx, ctx, host, initialized, m.awaitExtensions)
		a.phase = 4
	case 4:
		select {
		case <-a.readEntered:
		default:
			return
		}
		if a.vsixDone || !m.openBusy {
			a.failure = "VSIX showTextDocument acknowledged before the disk result"
			return
		}
		a.control(cx, m, "tab-0", func() { m.active = 0; m.documentEvent("focus", a.original, textbuffer.ChangeEvent{}) }, func() { close(a.readGate); a.phase = 5 })
	case 5:
		if !a.vsixDone || m.openBusy {
			return
		}
		if a.vsixErr == nil || m.current() != a.original || a.original.buffer.Text() != a.local || len(m.docs) != 2 {
			a.failure = "stale VSIX open stole newer focus or silently succeeded"
			return
		}
		if d := m.findDocument("README.md"); d == nil || d.buffer.Text() != openReadmeFixture {
			a.failure = "actual worker file body differs"
			return
		}
		a.execute(cx, ctx, host, initialized, m.awaitExtensions)
		a.phase = 6
	case 6:
		if !a.vsixDone {
			return
		}
		if a.vsixErr != nil {
			a.failure = a.vsixErr.Error()
			return
		}
		if m.current() == a.original || m.current().buffer.Text() != openReadmeFixture {
			a.failure = "awaited VSIX did not select the actual document"
			return
		}
		close(a.scanGate)
		a.frame = cx.RenderedFrames()
		a.phase = 7
	case 7:
		if m.workspaceBusy || cx.RenderedFrames() < a.frame+3 {
			cx.Invalidate()
			return
		}
		if len(m.files) != 2 || m.current().buffer.Text() != openReadmeFixture || !a.original.dirty() || a.original.buffer.Text() != a.local {
			a.failure = "late Explorer scan changed focus/buffer"
			return
		}
		row, ok := cx.ElementBounds("code-line-1")
		if !ok {
			return
		}
		err := captureOpenAcceptance(m.workspace, "awaited-vsix", row)
		if errors.Is(err, errOpenPixelsPending) {
			cx.Invalidate()
			return
		}
		if err != nil {
			a.failure = err.Error()
			return
		}
		a.verified = true
		cx.Quit()
	}
}
func (a *openAcceptance) verify(m *model) error {
	if !a.verified {
		return fmt.Errorf("async-open native acceptance: %s", a.failure)
	}
	data, err := os.ReadFile(a.original.path)
	if err != nil || string(data) != "package main\nfunc main() {}\n" {
		return errors.New("native async-open acceptance wrote the original source")
	}
	data, err = os.ReadFile(filepath.Join(m.workspace, "README.md"))
	if err != nil || string(data) != openReadmeFixture {
		return errors.New("native async-open acceptance wrote the README")
	}
	return nil
}
