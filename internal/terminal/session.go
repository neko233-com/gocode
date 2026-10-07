// Package terminal owns a real PTY/ConPTY process and VT screen. It publishes
// immutable bounded frames; no process I/O or terminal parser runs on the UI.
package terminal

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

const MaxInputBytes = 64 << 10
const MaxHistory = 1000

var ErrBusy = errors.New("terminal input queue is full; input was not accepted")

type Size struct{ Columns, Rows int }

func (s Size) valid() bool { return s.Columns >= 2 && s.Columns <= 400 && s.Rows >= 2 && s.Rows <= 160 }

type Config struct {
	Command           []string
	Directory         string
	Environment       []string
	Name              string
	History           int
	WindowsArguments  *string
	InitialMessage    string
	StrictEnvironment bool
	cleanup           func()
	trace             func(string, []byte) // Internal owned-fixture diagnostics only.
}

// Discard releases a prepared shell profile when startup is cancelled before
// Start takes ownership. Do not call it on a config owned by a running Session.
func (c Config) Discard() {
	if c.cleanup != nil {
		c.cleanup()
	}
}

type Cell struct {
	Text                   string
	Width                  int
	Foreground, Background uint32
	Attrs                  uint8
	Underline              uint8
}
type Frame struct {
	Size             Size
	Lines            [][]Cell
	CursorX, CursorY int
	CursorVisible    bool
	CursorStyle      int
	Title            string
	Alternate        bool
	History, Offset  int
	Generation       uint64
	Exited           bool
	ExitCode         int
	Error            string
}

func (f *Frame) Text() string {
	var text strings.Builder
	for _, line := range f.Lines {
		for _, cell := range line {
			if cell.Width > 0 {
				text.WriteString(cell.Text)
			}
		}
		text.WriteByte('\n')
	}
	return text.String()
}

type request struct {
	key    uv.KeyPressEvent
	text   string
	paste  bool
	size   Size
	scroll int
	kind   uint8
}
type Session struct {
	backend       *processTerminal
	emulator      *vt.Emulator
	mu            sync.Mutex
	ctx           context.Context
	cancel        context.CancelFunc
	requests      chan request
	writes        chan []byte
	updates       chan struct{}
	done          chan struct{}
	outputDone    chan struct{}
	frame         atomic.Pointer[Frame]
	generation    uint64
	cursorVisible bool
	cursorStyle   int
	offset        int
	exited        bool
	exitCode      int
	failure       error
	title         string
	workers       sync.WaitGroup
	stopOnce      sync.Once
	trace         func(string, []byte)
}

