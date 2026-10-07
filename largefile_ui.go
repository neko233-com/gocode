package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/mattn/go-runewidth"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gocode/internal/largefile"
	ui "github.com/neko233-com/godesktop"
)

// Protocol snapshots stay below the framework's bounded 16 MiB message limit.
// Larger documents use a file-backed viewer, never a truncated editable buffer.
const editableFileLimit = 8 << 20

type largeDocument struct {
	file                                  *largefile.File
	ctx                                   context.Context
	cancel                                context.CancelFunc
	workers                               sync.WaitGroup
	closeOnce                             sync.Once
	shared                                bool
	started, loading, requested, byteMode bool
	start, byteOffset                     int64
	rows                                  int
	generation                            uint64
	requestCancel                         context.CancelFunc
	stats                                 largefile.Stats
	page                                  []largefile.Line
	window                                largefile.Window
	err                                   error
}

func openLargeDocument(path string) (*document, error) {
	ctx, cancel := context.WithCancel(context.Background())
	f, err := largefile.Open(ctx, path)
	if err != nil {
		cancel()
		return nil, err
	}
	return &document{path: path, large: &largeDocument{file: f, ctx: ctx, cancel: cancel, stats: f.Stats()}}, nil
}
func (d *document) dirty() bool { return d.buffer != nil && d.buffer.Dirty() }
func (l *largeDocument) close() {
	l.closeOnce.Do(func() {
		l.cancel()
		l.workers.Wait()
		if !l.shared {
			l.file.Close()
		}
	})
}
func (m *model) disposeLargeDocument(d *document, wait bool) {
	base := d.largeBase
	if base == nil {
		base = d.large
	}
	if base == nil {
		return
	}
	for _, g := range m.allGroups() {
		if v := g.views[d]; v != nil && v.large != nil && v.large != base {
			v.large.cancel()
			if wait {
				v.large.close()
			} else {
				go v.large.close()
			}
		}
	}
	base.cancel()
	if wait {
		base.close()
	} else {
		go base.close()
	}
}
func (m *model) closeDocuments() {
	for _, d := range m.docs {
		m.disposeLargeDocument(d, true)
	}
}
func (m *model) startLargeDocument(d *document) {
	l := d.large
	if l == nil || l.started || m.native == nil {
		return
	}
	l.started = true
	cx := m.native
	l.workers.Go(func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			stats := l.file.Stats()
			if !cx.Dispatch(func() {
				if l.ctx.Err() != nil {
					return
				}
				l.stats = stats
				if errors.Is(l.err, largefile.ErrNotIndexed) {
					l.requested = false
				}
			}) {
				return
			}
			if stats.Complete || stats.Err != nil {
				return
			}
			select {
			case <-ticker.C:
			case <-l.ctx.Done():
				return
			}
		}
	})
}

func (m *model) requestLargePage(d *document, rows int) {
	l := d.large
	if m.native == nil || l.ctx.Err() != nil {
		return
	}
	rows = min(largefile.MaxRows, max(1, rows))
	if l.requested && l.start == int64(d.scroll) && l.rows == rows {
		return
	}
	if l.requestCancel != nil {
		l.requestCancel()
	}
	l.generation++
	generation := l.generation
	l.start, l.rows, l.requested, l.loading = int64(d.scroll), rows, true, true
	start, offset, byteMode, cx := l.start, l.byteOffset, l.byteMode, m.native
	ctx, stop := context.WithTimeout(l.ctx, 15*time.Second)
	l.requestCancel = stop
	l.workers.Go(func() {
		defer stop()
		var page []largefile.Line
		var window largefile.Window
		var err error
		if byteMode {
			window, err = l.file.Bytes(ctx, offset, largefile.MaxWindowBytes)
		} else {
			page, err = l.file.Lines(ctx, start, rows)
		}
		cx.Dispatch(func() {
			if l.ctx.Err() != nil || generation != l.generation {
				return
			}
			l.loading = false
			l.page, l.window, l.err = page, window, err
		})
	})
}

func (m *model) navigateLarge(d *document, line, offset int64, bytes bool) {
	l := d.large
	l.requested = false
	l.byteMode = bytes
	if bytes {
		l.byteOffset = max(0, min(l.stats.Size, offset))
	} else {
		d.scroll = int(max(0, line))
		d.line = d.scroll
	}
}

