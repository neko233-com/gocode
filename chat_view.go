package main

import (
	"path/filepath"
	"strings"

	ui "github.com/neko233-com/godesktop"
)

func (m *model) attachChatContext(selection bool) {
	d := m.current()
	if d == nil {
		return
	}
	if d.buffer == nil {
		m.message = "Attach a bounded selection from a source file; large-file pages are not sent automatically"
		return
	}
	value, label := d.buffer.Text(), filepath.Base(d.path)
	if selection {
		r := d.buffer.Selection().Range()
		if r.Start == r.End {
			m.message = "Select text in the editor first"
			return
		}
		value, _ = d.buffer.RangeText(r)
		label += " (selection)"
	}
	if len(value) > 64<<10 {
		m.message = "Select a smaller region to attach to Copilot (limit 64 KiB)"
		return
	}
	m.chatContext, m.chatContextLabel = value, label
}
func wrapChatText(text string, width float32) []string {
	// Only shape the visible tail, keeping redraw cost bounded during streaming.
	r := []rune(text)
	if len(r) > 4000 {
		text = string(r[len(r)-4000:])
	}
	var result []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		r := []rune(line)
		if len(r) == 0 {
			result = append(result, "")
			continue
		}
		for len(r) > 0 {
			low, high := 1, len(r)
			for low < high {
				mid := (low + high + 1) / 2
				w, _ := ui.MeasureText(string(r[:mid]), 13, "")
				if w <= width {
					low = mid
				} else {
					high = mid - 1
				}
			}
			end := low
			if end < len(r) {
				for i := end - 1; i > end/2; i-- {
					if r[i] == ' ' {
						end = i + 1
						break
					}
				}
			}
			result = append(result, string(r[:end]))
			r = r[end:]
		}
	}
	return result
}
