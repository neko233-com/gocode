//go:build windows

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// This fixture is a genuine process attached to the controller's real ConPTY.
// It never opens a GUI or changes a global terminal, console or input device.
func TestTerminalDispatchChild(t *testing.T) {
	if os.Getenv("GOCODE_TERMINAL_DISPATCH_CHILD") != "1" {
		t.Skip("owned PTY helper only")
	}
	input, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if err != nil || windows.SetConsoleMode(input, windows.ENABLE_PROCESSED_INPUT|windows.ENABLE_VIRTUAL_TERMINAL_INPUT) != nil {
		os.Exit(3)
	}
	output, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil || windows.SetConsoleMode(output, windows.ENABLE_PROCESSED_OUTPUT|windows.ENABLE_WRAP_AT_EOL_OUTPUT|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) != nil {
		os.Exit(4)
	}
	if err := os.WriteFile(os.Getenv("GOCODE_TERMINAL_DISPATCH_PID"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		os.Exit(5)
	}
	fmt.Print("TERMINAL_DISPATCH_READY\r\n")
	reader := bufio.NewReader(os.Stdin)
	for {
		var line strings.Builder
		for {
			r, _, err := reader.ReadRune()
			if err != nil {
				os.Exit(2)
			}
			if r == '\r' || r == '\n' {
				break
			}
			line.WriteRune(r)
		}
		if line.Len() == 0 {
			continue
		}
		if line.String() == "quit" {
			fmt.Print("TERMINAL_DISPATCH_EXIT\r\n")
			os.Exit(7)
		}
		fmt.Printf("TERMINAL_DISPATCH_ECHO:%s\r\n", line.String())
	}
}

func terminalDispatchOptions(t *testing.T, root, id string) (terminalLaunchOptions, string) {
	t.Helper()
	args, err := json.Marshal([]string{"-test.run=^TestTerminalDispatchChild$"})
	if err != nil {
		t.Fatal(err)
	}
	child, pidPath := "1", filepath.Join(root, id+".pid")
	return terminalLaunchOptions{Name: id, ShellPath: os.Args[0], ShellArgs: args, CWD: root, Env: map[string]*string{"GOCODE_TERMINAL_DISPATCH_CHILD": &child, "GOCODE_TERMINAL_DISPATCH_PID": &pidPath}}, pidPath
}

func terminalDispatchUntil(t *testing.T, label string, ready func() bool) {
	t.Helper()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for !ready() {
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal(label)
		}
	}
}

func terminalDispatchPID(t *testing.T, path string) int {
	t.Helper()
	pid := 0
	terminalDispatchUntil(t, "owned PTY process did not write its PID", func() bool {
		raw, err := os.ReadFile(path)
		if err == nil {
			pid, err = strconv.Atoi(string(raw))
		}
		return err == nil && pid > 0
	})
	return pid
}

func terminalDispatchAlive(pid int) bool {
	process, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(process)
	status, err := windows.WaitForSingleObject(process, 0)
	return err == nil && status == uint32(windows.WAIT_TIMEOUT)
}

func terminalDispatchClosed(t *testing.T, tab *terminalTab, pid int) {
	t.Helper()
	select {
	case <-tab.workersDone:
	case <-time.After(2 * time.Second):
		t.Fatal("closed terminal retained its controller workers while the UI queue was full")
	}
	if terminalDispatchAlive(pid) {
		t.Fatal("closed terminal retained its owned process", pid)
	}
}

