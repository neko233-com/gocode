package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	ui "github.com/neko233-com/godesktop"
)

const closeFixture = "package main\n\nfunc main() {}\n"

type closeAcceptance struct {
	mode                      string
	phase                     int
	frame                     uint64
	errorFrame                uint64
	tick, cancelled, rejected bool
	failure                   string
	snapshot                  string
}

func (a *closeAcceptance) step(cx *ui.Context, m *model) {
	if a.failure != "" {
		cx.Quit()
		return
	}
	d := m.current()
	if d == nil || d.buffer == nil {
		a.failure = "editable fixture not opened"
		cx.Quit()
		return
	}
	if a.phase == 0 && cx.RenderedFrames() >= 2 {
		m.editing = true
		m.input(cx, ui.InputEvent{Kind: ui.Character, Key: 'X'})
		a.snapshot = d.buffer.Text()
		a.phase = 1
		if a.mode == "external" {
			path := d.path
			go func() {
				err := os.WriteFile(path, []byte(closeFixture+"// external edit\n"), 0600)
				cx.Dispatch(func() {
					if err != nil {
						a.failure = err.Error()
					} else {
						cx.RequestClose()
					}
				})
			}()
		} else {
			cx.RequestClose()
		}
	}
	if a.phase == 1 && m.closePrompt {
		if !d.dirty() || d.buffer.Text() != a.snapshot {
			a.failure = "native close guard lost the unsaved buffer"
		}
		a.phase, a.frame = 2, cx.RenderedFrames()
	}
	if a.phase == 2 && cx.RenderedFrames() > a.frame {
		if err := captureCloseAcceptance(m.workspace, a.mode); err != nil {
			a.failure = err.Error()
		}
		if a.mode == "cancel" && !a.cancelled {
			version := d.buffer.Version()
			m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 27})
			if m.closePrompt || !m.editing || d.buffer.Version() != version || d.buffer.Text() != a.snapshot {
				a.failure = "cancel did not restore the unchanged editing session"
			}
			m.input(cx, ui.InputEvent{Kind: ui.Character, Key: 'Y'})
			a.snapshot = d.buffer.Text()
			a.cancelled = true
			a.phase = 1
			cx.RequestClose()
		} else if a.mode == "discard" || a.rejected {
			a.phase = 4
			m.discardForClose()
		} else {
			if a.mode == "external" {
				a.phase = 3
			} else {
				a.phase = 4
			}
			m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 13})
		}
	}
	if a.phase == 3 && !m.closeBusy {
		if a.errorFrame == 0 {
			a.errorFrame = cx.RenderedFrames()
		}
		if cx.RenderedFrames() <= a.errorFrame {
			cx.Invalidate()
			return
		}
		if !m.closePrompt || !strings.Contains(m.closeError, "changed on disk") || !d.dirty() || d.buffer.Text() != a.snapshot {
			a.failure = "external conflict did not preserve the buffer/window"
		} else {
			if err := captureCloseAcceptance(m.workspace, "external-conflict"); err != nil {
				a.failure = err.Error()
			}
			m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 27})
			a.rejected, a.phase = true, 1
			cx.RequestClose()
		}
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

// Post-window disk checks run outside the native UI callback. The fixture is
// owned by this process and never reads/writes an actual user workspace.
func (a *closeAcceptance) verify(m *model) error {
	if a.failure != "" || a.phase != 4 || m.closePrompt || m.closeBusy {
		return fmt.Errorf("native close acceptance: phase %d: %s", a.phase, a.failure)
	}
	d := m.current()
	expected := a.snapshot
	if a.mode == "discard" {
		expected = closeFixture
	} else if a.mode == "external" {
		expected = closeFixture + "// external edit\n"
	}
	data, err := os.ReadFile(d.path)
	if err != nil || string(data) != expected {
		return errors.New("native close produced an unexpected disk snapshot")
	}
	if (a.mode == "save" || a.mode == "cancel") && d.dirty() {
		return errors.New("saved close did not acknowledge the document revision")
	}
	if a.mode == "cancel" && !a.cancelled || a.mode == "external" && !a.rejected {
		return errors.New("close cancellation/conflict path not exercised")
	}
	return nil
}