func (m *model) goTo(query string) {
	d := m.current()
	if d == nil {
		return
	}
	query = strings.TrimSpace(query)
	bytes := strings.HasPrefix(query, ":")
	value, err := strconv.ParseInt(strings.TrimPrefix(query, ":"), 10, 64)
	if err != nil || value < 0 || (!bytes && value == 0) {
		m.message = "Enter a line number (1-based), or :byte-offset (0-based)"
		return
	}
	if d.large != nil {
		m.navigateLarge(d, value-1, value, bytes)
	} else if bytes {
		m.message = "Byte navigation is available in large-file browsing"
		return
	} else {
		m.moveCursor(d, int(min(value-1, int64(d.buffer.LineCount()-1))), 0, false)
	}
	m.navigation = false
	m.query = ""
	m.editing = true
}

func (m *model) largeInput(cx *ui.Context, d *document, e ui.InputEvent) bool {
	l := d.large
	if e.Kind == ui.PointerPressed && cx != nil {
		if b, ok := cx.ElementBounds(m.editorKey("large-scrollbar")); ok && e.X >= b.X && e.X < b.X+b.Width && e.Y >= b.Y && e.Y < b.Y+b.Height {
			m.largeScrollbar = true
			m.scrollLargePointer(d, b, e.Y)
			return true
		}
		m.editing = true
	}
	if e.Kind == ui.PointerMoved && m.largeScrollbar && cx != nil {
		if b, ok := cx.ElementBounds(m.editorKey("large-scrollbar")); ok {
			m.scrollLargePointer(d, b, e.Y)
		}
		return true
	}
	if e.Kind == ui.PointerReleased || e.Kind == ui.InputCancelled {
		m.largeScrollbar = false
		return false
	}
	if e.Kind == ui.Scroll {
		if l.byteMode {
			m.navigateLarge(d, 0, l.byteOffset-int64(e.Y)*1024, true)
		} else {
			m.navigateLarge(d, int64(max(0, min(int(l.stats.Lines)-1, d.scroll-int(e.Y)))), 0, false)
		}
		return true
	}
	if e.Kind == ui.KeyPressed {
		command := e.Modifiers&(ui.ModifierControl|ui.ModifierCommand) != 0
		if command && e.Key == 'C' && m.writeClipboard != nil {
			value := l.window.Text
			if !l.byteMode {
				var lines []string
				for _, row := range l.page {
					lines = append(lines, row.Text)
				}
				value = strings.Join(lines, "\n")
			}
			if err := m.writeClipboard(value); err != nil {
				m.message = err.Error()
			}
			return true
		}
		step := 1
		if e.Key == 33 || e.Key == 34 {
			step = max(1, l.rows-1)
		}
		switch e.Key {
		case 38, 33:
			if l.byteMode {
				m.navigateLarge(d, 0, l.byteOffset-int64(step)*1024, true)
			} else {
				m.navigateLarge(d, int64(d.scroll-step), 0, false)
			}
			return true
		case 40, 34:
			if l.byteMode {
				m.navigateLarge(d, 0, l.byteOffset+int64(step)*1024, true)
			} else {
				m.navigateLarge(d, min(l.stats.Lines-1, int64(d.scroll+step)), 0, false)
			}
			return true
		case 36:
			m.navigateLarge(d, 0, 0, l.byteMode)
			return true
		case 35:
			if l.stats.Complete && !l.byteMode {
				m.navigateLarge(d, l.stats.Lines-1, 0, false)
			} else {
				m.navigateLarge(d, 0, max(0, l.stats.Size-largefile.MaxWindowBytes), true)
			}
			return true
		case 37:
			m.navigateLarge(d, 0, l.byteOffset-4096, true)
			return true
		case 39:
			m.navigateLarge(d, 0, l.byteOffset+4096, true)
			return true
		}
	}
	if e.Kind == ui.Character || (e.Kind == ui.KeyPressed && (e.Key == 8 || e.Key == 46 || e.Key == 13 || e.Key == 9)) {
		m.message = "Large-file browsing is read-only. Ctrl/Cmd+G: line or :byte offset; Ctrl/Cmd+C: visible page."
		return true
	}
	return false
}

