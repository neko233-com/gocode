package terminal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRealDefaultShellHighlightsInputBeforeExecution(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		t.Skip("default highlighting profiles target Windows and macOS")
	}
	if runtime.GOOS == "darwin" {
		t.Setenv("SHELL", "/bin/zsh")
		t.Setenv("ZDOTDIR", t.TempDir())
		if err := os.WriteFile(os.Getenv("ZDOTDIR")+"/.zshrc", []byte("PROMPT='gocode-test> '\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	config, err := Shell(t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	verifyShellHighlight(t, config)
}

func TestRealWindowsPowerShell51HighlightsInput(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows PowerShell compatibility")
	}
	config, err := Shell(t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	config.Command[0] = filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if _, err := os.Stat(config.Command[0]); err != nil {
		t.Skip("Windows PowerShell 5.1 unavailable")
	}
	verifyShellHighlight(t, config)
}

func verifyShellHighlight(t *testing.T, config Config) {
	t.Helper()
	command, token, quit := "echo 'HIGHLIGHT_MARKER'", "echo", "exit\r"
	if runtime.GOOS == "windows" {
		command, token = "Write-Output 'HIGHLIGHT_MARKER'", "Write-Output"
	}
	s, err := Start(context.Background(), config, Size{100, 18})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.CloseAndWait(); err != nil {
			t.Error(err)
		}
	})
	terminalUntil(t, s, func(f *Frame) bool { return strings.Contains(f.Text(), "gocode-test>") })
	if err := s.SendText(command); err != nil {
		t.Fatal(err)
	}
	terminalUntil(t, s, func(f *Frame) bool {
		return HasColoredText(f, token, 0xdcdcaa) && HasColoredText(f, "HIGHLIGHT_MARKER", 0xce9178)
	})
	if err := s.SendText("\r"); err != nil {
		t.Fatal(err)
	}
	terminalUntil(t, s, func(f *Frame) bool {
		return strings.Contains(f.Text(), "\nHIGHLIGHT_MARKER") || strings.Count(f.Text(), "HIGHLIGHT_MARKER") >= 2
	})
	if err := s.SendText(quit); err != nil {
		t.Fatal(err)
	}
	terminalUntil(t, s, func(f *Frame) bool { return f.Exited && f.ExitCode == 0 })
}

func TestRealWindowsShellUnicodeOutputFromLegacyCodePage(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows console encoding compatibility")
	}
	for _, name := range []string{"pwsh.exe", "powershell.exe"} {
		t.Run(name, func(t *testing.T) {
			path, err := exec.LookPath(name)
			if err != nil {
				t.Skip("shell unavailable")
			}
			config, err := Shell(t.TempDir(), true)
			if err != nil {
				t.Fatal(err)
			}
			config.Command[0] = path
			// Reproduce the US CI console before the actual product startup.
			index := len(config.Command) - 1
			config.Command[index] = `[Console]::InputEncoding = [Console]::OutputEncoding = [Text.Encoding]::GetEncoding(437); $OutputEncoding = [Text.Encoding]::ASCII; ` + config.Command[index]
			s, err := Start(context.Background(), config, Size{140, 18})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := s.CloseAndWait(); err != nil {
					t.Error(err)
				}
			})
			terminalUntil(t, s, func(f *Frame) bool { return strings.Contains(f.Text(), "gocode-test>") })
			command := `[Console]::WriteLine("$([char]27)[38;2;229;192;123mUNICODE_OUTPUT 世界 😀$([char]27)[0m")` + "\r"
			if err := s.SendText(command); err != nil {
				t.Fatal(err)
			}
			terminalUntil(t, s, func(f *Frame) bool { return HasColoredText(f, "UNICODE_OUTPUT 世界 😀", 0xe5c07b) })
			if err := s.SendText("exit\r"); err != nil {
				t.Fatal(err)
			}
			terminalUntil(t, s, func(f *Frame) bool { return f.Exited && f.ExitCode == 0 })
		})
	}
}

// HasColoredText checks the shell's real VT cells.
func HasColoredText(frame *Frame, text string, color uint32) bool {
	for _, line := range frame.Lines {
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
