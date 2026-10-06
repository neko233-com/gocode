// Package largefile provides cancellable, file-backed UTF-8 browsing. It never
// constructs a full document or a per-line array, including for a huge single line.
package largefile

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	BlockBytes     = 128 << 10
	MaxPoints      = 32768
	MaxRows        = 128
	PreviewBytes   = 4096
	MaxWindowBytes = 64 << 10
)

var (
	ErrChanged    = errors.New("file changed on disk; close and reopen the large-file viewer")
	ErrNotIndexed = errors.New("requested line has not been indexed yet")
)

type checkpoint struct{ offset, line, lineStart int64 }
type Stats struct {
	Size, Scanned, Lines int64
	IndexBytes           int
	Complete             bool
	EOL                  string
	Err                  error
}
type Line struct {
	Number, Offset int64
	Text           string
	Truncated      bool
}
type Window struct {
	Offset, Line, ColumnBytes int64
	Text                      string
}

type File struct {
	path      string
	file      *os.File
	info      os.FileInfo
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.RWMutex
	points    []checkpoint
	stats     Stats
}

// Open returns before indexing. Callers can read the first page immediately and
// read progress while the worker validates UTF-8 and builds a bounded sparse index.
func Open(parent context.Context, path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, errors.Join(err, errors.New("large-file browsing requires a regular file"))
	}
	ctx, cancel := context.WithCancel(parent)
	r := &File{path: path, file: f, info: info, ctx: ctx, cancel: cancel, done: make(chan struct{}), points: make([]checkpoint, 1, MaxPoints), stats: Stats{Size: info.Size(), Lines: 1, EOL: "LF"}}
	go r.scan()
	return r, nil
}

