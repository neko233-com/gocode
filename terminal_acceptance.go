package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/neko233-com/gocode/internal/terminal"
	ui "github.com/neko233-com/godesktop"
)

type terminalAcceptance struct {
	phase          int
	frame          uint64
	tick, verified bool
	failure        string
	marker         string
	editorVersion  int
	resizeWidth    float32
	started        time.Time
}

func terminalHasColor(f *terminal.Frame, text string, color uint32) bool {
	for _, line := range f.Lines {
		var run strings.Builder
		for _, cell := range line {
			if cell.Width == 0 {
				continue
			}
			if cell.Foreground != color {
				run.Reset()
				continue
			}
			run.WriteString(cell.Text)
			if strings.Contains(run.String(), text) {
				return true
			}
		}
	}
	return false
}
func shellPromptReady(f *terminal.Frame) bool {
	if f.CursorY < 0 || f.CursorY >= len(f.Lines) {
		return false
	}
	var line strings.Builder
	for _, cell := range f.Lines[f.CursorY] {
		if cell.Width > 0 {
			line.WriteString(cell.Text)
		}
	}
	return strings.TrimSpace(line.String()) == "gocode-test>"
}
func (a *terminalAcceptance) step(cx *ui.Context, m *model) {
	fail := func(err error) { a.failure = err.Error(); cx.Quit() }
	if a.started.IsZero() {
		a.started = time.Now()
	}
	if time.Since(a.started) > 20*time.Second {
		tab := m.currentTerminal()
		detail := "no terminal"
		if tab != nil && tab.frame != nil {
			detail = fmt.Sprintf("size=%+v cursor=%d,%d exited=%t\n%s", tab.frame.Size, tab.frame.CursorX, tab.frame.CursorY, tab.frame.Exited, tab.frame.Text())
		}
		fail(fmt.Errorf("phase %d timed out: %s", a.phase, detail))
		return
	}
	send := func(command string) {
		if err := m.currentTerminal().session.SendText(command); err != nil {
			fail(err)
			return
		}
		m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 13})
	}
	capture := func(name string) bool {
		directory := os.Getenv("GOCODE_TERMINAL_SCREENSHOTS")
		if directory == "" {
			return true
		}
		if err := captureTerminalAcceptance(m.workspace, filepath.Join(directory, name+".png")); err != nil {
			fail(err)
			return false
		}
		return true
	}
	if a.phase == 0 && cx.RenderedFrames() >= 2 {
		a.editorVersion = m.current().buffer.Version()
		m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 192, Modifiers: ui.ModifierControl})
		a.phase = 1
	}
	tab := m.currentTerminal()
	if tab != nil && tab.state != "" && tab.name == "Failed" {
		fail(fmt.Errorf("shell startup: %s", tab.state))
		return
	}
	if tab != nil && tab.frame != nil && tab.frame.Error != "" {
		fail(fmt.Errorf("shell: %s", tab.frame.Error))
		return
	}
	if tab != nil && tab.frame != nil {
		f := tab.frame
		switch a.phase {
		case 1:
			if shellPromptReady(f) && f.Size == tab.size {
				command := "echo 'NATIVE_INPUT_HIGHLIGHT'"
				if runtime.GOOS == "windows" {
					command = "Write-Output 'NATIVE_INPUT_HIGHLIGHT'"
				}
				for _, r := range command {
					m.input(cx, ui.InputEvent{Kind: ui.Character, Key: int(r)})
				}
				a.phase = 2
			}
		case 2:
			token := "echo"
			if runtime.GOOS == "windows" {
				token = "Write-Output"
			}
			if terminalHasColor(f, token, 0xdcdcaa) && terminalHasColor(f, "NATIVE_INPUT_HIGHLIGHT", 0xce9178) {
				a.frame = cx.RenderedFrames()
				a.phase = 3
			}
		case 3:
			if cx.RenderedFrames() > a.frame {
				if !capture("input-highlight") {
					return
				}
				m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 13})
				a.phase = 4
			}
		case 4:
			if shellPromptReady(f) && strings.Count(f.Text(), "NATIVE_INPUT_HIGHLIGHT") >= 2 {
				command := `printf '\033[38;2;229;192;123mNATIVE_TRUECOLOR\033[0m\n'`
				if runtime.GOOS == "windows" {
					command = `[Console]::WriteLine("$([char]27)[38;2;229;192;123mNATIVE_TRUECOLOR$([char]27)[0m")`
				}
				send(command)
				a.phase = 5
			}
		case 5:
			if shellPromptReady(f) && terminalHasColor(f, "NATIVE_TRUECOLOR", 0xe5c07b) {
				m.terminalHeight = 360
				width, err := resizeTerminalAcceptance(m.workspace)
				if err != nil {
					fail(err)
					return
				}
				a.resizeWidth = width
				a.phase = 6
			}
		case 6:
			width, _ := cx.WindowSize()
			if a.resizeWidth > 0 {
				width = a.resizeWidth
			}
			cw, _ := terminalMetrics()
			expectedColumns := max(2, min(400, int((width-290-24)/cw)))
			if f.Size == tab.size && f.Size.Rows >= 12 && f.Size.Columns == expectedColumns {
				a.marker = fmt.Sprintf("NATIVE_SIZE:%d", f.Size.Columns)
				command := `printf 'NATIVE_SIZE:%s\n' "$COLUMNS"`
				if runtime.GOOS == "windows" {
					command = `Write-Output ('NATIVE_SIZE:' + $Host.UI.RawUI.WindowSize.Width)`
				}
				send(command)
				a.phase = 7
			}
		case 7:
			if shellPromptReady(f) && strings.Contains(f.Text(), a.marker) {
				a.frame = cx.RenderedFrames()
				a.phase = 8
			}
		case 8:
			if cx.RenderedFrames() > a.frame {
				if !capture("output-resized") {
					return
				}
				command := `echo INTERRUPT_STARTED; sleep 60`
				if runtime.GOOS == "windows" {
					command = `Write-Output 'INTERRUPT_STARTED'; Start-Sleep -Seconds 60`
				}
				send(command)
				a.phase = 9
			}
		case 9:
			if strings.Count(f.Text(), "INTERRUPT_STARTED") >= 2 {
				m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 'C', Modifiers: ui.ModifierControl})
				a.phase = 10
			}
		case 10:
			if shellPromptReady(f) {
				send("exit")
				a.phase = 11
			}
		case 11:
			if f.Exited && f.ExitCode == 0 {
				if m.current().buffer.Version() != a.editorVersion || m.current().dirty() {
					fail(fmt.Errorf("terminal input modified the editor"))
					return
				}
				a.verified = true
				cx.Quit()
				return
			}
		}
	}
	if !a.tick {
		a.tick = true
		time.AfterFunc(30*time.Millisecond, func() { cx.Dispatch(func() { a.tick = false }) })
	}
}
