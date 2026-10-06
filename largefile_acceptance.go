package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	ui "github.com/neko233-com/godesktop"
)

type largefileAcceptance struct {
	phase          int
	frame          uint64
	tick, verified bool
	failure        string
}

func (a *largefileAcceptance) step(cx *ui.Context, m *model) {
	d := m.current()
	if d == nil || d.large == nil {
		a.failure = "file-backed document was not opened"
		cx.Quit()
		return
	}
	l := d.large
	if l.stats.Err != nil || l.err != nil {
		a.failure = fmt.Sprint(l.stats.Err, l.err)
		cx.Quit()
		return
	}
	if a.phase == 0 && l.stats.Complete && !l.loading && len(l.page) > 0 && cx.RenderedFrames() >= 2 {
		if d.buffer != nil || !l.page[0].Truncated || l.stats.Size < 16<<20 {
			a.failure = "large line was loaded into an editable buffer"
			cx.Quit()
			return
		}
		m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 'G', Modifiers: ui.ModifierControl})
		for _, r := range fmt.Sprintf(":%d", l.stats.Size-100) {
			m.input(cx, ui.InputEvent{Kind: ui.Character, Key: int(r)})
		}
		m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 13})
		a.phase = 1
	}
	if a.phase == 1 && l.byteMode && !l.loading && strings.Contains(l.window.Text, "NATIVE_LARGEFILE_TAIL") {
		if l.window.Line != 0 || l.window.ColumnBytes < 16<<20-100 {
			a.failure = "incorrect long-line byte coordinates"
			cx.Quit()
			return
		}
		a.frame = cx.RenderedFrames()
		a.phase = 2
	}
	if a.phase == 2 && cx.RenderedFrames() > a.frame {
		info, err := os.Stat(d.path)
		if err != nil || info.Size() != l.stats.Size {
			a.failure = "source changed during browsing"
			cx.Quit()
			return
		}
		if err := captureLargefileAcceptance(m.workspace); err != nil {
			a.failure = err.Error()
			cx.Quit()
			return
		}
		a.verified = true
		cx.Quit()
		return
	}
	if !a.tick {
		a.tick = true
		time.AfterFunc(50*time.Millisecond, func() { cx.Dispatch(func() { a.tick = false }) })
	}
}