func TestTerminalRejectedStartupClosesWorkersAndPreservesEightSlotBound(t *testing.T) {
	m := testModel(t)
	root := t.TempDir()
	rejected := make(chan struct{}, 32)
	stop := m.bindTerminals(context.Background(), func(func()) bool {
		select {
		case rejected <- struct{}{}:
		default:
		}
		return false
	})
	defer stop()
	for batch := range 2 {
		tabs := make([]*terminalTab, 0, 8)
		pids := make([]int, 0, 8)
		completed := 0
		for index := range 8 {
			id := fmt.Sprintf("batch-%d-%d", batch, index)
			options, pidPath := terminalDispatchOptions(t, root, id)
			m.requestTerminal(context.Background(), id, options, func(err error) {
				if !errors.Is(err, context.Canceled) {
					t.Error("unadopted closed terminal did not cancel exactly once", err)
				}
				completed++
			})
			tabs = append(tabs, m.terminals[len(m.terminals)-1])
			pids = append(pids, terminalDispatchPID(t, pidPath))
		}
		var overflow error
		m.requestTerminal(context.Background(), "overflow", terminalLaunchOptions{}, func(err error) { overflow = err })
		if overflow == nil || !strings.Contains(overflow.Error(), "maximum 8") || len(m.terminals) != 8 || completed != 0 {
			t.Fatal("full queue bypassed the eight-terminal bound", overflow)
		}
		select {
		case <-rejected:
		case <-time.After(5 * time.Second):
			t.Fatal("real startup never reached rejected UI admission")
		}
		for _, tab := range tabs {
			m.closeTerminal(tab, 0)
		}
		if len(m.terminals) != 0 || completed != 8 {
			t.Fatal("close did not release UI slots or completed twice", completed)
		}
		for index, tab := range tabs {
			terminalDispatchClosed(t, tab, pids[index])
			if tab.session != nil || tab.frame != nil {
				t.Fatal("unadopted closed session changed UI")
			}
		}
	}
}

func TestTerminalHeldStartupCallbackCannotReviveClosedTab(t *testing.T) {
	m := testModel(t)
	mailbox := make(chan func(), 16)
	stop := m.bindTerminals(context.Background(), func(fn func()) bool { mailbox <- fn; return true })
	defer stop()
	options, pidPath := terminalDispatchOptions(t, t.TempDir(), "late-startup")
	completed := 0
	m.requestTerminal(context.Background(), "late-startup", options, func(err error) {
		if !errors.Is(err, context.Canceled) {
			t.Error(err)
		}
		completed++
	})
	tab := m.currentTerminal()
	held := saveAck(t, mailbox)
	pid := terminalDispatchPID(t, pidPath)
	if !tab.pending || tab.session != nil || completed != 0 {
		t.Fatal("queued startup was acknowledged before UI execution")
	}
	m.closeTerminal(tab, 0)
	terminalDispatchClosed(t, tab, pid)
	held()
	for len(mailbox) != 0 {
		(<-mailbox)()
	}
	if len(m.terminals) != 0 || tab.session != nil || tab.frame != nil || completed != 1 {
		t.Fatal("late startup revived a closed terminal or completed twice")
	}
}

func TestTerminalRejectedOutputClosesWorkersAfterRealUnicodeEcho(t *testing.T) {
	m := testModel(t)
	mailbox := make(chan func(), 1)
	rejected := make(chan struct{}, 1)
	var admissions atomic.Int32
	stop := m.bindTerminals(context.Background(), func(fn func()) bool {
		if admissions.Add(1) == 1 {
			mailbox <- fn
			return true
		}
		select {
		case rejected <- struct{}{}:
		default:
		}
		return false
	})
	defer stop()
	options, pidPath := terminalDispatchOptions(t, t.TempDir(), "output")
	completed := 0
	m.requestTerminal(context.Background(), "output", options, func(err error) {
		if err != nil {
			t.Error(err)
		}
		completed++
	})
	tab := m.currentTerminal()
	saveAck(t, mailbox)()
	pid := terminalDispatchPID(t, pidPath)
	session := tab.session
	if session == nil || tab.pending || completed != 1 {
		t.Fatal("genuine startup was not adopted")
	}
	if err := session.SendText("hello 世界😀\r"); err != nil {
		t.Fatal(err)
	}
	terminalDispatchUntil(t, "real PTY output did not contain Unicode echo", func() bool {
		frame := session.Snapshot()
		return frame != nil && strings.Contains(frame.Text(), "TERMINAL_DISPATCH_ECHO:hello 世界😀")
	})
	select {
	case <-rejected:
	case <-time.After(5 * time.Second):
		t.Fatal("real output never reached rejected receipt")
	}
	frame := tab.frame
	m.closeTerminal(tab, 0)
	terminalDispatchClosed(t, tab, pid)
	if len(m.terminals) != 0 || tab.frame != frame || completed != 1 {
		t.Fatal("closed output receipt changed UI or startup completion")
	}
}

