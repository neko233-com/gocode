package main

import (
	"strings"
	"time"

	ui "github.com/neko233-com/godesktop"
)

type copilotAcceptance struct {
	phase    int
	frame    uint64
	verified bool
	failure  string
	tick     bool
}

func (a *copilotAcceptance) step(cx *ui.Context, m *model) {
	if a.phase == 0 && m.requestInline != nil && m.askChat != nil {
		d := m.current()
		m.editing = true
		m.moveCursor(d, 4, 4, false)
		m.requestInline(d)
		a.phase = 1
	}
	if a.phase == 1 && m.suggestion != nil {
		if ghostText(m.current(), m.suggestion) == "" {
			a.failure = "official suggestion has no drawable ghost text"
			cx.Quit()
			return
		}
		a.frame = cx.RenderedFrames()
		a.phase = 2
	}
	if a.phase == 2 && cx.RenderedFrames() > a.frame {
		m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 9})
		if !strings.Contains(m.current().buffer.Text(), "return") || m.current().buffer.Version() != 2 {
			a.failure = "Tab did not insert the real suggestion"
			cx.Quit()
			return
		}
		if m.cancelInline != nil {
			m.cancelInline()
		}
		m.requestInline = nil
		a.phase = 6
	}
	if a.phase == 6 && m.acceptedInline > 0 {
		m.panel = "COPILOT"
		m.showPanel = true
		m.chatFocused = true
		m.editing = false
		m.askChat("Reply with exactly GOCODE_COPILOT_OK. Do not use tools or inspect any files.")
		a.phase = 3
	}
	if a.phase == 3 && !m.chatBusy {
		if m.chatAnswer != "GOCODE_COPILOT_OK" {
			a.failure = "official chat failed: " + m.message
			cx.Quit()
			return
		}
		m.askChat("Explain addition in 3000 words. Do not use any tools or inspect files.")
		a.phase = 4
		time.AfterFunc(300*time.Millisecond, func() {
			cx.Dispatch(func() {
				if !m.chatBusy {
					a.failure = "cancellation fixture completed before cancel"
					cx.Quit()
					return
				}
				m.cancelChat()
			})
		})
	}
	if a.phase == 4 && !m.chatBusy {
		if !strings.Contains(m.message, "context canceled") {
			a.failure = "chat cancellation did not report cancellation: " + m.message
			cx.Quit()
			return
		}
		m.askChat("Reply with exactly GOCODE_COPILOT_OK. Do not use tools or inspect any files.")
		a.phase = 5
	}
	if a.phase == 5 && !m.chatBusy {
		if m.chatAnswer != "GOCODE_COPILOT_OK" {
			a.failure = "chat did not recover after cancellation: " + m.message
			cx.Quit()
			return
		}
		a.frame = cx.RenderedFrames()
		a.phase = 7
	}
	if a.phase == 7 && cx.RenderedFrames() > a.frame {
		if err := captureCopilotAcceptance(m.workspace); err != nil {
			a.failure = err.Error()
			cx.Quit()
			return
		}
		a.verified = true
		cx.Quit()
		return
	}
	// This explicit acceptance mode waits for actual submissions without polling
	// the production editor, which otherwise sleeps while idle.
	if !a.tick {
		a.tick = true
		time.AfterFunc(50*time.Millisecond, func() { cx.Dispatch(func() { a.tick = false }) })
	}
}
