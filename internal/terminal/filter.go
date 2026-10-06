package terminal

const maxControlBytes = 8 << 10

// controlFilter keeps fragmented terminal string controls bounded before VT
// parsing. Overlong OSC/DCS/APC/PM/SOS are discarded as complete controls, never
// interpreted as printable text or allowed to grow the upstream parser buffer.
type controlFilter struct {
	escape        bool
	stringMode    bool
	stringEscape  bool
	oversized     bool
	osc           bool
	pending       []byte
	utf8Remaining int
}

func (f *controlFilter) feed(data []byte) []byte {
	result := make([]byte, 0, len(data))
	for _, b := range data {
		raw := f.utf8Remaining == 0
		if f.utf8Remaining > 0 && b >= 0x80 && b < 0xc0 {
			f.utf8Remaining--
		} else {
			f.utf8Remaining = 0
			switch {
			case b >= 0xc2 && b <= 0xdf:
				f.utf8Remaining = 1
			case b >= 0xe0 && b <= 0xef:
				f.utf8Remaining = 2
			case b >= 0xf0 && b <= 0xf4:
				f.utf8Remaining = 3
			}
		}
		if f.stringMode {
			if !f.oversized {
				f.pending = append(f.pending, b)
				if len(f.pending) > maxControlBytes {
					f.oversized = true
					f.pending = f.pending[:0]
				}
			}
			end := (f.osc && b == 7) || (f.stringEscape && b == '\\') || (raw && b == 0x9c)
			if b == 0x18 || b == 0x1a {
				f.stringMode, f.stringEscape, f.oversized = false, false, false
				f.pending = f.pending[:0]
				result = append(result, b)
				continue
			}
			f.stringEscape = b == 27
			if end {
				if !f.oversized {
					result = append(result, f.pending...)
				}
				f.stringMode, f.stringEscape, f.oversized = false, false, false
				f.pending = f.pending[:0]
			}
			continue
		}
		if f.escape {
			f.escape = false
			if b == ']' || b == 'P' || b == '_' || b == '^' || b == 'X' {
				f.stringMode = true
				f.osc = b == ']'
				f.pending = append(f.pending[:0], 27, b)
				continue
			}
			result = append(result, 27, b)
			continue
		}
		if b == 27 {
			f.escape = true
			continue
		}
		if raw && (b == 0x9d || b == 0x90 || b == 0x9f || b == 0x9e || b == 0x98) {
			f.stringMode = true
			f.osc = b == 0x9d
			f.pending = append(f.pending[:0], b)
			continue
		}
		result = append(result, b)
	}
	return result
}
