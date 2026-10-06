package terminal

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

func TestTerminalChild(t *testing.T) {
	if os.Getenv("GOCODE_PTY_CHILD") != "1" {
		return
	}
	if err := fixtureConsoleMode(); err != nil {
		fmt.Println("FIXTURE_MODE_FAILED", err)
		os.Exit(3)
	}
	if os.Getenv("GOCODE_PTY_DESCENDANT") == "1" {
		for {
			time.Sleep(time.Second)
		}
	}
	fmt.Print("\x1b[2J\x1b[H\x1b[38;2;229;192;123mCOLOR\x1b[0m\r\nPTY_READY\r\n")
	reader := bufio.NewReader(os.Stdin)
	for {
		var input strings.Builder
		var err error
		for {
			var r rune
			r, _, err = reader.ReadRune()
			if err != nil || r == '\r' || r == '\n' {
				break
			}
			input.WriteRune(r)
		}
		if err != nil {
			os.Exit(2)
		}
		line := strings.TrimSpace(input.String())
		if line == "" {
			continue
		}
		switch line {
		case "spawn":
			command := exec.Command(os.Args[0], "-test.run=^TestTerminalChild$")
			command.Env = append(os.Environ(), "GOCODE_PTY_DESCENDANT=1")
			command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := command.Start(); err != nil {
				fmt.Println("SPAWN_FAILED", err)
				continue
			}
			_ = os.WriteFile("terminal-child.pid", []byte(strconv.Itoa(command.Process.Pid)), 0600)
			go command.Wait()
			fmt.Println("SPAWN_READY")
		case "flood":
			for i := 0; i < 5000; i++ {
				fmt.Printf("LINE_%04d %s\r\n", i, strings.Repeat("output", 30))
			}
			fmt.Print("\x1b]2;" + strings.Repeat("x", maxControlBytes*3) + "\aHISTORY_DONE\r\n")
		case "quit":
			fmt.Print("PTY_EXIT\r\n")
			os.Exit(7)
		case "alternate":
			fmt.Print("\x1b[?1049h\x1b[2J\x1b[H\x1b[32mALT_SCREEN\x1b[0m")
		case "primary":
			fmt.Print("\x1b[?1049l\r\nMAIN_SCREEN\r\n")
		default:
			fmt.Printf("PTY_ECHO:%s\r\n", line)
		}
	}
}
func childSession(t *testing.T, size Size) *Session {
	t.Helper()
	config := Config{Command: []string{os.Args[0], "-test.run=^TestTerminalChild$"}, Directory: t.TempDir(), Environment: append(os.Environ(), "GOCODE_PTY_CHILD=1"), History: 16}
	s, err := Start(context.Background(), config, size)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.CloseAndWait(); err != nil {
			t.Error(err)
		}
	})
	return s
}
func terminalUntil(t *testing.T, s *Session, condition func(*Frame) bool) *Frame {
	t.Helper()
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	for {
		frame := s.Snapshot()
		if condition(frame) {
			return frame
		}
		select {
		case <-s.Updates():
		case <-time.After(20 * time.Millisecond):
		case <-deadline.C:
			t.Fatalf("terminal condition timed out: size=%+v generation=%d exited=%t code=%d error=%s\n%s", frame.Size, frame.Generation, frame.Exited, frame.ExitCode, frame.Error, frame.Text())
		}
	}
}
func TestRealTerminalColorUnicodeInputResizeAlternateAndExit(t *testing.T) {
	s := childSession(t, Size{80, 12})
	frame := terminalUntil(t, s, func(f *Frame) bool { return strings.Contains(f.Text(), "PTY_READY") })
	colored := false
	for _, line := range frame.Lines {
		for _, cell := range line {
			if cell.Text == "C" && cell.Foreground == 0xe5c07b {
				colored = true
			}
		}
	}
	if !colored {
		t.Fatal("real terminal SGR was flattened")
	}
	if err := s.SendText("hello 世界 😀"); err != nil {
		t.Fatal(err)
	}
	if err := s.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter}); err != nil {
		t.Fatal(err)
	}
	terminalUntil(t, s, func(f *Frame) bool { return strings.Contains(f.Text(), "PTY_ECHO:hello 世界 😀") })
	if err := s.Resize(Size{100, 16}); err != nil {
		t.Fatal(err)
	}
	terminalUntil(t, s, func(f *Frame) bool { return f.Size == (Size{100, 16}) })
	if err := s.SendText("alternate\r"); err != nil {
		t.Fatal(err)
	}
	terminalUntil(t, s, func(f *Frame) bool { return f.Alternate && strings.Contains(f.Text(), "ALT_SCREEN") })
	if err := s.SendText("primary\r"); err != nil {
		t.Fatal(err)
	}
	terminalUntil(t, s, func(f *Frame) bool { return !f.Alternate && strings.Contains(f.Text(), "MAIN_SCREEN") })
	if err := s.SendText("quit\r"); err != nil {
		t.Fatal(err)
	}
	terminalUntil(t, s, func(f *Frame) bool { return f.Exited && f.ExitCode == 7 && strings.Contains(f.Text(), "PTY_EXIT") })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.Wait(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestTerminalCancellationAndInputBounds(t *testing.T) {
	s := childSession(t, Size{80, 12})
	terminalUntil(t, s, func(f *Frame) bool { return strings.Contains(f.Text(), "PTY_READY") })
	if err := s.Paste(strings.Repeat("x", MaxInputBytes+1)); err == nil {
		t.Fatal("unbounded paste accepted")
	}
	if err := s.Resize(Size{10000, 10000}); err == nil {
		t.Fatal("unbounded screen accepted")
	}
	if err := s.CloseAndWait(); err != nil {
		t.Fatal(err)
	}
}
func TestControlFilterFragmentedAndOversizedStrings(t *testing.T) {
	var f controlFilter
	var result []byte
	for _, part := range []string{"before\x1b", "]2;hello", "\x1b", "\\after"} {
		result = append(result, f.feed([]byte(part))...)
	}
	if string(result) != "before\x1b]2;hello\x1b\\after" {
		t.Fatalf("fragmented control %q", result)
	}
	result = f.feed([]byte("\x1b]2;" + strings.Repeat("x", maxControlBytes*3) + "\aNEXT"))
	if string(result) != "NEXT" || cap(f.pending) > maxControlBytes*2 {
		t.Fatal("oversized string was not bounded", len(result), cap(f.pending))
	}
}

func TestControlFilterC1CancellationAndUTF8(t *testing.T) {
	var f controlFilter
	result := f.feed([]byte("\x9d2;" + strings.Repeat("x", maxControlBytes*3) + "\x9cC1_DONE"))
	if string(result) != "C1_DONE" {
		t.Fatalf("C1 control escaped limit: %q", result)
	}
	result = f.feed([]byte("\x1b]2;oversized\x18CANCEL_DONE"))
	if string(result) != "\x18CANCEL_DONE" {
		t.Fatalf("control cancel: %q", result)
	}
	result = f.feed([]byte("世界\xc2\x9d\x1b]2;\xc2\x9c\x07"))
	if string(result) != "世界\xc2\x9d\x1b]2;\xc2\x9c\x07" {
		t.Fatal("UTF-8 continuation was treated as C1 control")
	}
}

func TestRealTerminalFloodHasBoundedHistoryAndScroll(t *testing.T) {
	s := childSession(t, Size{80, 12})
	terminalUntil(t, s, func(f *Frame) bool { return strings.Contains(f.Text(), "PTY_READY") })
	if err := s.SendText("flood\r"); err != nil {
		t.Fatal(err)
	}
	f := terminalUntil(t, s, func(f *Frame) bool { return strings.Contains(f.Text(), "HISTORY_DONE") })
	if f.History > 16 || len(f.Lines) != 12 || len(f.Title) > 256 || strings.Contains(f.Title, "xxxxxxxx") {
		t.Fatal("history/control bounds", f.History, f.Title)
	}
	if err := s.Scroll(1000000); err != nil {
		t.Fatal(err)
	}
	terminalUntil(t, s, func(f *Frame) bool { return f.Offset == 16 && f.History == 16 })
	if err := s.Scroll(-1000000); err != nil {
		t.Fatal(err)
	}
	terminalUntil(t, s, func(f *Frame) bool { return f.Offset == 0 && strings.Contains(f.Text(), "HISTORY_DONE") })
}
func TestVTReplyQueueOverflowUnblocksLockedWriter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &Session{ctx: ctx, cancel: cancel, emulator: vt.NewEmulator(80, 12), writes: make(chan []byte, 1), updates: make(chan struct{}, 1)}
	// Saturate deterministically: real OS consoles may drain replies faster than
	// a flood fills them. The production reader must unblock a VT/UI actor that
	// holds mu, then publish its failure, without acquiring mu first.
	s.writes <- []byte("full")
	readDone, fed := make(chan struct{}), make(chan struct{})
	go func() { s.readInput(); close(readDone) }()
	go func() { s.mu.Lock(); s.emulator.SendText("blocked reply"); s.mu.Unlock(); close(fed) }()
	for _, done := range []chan struct{}{fed, readDone} {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("VT writer/reader deadlocked while canceling backlog")
		}
	}
	if ctx.Err() == nil || !strings.Contains(s.Snapshot().Error, "backlog") {
		t.Fatal("overflow not canceled/reported")
	}
}

func TestRealTerminalCloseKillsDescendants(t *testing.T) {
	directory := t.TempDir()
	s, err := Start(context.Background(), Config{Command: []string{os.Args[0], "-test.run=^TestTerminalChild$"}, Directory: directory, Environment: append(os.Environ(), "GOCODE_PTY_CHILD=1")}, Size{80, 12})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.CloseAndWait() })
	terminalUntil(t, s, func(f *Frame) bool { return strings.Contains(f.Text(), "PTY_READY") })
	if err := s.SendText("spawn\r"); err != nil {
		t.Fatal(err)
	}
	terminalUntil(t, s, func(f *Frame) bool { return strings.Contains(f.Text(), "SPAWN_READY") })
	data, err := os.ReadFile(filepath.Join(directory, "terminal-child.pid"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if !processRunning(pid) {
		t.Fatal("owned descendant was not running")
	}
	if err := s.CloseAndWait(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for processRunning(pid) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if processRunning(pid) {
		process, _ := os.FindProcess(pid)
		_ = process.Kill()
		t.Fatal("owned child survived terminal close")
	}
}
