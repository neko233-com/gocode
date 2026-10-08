package nativeguard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// These are real console-only processes, not a native-window success substitute.
func TestGuardProcessHelper(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == "--nativeguard-helper" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	args := os.Args[index+1:]
	switch args[0] {
	case "success":
		fmt.Fprintln(os.Stdout, "actual private console 世界😀", strings.Join(args[1:], "|"))
		fmt.Fprintln(os.Stderr, "actual diagnostic")
	case "nonzero":
		fmt.Fprintln(os.Stderr, "actual exit7")
		os.Exit(7)
	case "environment":
		for _, name := range args[1:] {
			fmt.Fprintf(os.Stdout, "%s=%s\n", name, os.Getenv(name))
		}
	case "overflow":
		fmt.Fprint(os.Stdout, strings.Repeat("o", 400<<10))
		fmt.Fprint(os.Stderr, strings.Repeat("e", 256<<10))
	case "tree":
		executable, _ := os.Executable()
		child := exec.Command(executable, "-test.run=^TestGuardProcessHelper$", "--", "--nativeguard-helper", "hold", args[1])
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(3)
		}
		_ = child.Wait()
	case "hold":
		if err := os.WriteFile(args[1], []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			os.Exit(4)
		}
		for {
			time.Sleep(time.Second)
		}
	default:
		os.Exit(5)
	}
	os.Exit(0)
}

func helperCommand(t *testing.T, mode string, args ...string) []string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return append([]string{executable, "-test.run=^TestGuardProcessHelper$", "--", "--nativeguard-helper", mode}, args...)
}

func TestGuardActualSuccessExitAndOutputBounds(t *testing.T) {
	directory := t.TempDir()
	result, err := Run(context.Background(), Options{Command: helperCommand(t, "success", "space arg", "界😀"), Directory: directory, Timeout: 5 * time.Second})
	if err != nil || !result.Report.RootReaped || !result.Report.TreeClosed || result.Report.PID <= 0 || result.Report.ExitCode != 0 || !strings.Contains(string(result.Stdout), "space arg|界😀") || !strings.Contains(string(result.Stderr), "actual diagnostic") {
		t.Fatalf("actual successful process: %+v %v", result.Report, err)
	}
	result, err = Run(context.Background(), Options{Command: helperCommand(t, "nonzero"), Directory: directory, Timeout: 5 * time.Second})
	if err == nil || result.Report.ExitCode != 7 || !result.Report.RootReaped || !result.Report.TreeClosed || !strings.Contains(string(result.Stderr), "actual exit7") {
		t.Fatalf("actual nonzero process: %+v %v", result.Report, err)
	}
	result, err = Run(context.Background(), Options{Command: helperCommand(t, "overflow"), Directory: directory, Timeout: 5 * time.Second})
	if !errors.Is(err, ErrOutputLimit) || !result.Report.OutputLimit || len(result.Stdout)+len(result.Stderr) != MaxOutputBytes || !result.Report.RootReaped || !result.Report.TreeClosed {
		t.Fatalf("actual combined pipe limit: %+v %v", result.Report, err)
	}
}

func TestGuardActualHeldProcessDeadline(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "held.pid")
	result, err := Run(context.Background(), Options{Command: helperCommand(t, "hold", marker), Timeout: 700 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) || !result.Report.TimedOut || !result.Report.RootReaped || !result.Report.TreeClosed || result.Report.ElapsedMS < 700 || result.Report.ElapsedMS > 3700 {
		t.Fatalf("actual held process deadline: %+v %v", result.Report, err)
	}
	data, readErr := os.ReadFile(marker)
	if readErr != nil || strings.TrimSpace(string(data)) != strconv.Itoa(result.Report.PID) {
		t.Fatalf("real held process did not start: %q %v", data, readErr)
	}
}

func TestGuardDeadlineClosesDescendantAndPreservesSibling(t *testing.T) {
	directory := t.TempDir()
	siblingMarker := filepath.Join(directory, "sibling.pid")
	siblingCommand := helperCommand(t, "hold", siblingMarker)
	sibling := exec.Command(siblingCommand[0], siblingCommand[1:]...)
	configureSibling(sibling)
	if err := sibling.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sibling.Process.Kill(); _ = sibling.Wait() })
	siblingPID := waitPID(t, siblingMarker)
	assertSiblingAlive := observeProcess(t, siblingPID)
	descendantMarker := filepath.Join(directory, "descendant.pid")
	type outcome struct {
		result Result
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := Run(context.Background(), Options{Command: helperCommand(t, "tree", descendantMarker), Directory: directory, Timeout: 3 * time.Second})
		done <- outcome{result, err}
	}()
	descendantPID := waitPID(t, descendantMarker)
	assertDescendantDead := observeProcess(t, descendantPID)
	select {
	case finished := <-done:
		if !errors.Is(finished.err, context.DeadlineExceeded) || !finished.result.Report.TimedOut || !finished.result.Report.RootReaped || !finished.result.Report.TreeClosed {
			t.Fatalf("owned tree deadline: %+v %v", finished.result.Report, finished.err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("held descendant retained owned pipes/process")
	}
	assertDescendantDead(false)
	assertSiblingAlive(true)
}

func waitPID(t *testing.T, marker string) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(marker); err == nil {
			pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if pid > 0 {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("actual private process never published its PID")
	return 0
}

func TestGuardRejectsUnboundedDeadlineBeforeStarting(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "must-not-start.pid")
	for _, timeout := range []time.Duration{0, -time.Second, MaxRuntime + time.Nanosecond} {
		result, err := Run(context.Background(), Options{Command: helperCommand(t, "hold", marker), Timeout: timeout})
		if err == nil || result.Report.PID != 0 || result.Report.RootReaped || result.Report.Error == "" {
			t.Fatalf("invalid deadline admitted a process: %+v %v", result.Report, err)
		}
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid deadline started the actual helper")
	}
	result, err := Run(context.Background(), Options{Command: helperCommand(t, "hold", marker), Timeout: time.Second, Environment: []string{"oversized=" + strings.Repeat("x", 1<<20)}})
	if err == nil || result.Report.PID != 0 || result.Report.Error == "" {
		t.Fatal("oversized environment admitted the actual helper")
	}
}
