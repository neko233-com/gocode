package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// This is a real child executable, isolated from the user's Git configuration.
func TestOwnedCommandHelper(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == "--gocode-git-helper" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	args := os.Args[index+1:]
	switch args[0] {
	case "echo":
		_, _ = io.Copy(os.Stdout, os.Stdin)
	case "stderr":
		_, _ = fmt.Fprint(os.Stderr, strings.Repeat("diagnostic", 20000))
	case "tree":
		executable, _ := os.Executable()
		child := exec.Command(executable, "-test.run=^TestOwnedCommandHelper$", "--", "--gocode-git-helper", "child", args[1])
		child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(3)
		}
		_ = child.Wait()
	case "child":
		_ = os.WriteFile(args[1], []byte(strconv.Itoa(os.Getpid())), 0600)
		for {
			time.Sleep(time.Second)
		}
	}
	os.Exit(0)
}

func helperCommand(t *testing.T, mode string, args ...string) []string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return append([]string{executable, "-test.run=^TestOwnedCommandHelper$", "--", "--gocode-git-helper", mode}, args...)
}

func TestOwnedRunnerStdinAndStderrBound(t *testing.T) {
	input := []byte(strings.Repeat("世界😀\r\n", 8192))
	output, err := runCommand(context.Background(), t.TempDir(), helperCommand(t, "echo"), input, len(input))
	if err != nil || string(output) != string(input) {
		t.Fatalf("Unicode stdin: %v, %d bytes", err, len(output))
	}
	if _, err := runCommand(context.Background(), t.TempDir(), helperCommand(t, "stderr"), nil, 1024); !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("stderr bound: %v", err)
	}
}

func cancelOwnedTree(t *testing.T, acquire func(int) func()) {
	t.Helper()
	directory := t.TempDir()
	marker := filepath.Join(directory, "owned-child.pid")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	command := helperCommand(t, "tree", marker)
	go func() { _, err := runCommand(ctx, directory, command, nil, 1024); done <- err }()
	deadline := time.Now().Add(5 * time.Second)
	var pid int
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(marker)
		if err == nil {
			pid, _ = strconv.Atoi(string(data))
			if pid > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("owned descendant never started")
	}
	check := acquire(pid)
	started := time.Now()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation retained descendant pipes")
	}
	check()
	if time.Since(started) > 3*time.Second {
		t.Fatal("owned cancellation exceeded bound")
	}
}