func Start(parent context.Context, config Config, size Size) (_ *Session, failure error) {
	defer func() {
		if failure != nil && config.cleanup != nil {
			config.cleanup()
		}
	}()
	if !size.valid() || len(config.Command) == 0 || len(config.Command) > 128 {
		return nil, errors.New("invalid terminal size or command")
	}
	if parent == nil {
		return nil, errors.New("terminal context is required")
	}
	total := len(config.InitialMessage)
	if !utf8.ValidString(config.InitialMessage) || len(config.InitialMessage) > 8192 {
		return nil, errors.New("invalid terminal initial message")
	}
	if config.WindowsArguments != nil {
		if runtime.GOOS != "windows" || !utf8.ValidString(*config.WindowsArguments) || strings.ContainsRune(*config.WindowsArguments, 0) {
			return nil, errors.New("invalid Windows terminal arguments")
		}
		total += len(*config.WindowsArguments)
	}
	for _, arg := range config.Command {
		total += len(arg)
		if !utf8.ValidString(arg) || strings.ContainsRune(arg, 0) {
			return nil, errors.New("invalid terminal command")
		}
	}
	for _, entry := range config.Environment {
		total += len(entry)
		if !utf8.ValidString(entry) || strings.ContainsRune(entry, 0) {
			return nil, errors.New("invalid terminal environment")
		}
	}
	if total > 1<<20 {
		return nil, errors.New("terminal startup exceeds size policy")
	}
	if err := parent.Err(); err != nil {
		return nil, err
	}
	if config.Directory == "" {
		config.Directory = "."
	}
	directory, err := filepath.Abs(config.Directory)
	if err != nil {
		return nil, err
	}
	config.Directory = directory
	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		return nil, errors.New("terminal directory must be an existing directory")
	}
	if config.Environment == nil && !config.StrictEnvironment {
		config.Environment = os.Environ()
	}
	config.Environment = ProcessEnvironment(config.Environment, config.StrictEnvironment)
	if config.History <= 0 {
		config.History = MaxHistory
	}
	config.History = min(config.History, MaxHistory)
	backend, err := startBackend(parent, config, size)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	s := &Session{backend: backend, emulator: vt.NewEmulator(size.Columns, size.Rows), ctx: ctx, cancel: cancel, requests: make(chan request, 64), writes: make(chan []byte, 64), updates: make(chan struct{}, 1), done: make(chan struct{}), outputDone: make(chan struct{}), cursorVisible: true, exitCode: -1}
	s.emulator.SetScrollbackSize(config.History)
	s.trace = config.trace
	s.emulator.SetDefaultForegroundColor(color.RGBA{0xcc, 0xcc, 0xcc, 255})
	s.emulator.SetDefaultBackgroundColor(color.RGBA{0x18, 0x18, 0x18, 255})
	palette := []uint32{0, 0xcd3131, 0x0dbc79, 0xe5e510, 0x2472c8, 0xbc3fbc, 0x11a8cd, 0xe5e5e5, 0x666666, 0xf14c4c, 0x23d18b, 0xf5f543, 0x3b8eea, 0xd670d6, 0x29b8db, 0xffffff}
	for index, value := range palette {
		s.emulator.SetIndexedColor(index, color.RGBA{uint8(value >> 16), uint8(value >> 8), uint8(value), 255})
	}
	s.emulator.RegisterOscHandler(8, func([]byte) bool { return true })  // No automatic hyperlink action/storage.
	s.emulator.RegisterOscHandler(52, func([]byte) bool { return true }) // Output cannot set/read the user's clipboard.
	if config.InitialMessage != "" {
		var filter controlFilter
		_, _ = s.emulator.Write(filter.feed([]byte(config.InitialMessage + "\r\n")))
	}
	s.emulator.SetCallbacks(vt.Callbacks{Title: func(title string) {
		if len(title) > 256 {
			title = title[:256]
			for !utf8.ValidString(title) {
				title = title[:len(title)-1]
			}
		}
		s.title = title
	}, CursorVisibility: func(visible bool) { s.cursorVisible = visible }, CursorStyle: func(style vt.CursorStyle, _ bool) { s.cursorStyle = int(style) }})
	s.mu.Lock()
	s.publish()
	s.mu.Unlock()
	s.workers.Go(s.readOutput)
	s.workers.Go(s.readInput)
	s.workers.Go(s.writeInput)
	s.workers.Go(s.handleRequests)
	s.workers.Go(func() {
		ticker := time.NewTicker(time.Second / 60)
		defer ticker.Stop()
		var published uint64
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.mu.Lock()
				if s.generation != published {
					s.publish()
					published = s.generation
				}
				s.mu.Unlock()
			}
		}
	})
	s.workers.Go(func() {
		code, err := backend.Wait()
		backend.End()
		// Drain the real final output before publishing exit. Cancellation may
		// interrupt draining; descendants holding a PTY cannot delay shutdown.
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-s.outputDone:
		case <-s.ctx.Done():
		case <-timer.C:
		}
		timer.Stop()
		s.mu.Lock()
		s.exited, s.exitCode = true, code
		if err != nil {
			s.failure = err
		}
		s.generation++
		s.publish()
		s.mu.Unlock()
		s.cancel()
	})
	go func() {
		<-ctx.Done()
		s.stop()
		s.workers.Wait()
		if config.cleanup != nil {
			config.cleanup()
		}
		close(s.done)
	}()
	return s, nil
}
func (s *Session) PID() int                 { return s.backend.PID() }
func (s *Session) Updates() <-chan struct{} { return s.updates }
func (s *Session) Done() <-chan struct{}    { return s.done }
func (s *Session) Snapshot() *Frame         { return s.frame.Load() }
func (s *Session) Close()                   { s.cancel() }
func (s *Session) Wait(ctx context.Context) error {
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (s *Session) enqueue(r request) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	select {
	case s.requests <- r:
		return nil
	default:
		return ErrBusy
	}
}
func (s *Session) SendKey(key uv.KeyPressEvent) error {
	if len(key.Text) > MaxInputBytes || !utf8.ValidString(key.Text) {
		return errors.New("invalid terminal key text")
	}
	return s.enqueue(request{key: key})
}
func (s *Session) SendText(text string) error {
	if len(text) > MaxInputBytes || !utf8.ValidString(text) {
		return errors.New("terminal input exceeds UTF-8/size policy")
	}
	return s.enqueue(request{text: text, kind: 1})
}
func (s *Session) Paste(text string) error {
	if len(text) > MaxInputBytes || !utf8.ValidString(text) {
		return errors.New("terminal paste exceeds UTF-8/size policy")
	}
	return s.enqueue(request{text: text, paste: true, kind: 1})
}
func (s *Session) Resize(size Size) error {
	if !size.valid() {
		return errors.New("invalid terminal size")
	}
	return s.enqueue(request{size: size, kind: 2})
}
func (s *Session) Scroll(lines int) error { return s.enqueue(request{scroll: lines, kind: 3}) }
func (s *Session) fail(err error) {
	if err == nil {
		return
	}
	// A VT writer can hold mu while blocked on its pipe. Unblock it before
	// acquiring mu, including when the bounded write queue overflows.
	s.cancel()
	_ = s.emulator.InputPipe().(io.Closer).Close()
	s.mu.Lock()
	if s.failure == nil {
		s.failure = err
	}
	s.generation++
	s.publish()
	s.mu.Unlock()
}
func (s *Session) stop() {
	s.stopOnce.Do(func() {
		// Closing the I/O writer, rather than Emulator.Close, avoids racing the
		// upstream Read method's unguarded closed flag while unblocking VT replies.
		_ = s.emulator.InputPipe().(io.Closer).Close()
		_ = s.backend.Close()
	})
}
func (s *Session) readInput() {
	buffer := make([]byte, 8<<10)
	for {
		n, err := s.emulator.Read(buffer)
		if n > 0 {
			value := append([]byte{}, buffer[:n]...)
			select {
			case s.writes <- value:
			case <-s.ctx.Done():
				return
			default:
				s.fail(errors.New("terminal write backlog exceeded the bounded queue"))
				return
			}
		}
		if err != nil {
			return
		}
	}
}
func (s *Session) writeInput() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case value := <-s.writes:
			if s.trace != nil {
				s.trace("input", value)
			}
			n, err := s.backend.Write(value)
			if s.trace != nil {
				s.trace("write-result", []byte(fmt.Sprintf("%d %v", n, err)))
			}
			if err != nil || n != len(value) {
				s.fail(fmt.Errorf("terminal input: %w", errors.Join(err, io.ErrShortWrite)))
				return
			}
		}
	}
}
func (s *Session) handleRequests() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case r := <-s.requests:
			if r.kind == 2 {
				if err := s.backend.Resize(r.size); err != nil {
					s.fail(err)
					return
				}
			}
			s.mu.Lock()
			switch r.kind {
			case 0:
				s.offset = 0
				s.emulator.SendKey(r.key)
			case 1:
				s.offset = 0
				if r.paste {
					s.emulator.Paste(r.text)
				} else {
					s.emulator.SendText(r.text)
				}
			case 2:
				s.emulator.Resize(r.size.Columns, r.size.Rows)
			case 3:
				s.offset = max(0, min(s.emulator.ScrollbackLen(), s.offset+r.scroll))
			}
			s.generation++
			s.mu.Unlock()
		}
	}
}
func (s *Session) readOutput() {
	defer close(s.outputDone)
	buffer := make([]byte, 8<<10)
	var filter controlFilter
	for {
		n, err := s.backend.Read(buffer)
		if n > 0 && s.ctx.Err() == nil {
			if s.trace != nil {
				s.trace("output", buffer[:n])
			}
			filtered := filter.feed(buffer[:n])
			s.mu.Lock()
			_, writeErr := s.emulator.Write(filtered)
			s.limitCells()
			s.generation++
			s.mu.Unlock()
			if writeErr != nil {
				s.fail(writeErr)
				return
			}
		}
		if err != nil {
			if s.ctx.Err() == nil && !isTerminalEOF(err) {
				s.fail(err)
			}
			return
		}
	}
}
func (s *Session) limitCells() {
	for y := range s.emulator.Height() {
		for x := range s.emulator.Width() {
			c := s.emulator.CellAt(x, y)
			if c != nil && len(c.Content) > 256 {
				copy := *c
				limit := 256
				for limit > 0 && !utf8.RuneStart(copy.Content[limit]) {
					limit--
				}
				copy.Content = copy.Content[:limit]
				s.emulator.SetCell(x, y, &copy)
			}
		}
	}
}
func rgb(value color.Color, fallback uint32) uint32 {
	if value == nil {
		return fallback
	}
	r, g, b, _ := value.RGBA()
	return uint32(r>>8)<<16 | uint32(g>>8)<<8 | uint32(b>>8)
}
func (s *Session) publish() {
	e := s.emulator
	size := Size{e.Width(), e.Height()}
	history := e.ScrollbackLen()
	s.offset = min(s.offset, history)
	if e.IsAltScreen() {
		s.offset = 0
	}
	f := &Frame{Size: size, Lines: make([][]Cell, size.Rows), CursorVisible: s.cursorVisible, CursorStyle: s.cursorStyle, Title: s.title, Alternate: e.IsAltScreen(), History: history, Offset: s.offset, Generation: s.generation, Exited: s.exited, ExitCode: s.exitCode}
	position := e.CursorPosition()
	f.CursorX, f.CursorY = position.X, position.Y+s.offset
	if s.failure != nil {
		f.Error = s.failure.Error()
	}
	for y := range size.Rows {
		f.Lines[y] = make([]Cell, size.Columns)
		for x := range size.Columns {
			if x > 0 && f.Lines[y][x-1].Width > 1 {
				f.Lines[y][x] = Cell{Width: 0}
				continue
			}
			var c *uv.Cell
			index := y - s.offset
			if index < 0 {
				c = e.ScrollbackCellAt(x, history+index)
			} else {
				c = e.CellAt(x, index)
			}
			cell := Cell{Text: " ", Width: 1, Foreground: 0xcccccc, Background: 0x181818}
			if c != nil && !c.IsZero() {
				cell.Text, cell.Width = c.Content, c.Width
				cell.Foreground, cell.Background = rgb(c.Style.Fg, cell.Foreground), rgb(c.Style.Bg, cell.Background)
				cell.Attrs, cell.Underline = c.Style.Attrs, uint8(c.Style.Underline)
				if cell.Attrs&uv.AttrReverse != 0 {
					cell.Foreground, cell.Background = cell.Background, cell.Foreground
				}
				if cell.Attrs&uv.AttrConceal != 0 {
					cell.Foreground = cell.Background
				}
			}
			f.Lines[y][x] = cell
		}
	}
	s.frame.Store(f)
	select {
	case s.updates <- struct{}{}:
	default:
	}
}

// CloseAndWait is intended for process shutdown; a blocked OS operation cannot
// keep the UI shutdown caller waiting indefinitely.
func (s *Session) CloseAndWait() error {
	s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.Wait(ctx)
}
