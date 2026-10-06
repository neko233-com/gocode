package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/neko233-com/gocode/internal/filewatch"
	"os"
	"path/filepath"
	"strings"
	"time"

	ui "github.com/neko233-com/godesktop"
)

const watchFixture = "package main\n// TODO original\nfunc main() {}\n"
const watchClean = "package main\r\n// disk clean\r\nfunc externalCleanConfirmed() {}\r\n"
const watchConflict = "package main\n// TODO external conflict\nfunc externalConflictReplacementCompleted() {}\n"

var errWatchPixelsPending = errors.New("file watch pixels are pending")

type filewatchAcceptance struct {
	phase                      int
	frame                      uint64
	tick, verified, requireLSP bool
	failure, local             string
	version                    int
	activated, logged          bool
	loggedPhase                int
}

func (a *filewatchAcceptance) write(cx *ui.Context, path, value string) {
	go func() {
		temp := filepath.Join(filepath.Dir(path), ".external-editor-save")
		err := os.WriteFile(temp, []byte(value), 0600)
		if err == nil {
			err = filewatch.Replace(context.Background(), temp, path, nil)
		}
		cx.Dispatch(func() {
			if err != nil {
				a.failure = err.Error()
			}
		})
	}()
}
func (a *filewatchAcceptance) step(cx *ui.Context, m *model) {
	if !a.logged || a.loggedPhase != a.phase {
		fmt.Fprintf(os.Stderr, "gocode file watch acceptance phase=%d status=%q message=%q\n", a.phase, m.lspStatus, m.message)
		a.logged, a.loggedPhase = true, a.phase
	}
	d := m.current()
	if d == nil || d.buffer == nil {
		a.failure = "no editable fixture"
	}
	if a.failure != "" {
		cx.Quit()
		return
	}
	_, warnings := m.diagnosticCounts()
	ready := !a.requireLSP || strings.HasSuffix(m.lspStatus, " ready")
	capture := func(stage string, skip int, control string) bool {
		row, ok := cx.ElementBounds("code-line-2")
		if !ok {
			return false
		}
		prefix, _ := ui.MeasureText("func ", 14, codeFont())
		row.X += 68 + prefix + float32(skip)*ui.TextAdvance("M", 14, codeFont())
		row.Width = 180
		row.Height = 20
		var target ui.Bounds
		if control != "" {
			target, ok = cx.ElementBounds(control)
			if !ok {
				return false
			}
		}
		err := captureFileWatchAcceptance(m.workspace, stage, row, target)
		if errors.Is(err, errWatchPixelsPending) {
			return false
		}
		if err != nil {
			a.failure = err.Error()
			return false
		}
		return true
	}
	click := func(key string, fallback func()) bool {
		bounds, ok := cx.ElementBounds(key)
		if !ok {
			return false
		}
		activateFileWatchControl(cx, m.workspace, bounds, fallback, func(err error) {
			if err != nil {
				a.failure = err.Error()
			}
		})
		return true
	}
	switch a.phase {
	case 0:
		if !a.activated && cx.RenderedFrames() >= 2 && m.execute != nil {
			a.activated = true
			m.execute("gocode.hello")
		}
		if cx.RenderedFrames() < 2 || warnings != 1 || !ready {
			break
		}
		a.version = d.buffer.Version()
		a.phase = 1
		a.write(cx, d.path, watchClean)
	case 1:
		if d.buffer.Text() != watchClean || d.dirty() || d.buffer.Version() != a.version+1 || warnings != 0 {
			break
		}
		if a.frame == 0 {
			a.frame = cx.RenderedFrames()
			break
		}
		if cx.RenderedFrames() < a.frame+3 {
			break
		}
		if !capture("clean-reload", 4, "") {
			break
		}
		m.editing = true
		m.moveCursor(d, 0, 0, false)
		m.input(cx, ui.InputEvent{Kind: ui.Character, Key: 'X'})
		a.local, a.version = d.buffer.Text(), d.buffer.Version()
		a.phase = 2
		a.write(cx, d.path, watchConflict)
	case 2:
		if d.diskConflict == nil {
			break
		}
		if d.buffer.Text() != a.local || !d.dirty() || d.buffer.Version() != a.version {
			a.failure = "external change overwrote dirty document"
			break
		}
		if !capture("dirty-conflict", 4, "disk-reload") {
			break
		}
		if click("disk-reload", func() { m.beginReload(d) }) {
			a.phase = 3
			a.frame = cx.RenderedFrames()
		}
	case 3:
		if m.reloadPrompt == nil || cx.RenderedFrames() < a.frame+3 {
			break
		}
		if !capture("reload-dialog", 4, "reload-confirm") {
			break
		}
		m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 27})
		if m.reloadPrompt != nil || d.buffer.Text() != a.local || !d.dirty() {
			a.failure = "reload cancellation discarded edits"
			break
		}
		a.phase = 4
		a.frame = cx.RenderedFrames()
	case 4:
		if cx.RenderedFrames() < a.frame+3 {
			break
		}
		if click("disk-reload", func() { m.beginReload(d) }) {
			a.phase = 5
			a.frame = cx.RenderedFrames()
		}
	case 5:
		if m.reloadPrompt == nil || cx.RenderedFrames() < a.frame+3 {
			break
		}
		if click("reload-confirm", func() { m.requestDiskReload(d) }) {
			a.phase = 6
		}
	case 6:
		if m.reloadPrompt != nil || d.buffer.Text() != watchConflict || d.dirty() || warnings != 1 {
			break
		}
		if d.buffer.Version() != a.version+1 || d.buffer.EOL() != "\n" {
			a.failure = "confirmed reload lost version/EOL"
			break
		}
		// Undo and redo must traverse the external saved revision, including EOL.
		m.editing = true
		m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 'Z', Modifiers: ui.ModifierControl})
		if d.buffer.Text() != a.local || !d.dirty() || d.buffer.EOL() != "\r\n" {
			a.failure = "native undo lost prior edits/EOL"
			break
		}
		m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 'Y', Modifiers: ui.ModifierControl})
		if d.buffer.Text() != watchConflict || d.dirty() {
			a.failure = "native redo lost external savepoint"
			break
		}
		a.phase = 7
		if a.requireLSP {
			m.moveCursor(d, 2, 12, false)
			m.requestLSP(d, "textDocument/hover")
		}
	case 7:
		if a.requireLSP && !strings.Contains(strings.Join(m.output, "\n"), "externalConflictReplacementCompleted()") {
			break
		}
		m.panel = "PROBLEMS"
		m.showPanel = true
		a.phase = 8
		a.frame = cx.RenderedFrames()
	case 8:
		if cx.RenderedFrames() < a.frame+3 || warnings != 1 {
			break
		}
		if !capture("confirmed-reload", len("externalCleanConfirmed"), "problem-0") {
			break
		}
		a.verified = true
		cx.Quit()
		return
	}
	if a.failure != "" {
		cx.Quit()
		return
	}
	if !a.tick {
		a.tick = true
		time.AfterFunc(30*time.Millisecond, func() { cx.Dispatch(func() { a.tick = false }) })
	}
}
func (a *filewatchAcceptance) verify(m *model) error {
	if !a.verified {
		return fmt.Errorf("native file watch acceptance phase %d: %s", a.phase, a.failure)
	}
	data, err := os.ReadFile(m.current().path)
	if err != nil || string(data) != watchConflict {
		return errors.New("file watch acceptance changed external disk contents")
	}
	return nil
}