func (m *model) scrollLargePointer(d *document, b ui.Bounds, y float32) {
	l := d.large
	fraction := max(float32(0), min(float32(1), (y-b.Y)/max(float32(1), b.Height)))
	if l.byteMode || !l.stats.Complete {
		m.navigateLarge(d, 0, int64(float64(l.stats.Size)*float64(fraction)), true)
	} else {
		m.navigateLarge(d, int64(float64(max(int64(0), l.stats.Lines-1))*float64(fraction)), 0, false)
	}
}

func (m *model) largeCodeView(d *document, visible int) *ui.Element {
	width := float32(900)
	if m.native != nil {
		w, _ := m.native.WindowSize()
		width = max(1, w-290)
	}
	return m.largeCodeViewFor(d, visible, width, func(key string) string { return key }, func(line, offset int64, bytes bool) { m.navigateLarge(d, line, offset, bytes) }, func() { m.navigation = true; m.query = "" })
}
func (m *model) largeCodeViewFor(d *document, visible int, width float32, key func(string) string, navigate func(int64, int64, bool), goTo func()) *ui.Element {
	l := d.large
	m.startLargeDocument(d)
	m.requestLargePage(d, visible-2)
	percent := 100 * float64(l.stats.Scanned) / float64(max(int64(1), l.stats.Size))
	mode := "Lines"
	if l.byteMode {
		mode = fmt.Sprintf("Byte %d", l.window.Offset)
	}
	info := fmt.Sprintf("Read-only · %.2f GiB · %s · index %.0f%% · Ctrl/Cmd+G: line or :byte", float64(l.stats.Size)/(1<<30), mode, percent)
	rows := []*ui.Element{label(info).Height(24).PaddingXY(10, 0).Foreground(ui.RGB(muted)), ui.Row(button("Line view", key("large-line-view"), func(*ui.Context) { navigate(int64(d.scroll), 0, false) }), button("Byte view", key("large-byte-view"), func(*ui.Context) { navigate(0, l.byteOffset, true) }), button("Go to…", key("large-goto"), func(*ui.Context) { goTo() })).Height(24)}
	err := l.err
	if l.stats.Err != nil {
		err = l.stats.Err
	}
	if err != nil {
		rows = append(rows, label(err.Error()).Height(24).PaddingXY(10, 0).Foreground(ui.RGB(0xf48771)))
	}
	if l.loading {
		rows = append(rows, label("Reading page…").Height(20).PaddingXY(10, 0))
	} else if l.byteMode {
		// Wrap only a bounded byte window, so a single long line is fully navigable.
		columns := max(1, int(max(1, width-34)/max(1, ui.TextAdvance("M", 14, codeFont()))))
		lines := strings.Split(strings.ReplaceAll(l.window.Text, "\r\n", "\n"), "\n")
		for _, line := range lines {
			runes := []rune(line)
			for len(runes) > 0 && len(rows) < visible {
				n, cells := 0, 0
				for n < len(runes) {
					next := max(0, runewidth.RuneWidth(runes[n]))
					if cells+next > columns && n > 0 {
						break
					}
					cells += next
					n++
				}
				rows = append(rows, ui.Text(string(runes[:n])).FontFamily(codeFont()).FontSize(14).Height(20).PaddingXY(10, 0).Foreground(ui.RGB(foreground)))
				runes = runes[n:]
			}
			if len(rows) >= visible {
				break
			}
		}
	} else {
		for _, line := range l.page {
			text := line.Text
			if line.Truncated {
				text += " … [long line: use byte view]"
			}
			rows = append(rows, ui.Row(label(fmt.Sprintf("%d", line.Number+1)).Width(80).Foreground(ui.RGB(0x858585)), ui.Text(text).FontFamily(codeFont()).FontSize(14).Foreground(ui.RGB(foreground)).Flex(1)).Height(20))
		}
	}
	rows = append(rows, spacer())
	return ui.Row(ui.Column(rows...).Flex(1), ui.Column().Width(14).Background(ui.RGB(0x333333)).Key(key("large-scrollbar")).OnClick(func(*ui.Context) {}))
}