func TestTerminalFinalExitReceiptSurvivesProcessContextCancellation(t *testing.T) {
	m := testModel(t)
	mailbox := make(chan func(), 16)
	var allow atomic.Bool
	var admissions atomic.Int32
	stop := m.bindTerminals(context.Background(), func(fn func()) bool {
		if admissions.Add(1) == 1 || allow.Load() {
			mailbox <- fn
			return true
		}
		return false
	})
	defer stop()
	options, pidPath := terminalDispatchOptions(t, t.TempDir(), "exit")
	m.requestTerminal(context.Background(), "exit", options, func(err error) {
		if err != nil {
			t.Error(err)
		}
	})
	tab := m.currentTerminal()
	saveAck(t, mailbox)()
	pid := terminalDispatchPID(t, pidPath)
	session := tab.session
	if session == nil {
		t.Fatal("exit fixture has no real process")
	}
	if err := session.SendText("quit\r"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-session.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("owned PTY process did not exit")
	}
	frame := session.Snapshot()
	if frame == nil || !frame.Exited || frame.ExitCode != 7 || !strings.Contains(frame.Text(), "TERMINAL_DISPATCH_EXIT") || terminalDispatchAlive(pid) {
		t.Fatal("final frame is not from the actual exited process", frame)
	}
	if len(m.terminals) != 1 || tab.frame == frame {
		t.Fatal("rejected final receipt was acknowledged off UI")
	}
	allow.Store(true)
	for len(m.terminals) != 0 {
		saveAck(t, mailbox)()
	}
	terminalDispatchClosed(t, tab, pid)
	if len(m.closedTerminals) != 1 || m.closedTerminals[0].ExitStatus == nil || m.closedTerminals[0].ExitStatus.Code == nil || *m.closedTerminals[0].ExitStatus.Code != 7 {
		t.Fatal("actual final exit was dropped or acknowledged more than once", m.closedTerminals)
	}
}

func TestTerminalRejectedStartupErrorClearsOrCancelsPendingTab(t *testing.T) {
	for _, recover := range []bool{false, true} {
		t.Run(fmt.Sprintf("recover-%t", recover), func(t *testing.T) {
			m := testModel(t)
			root := t.TempDir()
			mailbox := make(chan func(), 1)
			rejected := make(chan struct{}, 1)
			var allow atomic.Bool
			stop := m.bindTerminals(context.Background(), func(fn func()) bool {
				if allow.Load() {
					mailbox <- fn
					return true
				}
				select {
				case rejected <- struct{}{}:
				default:
				}
				return false
			})
			defer stop()
			completed := 0
			var failure error
			m.requestTerminal(context.Background(), "failed", terminalLaunchOptions{Name: "bad path", CWD: root, ShellPath: filepath.Join(root, "does-not-exist.exe")}, func(err error) { completed++; failure = err })
			tab := m.currentTerminal()
			select {
			case <-rejected:
			case <-time.After(5 * time.Second):
				t.Fatal("actual startup failure never reached rejected receipt")
			}
			if !tab.pending || completed != 0 || tab.state != "Starting shell…" {
				t.Fatal("failed worker updated UI before receipt")
			}
			if recover {
				allow.Store(true)
				saveAck(t, mailbox)()
				if failure == nil || errors.Is(failure, context.Canceled) || tab.name != "Failed" || tab.state == "" {
					t.Fatal("real preparation failure was lost", failure, tab.state)
				}
			} else {
				m.closeTerminal(tab, 0)
				if !errors.Is(failure, context.Canceled) {
					t.Fatal("closing failed startup did not complete cancellation", failure)
				}
			}
			select {
			case <-tab.workersDone:
			case <-time.After(2 * time.Second):
				t.Fatal("failed closed startup retained controller workers")
			}
			if len(m.terminals) != 0 || tab.session != nil || completed != 1 {
				t.Fatal("failed startup retained its UI slot or completed twice")
			}
		})
	}
}