func (r *File) Stats() Stats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s := r.stats
	s.IndexBytes = cap(r.points) * 24
	return s
}
func (r *File) Wait(ctx context.Context) error {
	select {
	case <-r.done:
		return r.Stats().Err
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (r *File) Close() error {
	var err error
	r.closeOnce.Do(func() { r.cancel(); <-r.done; err = r.file.Close() })
	return err
}
func (r *File) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if s := r.Stats(); s.Err != nil {
		return s.Err
	}
	info, err := os.Stat(r.path)
	if err != nil {
		return err
	}
	if !os.SameFile(r.info, info) || info.Size() != r.info.Size() || !info.ModTime().Equal(r.info.ModTime()) {
		return ErrChanged
	}
	return nil
}

func (r *File) scan() {
	defer close(r.done)
	buf := make([]byte, BlockBytes+utf8.UTFMax)
	var offset, line, lineStart, crlf, lf int64
	carry := 0
	lineStride, byteStride := int64(1024), int64(1<<20)
	lastPoint := checkpoint{}
	finish := func(err error) { r.mu.Lock(); r.stats.Err = err; r.stats.Complete = err == nil; r.mu.Unlock() }
	add := func(p checkpoint) {
		if p.offset <= lastPoint.offset || (p.line-lastPoint.line < lineStride && p.offset-lastPoint.offset < byteStride) {
			return
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if len(r.points) == MaxPoints {
			// Coarsen in place. Index storage stays fixed even for trillions of lines.
			for i := 1; i < MaxPoints/2; i++ {
				r.points[i] = r.points[i*2]
			}
			r.points = r.points[:MaxPoints/2]
			lineStride *= 2
			byteStride *= 2
		}
		r.points = append(r.points, p)
		lastPoint = p
	}
	var previous byte
	for offset < r.info.Size() {
		if err := r.ctx.Err(); err != nil {
			finish(err)
			return
		}
		n, err := r.file.ReadAt(buf[carry:carry+min(BlockBytes, int(r.info.Size()-offset))], offset)
		if err != nil && err != io.EOF {
			finish(err)
			return
		}
		if n == 0 {
			finish(io.ErrUnexpectedEOF)
			return
		}
		data := buf[:carry+n]
		end := len(data)
		if offset+int64(n) < r.info.Size() {
			start := end - 1
			for start > 0 && end-start < utf8.UTFMax && !utf8.RuneStart(data[start]) {
				start--
			}
			if !utf8.FullRune(data[start:]) {
				end = start
			}
		}
		if !utf8.Valid(data[:end]) || bytes.IndexByte(data[:end], 0) >= 0 {
			finish(fmt.Errorf("file is not UTF-8 text near byte %d", offset-int64(carry)))
			return
		}
		base := offset - int64(carry)
		for rest, at := data[:end], 0; ; {
			i := bytes.IndexByte(rest, '\n')
			if i < 0 {
				break
			}
			idx := at + i
			if (idx > 0 && data[idx-1] == '\r') || (idx == 0 && previous == '\r') {
				crlf++
			} else {
				lf++
			}
			line++
			lineStart = base + int64(idx) + 1
			add(checkpoint{lineStart, line, lineStart})
			at = idx + 1
			rest = data[at:end]
		}
		if end > 0 {
			previous = data[end-1]
		}
		processed := base + int64(end)
		add(checkpoint{processed, line, lineStart})
		carry = len(data) - end
		copy(buf, data[end:])
		offset += int64(n)
		r.mu.Lock()
		r.stats.Scanned = processed
		r.stats.Lines = line + 1
		if crlf > 0 && lf == 0 {
			r.stats.EOL = "CRLF"
		} else if crlf > 0 {
			r.stats.EOL = "Mixed"
		}
		r.mu.Unlock()
	}
	if carry != 0 {
		finish(errors.New("incomplete UTF-8 at end of file"))
		return
	}
	if err := r.check(r.ctx); err != nil {
		finish(err)
		return
	}
	finish(nil)
}

func (r *File) pointForLine(line int64) (checkpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if line < 0 || line >= r.stats.Lines {
		return checkpoint{}, ErrNotIndexed
	}
	i := sort.Search(len(r.points), func(i int) bool { return r.points[i].line > line }) - 1
	return r.points[max(0, i)], nil
}

// Lines returns bounded previews. A page ends at the first line longer than the
// read block, allowing immediate display without scanning a huge terminator.
// Indexed line navigation can jump past it; Bytes jumps directly inside it.
func (r *File) Lines(ctx context.Context, start int64, count int) ([]Line, error) {
	if count < 1 || count > MaxRows {
		return nil, errors.New("invalid page row count")
	}
	if err := r.check(ctx); err != nil {
		return nil, err
	}
	p, err := r.pointForLine(start)
	if err != nil {
		return nil, err
	}
	offset := p.offset
	if p.line == start {
		offset = p.lineStart
	}
	reader := bufio.NewReaderSize(io.NewSectionReader(r.file, offset, r.info.Size()-offset), BlockBytes)
	line := p.line
	result := make([]Line, 0, count)
	for line < start+int64(count) && offset <= r.info.Size() {
		lineOffset := offset
		var text []byte
		truncated := false
		for {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if r.ctx.Err() != nil {
				return nil, r.ctx.Err()
			}
			chunk, readErr := reader.ReadSlice('\n')
			offset += int64(len(chunk))
			if line >= start {
				remaining := PreviewBytes - len(text)
				text = append(text, chunk[:min(len(chunk), remaining)]...)
				if len(chunk) > remaining {
					truncated = true
				}
			}
			if readErr == bufio.ErrBufferFull && line >= start && truncated {
				text = trimUTF8Tail(text)
				if !utf8.Valid(text) || bytes.IndexByte(text, 0) >= 0 {
					return nil, errors.New("page is not UTF-8 text")
				}
				// Publish a long-line preview immediately. Finding its terminator
				// could otherwise delay the very first view by a gigabyte scan.
				result = append(result, Line{line, lineOffset, string(text), true})
				return result, r.check(ctx)
			}
			if readErr == bufio.ErrBufferFull {
				continue
			}
			if readErr != nil && readErr != io.EOF {
				return nil, readErr
			}
			if line >= start {
				if truncated {
					text = trimUTF8Tail(text)
				}
				if !utf8.Valid(text) || bytes.IndexByte(text, 0) >= 0 {
					return nil, errors.New("page is not UTF-8 text")
				}
				result = append(result, Line{line, lineOffset, strings.TrimSuffix(strings.TrimSuffix(string(text), "\n"), "\r"), truncated})
			}
			line++
			if readErr == io.EOF {
				return result, r.check(ctx)
			}
			break
		}
	}
	return result, r.check(ctx)
}

// Bytes is direct random access, also usable before indexing completes. The
// returned offset advances past a partial UTF-8 codepoint at the requested byte.
func (r *File) Bytes(ctx context.Context, offset int64, count int) (Window, error) {
	if offset < 0 || offset > r.info.Size() || count < 1 || count > MaxWindowBytes {
		return Window{}, errors.New("invalid byte window")
	}
	if err := r.check(ctx); err != nil {
		return Window{}, err
	}
	r.mu.RLock()
	i := sort.Search(len(r.points), func(i int) bool { return r.points[i].offset > offset }) - 1
	p := r.points[max(0, i)]
	scanned := r.stats.Scanned
	r.mu.RUnlock()
	data := make([]byte, min(count+utf8.UTFMax, int(r.info.Size()-offset)))
	n, err := r.file.ReadAt(data, offset)
	if err != nil && err != io.EOF {
		return Window{}, err
	}
	data = data[:n]
	for len(data) > 0 && !utf8.RuneStart(data[0]) {
		data = data[1:]
		offset++
	}
	end := len(trimUTF8Tail(data[:min(len(data), count)]))
	if !utf8.Valid(data[:end]) {
		return Window{}, errors.New("byte window is not UTF-8 text")
	}
	if bytes.IndexByte(data[:end], 0) >= 0 {
		return Window{}, errors.New("byte window contains NUL")
	}
	w := Window{Offset: offset, Line: -1, ColumnBytes: -1, Text: string(data[:end])}
	// Resolve coordinates only within indexed data. Random byte access never
	// waits for a scan of all preceding gigabytes.
	if offset <= scanned {
		buf := make([]byte, BlockBytes)
		line, lineStart, at := p.line, p.lineStart, p.offset
		for at < offset {
			if ctx.Err() != nil {
				return Window{}, ctx.Err()
			}
			if r.ctx.Err() != nil {
				return Window{}, r.ctx.Err()
			}
			n, err := r.file.ReadAt(buf[:min(int64(len(buf)), offset-at)], at)
			if err != nil && err != io.EOF {
				return Window{}, err
			}
			if n == 0 {
				return Window{}, io.ErrUnexpectedEOF
			}
			for rest, pos := buf[:n], 0; ; {
				j := bytes.IndexByte(rest, '\n')
				if j < 0 {
					break
				}
				line++
				lineStart = at + int64(pos+j) + 1
				pos += j + 1
				rest = buf[pos:n]
			}
			at += int64(n)
		}
		w.Line = line
		w.ColumnBytes = offset - lineStart
	}
	return w, r.check(ctx)
}

// Only the final incomplete codepoint can be trimmed. Invalid bytes in the body
// must remain visible to the caller's validation, rather than disappearing.
func trimUTF8Tail(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	start := len(data) - 1
	for start > 0 && len(data)-start < utf8.UTFMax && !utf8.RuneStart(data[start]) {
		start--
	}
	if !utf8.FullRune(data[start:]) {
		return data[:start]
	}
	return data
}
