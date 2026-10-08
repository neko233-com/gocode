//go:build windows || darwin || linux

package languageserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/neko233-com/godesktop/lsp"
)

// Console-only processes exercise real inherited stdout and descendant lifetime.
// They do not replace the separate actual gopls/TypeScript or native UI gates.
func TestOwnedLanguageProcessHelper(t *testing.T) {
	index := -1
	for i, argument := range os.Args {
		if argument == "--owned-language-process" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	args := os.Args[index+1:]
	if len(args) < 2 {
		os.Exit(2)
	}
	if err := os.WriteFile(args[1], []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		os.Exit(3)
	}
	if args[0] == "hold" {
		for {
			time.Sleep(time.Second)
		}
	}
	executable, _ := os.Executable()
	child := exec.Command(executable, "-test.run=^TestOwnedLanguageProcessHelper$", "--", "--owned-language-process", "hold", args[2])
	child.Stdout, child.Stderr = os.Stdout, os.Stderr
	if err := child.Start(); err != nil {
		os.Exit(4)
	}
	data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "fixture/ready", "params": map[string]int{"root": os.Getpid(), "child": child.Process.Pid}})
	fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(data), data)
	fmt.Fprintln(os.Stderr, "real server diagnostics 世界😀 are drained without retention")
	if args[0] == "exit-tree" {
		// The descendant retains stdout after the root exits. Waiting for stdout
		// EOF before reaping the root would leave this tree alive indefinitely.
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	_ = child.Wait()
	os.Exit(0)
}

func languageProcessCommand(t *testing.T, mode string, markers ...string) lsp.Command {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return lsp.Command{Executable: executable, Arguments: append([]string{"-test.run=^TestOwnedLanguageProcessHelper$", "--", "--owned-language-process", mode}, markers...)}
}

func languageProcessPID(t *testing.T, marker string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(marker); err == nil {
			pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if pid > 0 {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("real language process did not publish its private PID", marker)
	return 0
}

func TestOwnedLanguageTransportClosesTreeAndPreservesSibling(t *testing.T) {
	root := t.TempDir()
	siblingSpec := languageProcessCommand(t, "hold", filepath.Join(root, "unrelated.pid"))
	sibling := exec.Command(siblingSpec.Executable, siblingSpec.Arguments...)
	configureLanguageSibling(sibling)
	if err := sibling.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sibling.Process.Kill(); _ = sibling.Wait() })
	siblingPID := languageProcessPID(t, filepath.Join(root, "unrelated.pid"))
	assertSibling := observeLanguageProcess(t, siblingPID)
	for _, mode := range []string{"close", "cancel", "exit-tree"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			rootMarker, childMarker := filepath.Join(directory, "root.pid"), filepath.Join(directory, "child 世界😀.pid")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			helperMode := "tree"
			if mode == "exit-tree" {
				helperMode = mode
			}
			rpc, owner, err := startOwned(ctx, languageProcessCommand(t, helperMode, rootMarker, childMarker))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = rpc.Close(); _ = owner.Close() })
			rootPID, childPID := languageProcessPID(t, rootMarker), languageProcessPID(t, childMarker)
			if rootPID != owner.child.PID() {
				t.Fatal("transport does not own the observed root PID")
			}
			assertRoot, assertChild := observeLanguageProcess(t, rootPID), observeLanguageProcess(t, childPID)
			if runtime.GOOS == "windows" {
				identifiers, err := (&Client{RPC: rpc, process: owner}).ProcessIDs()
				if err != nil || !slices.Contains(identifiers, rootPID) || !slices.Contains(identifiers, childPID) || slices.Contains(identifiers, siblingPID) {
					t.Fatal("actual job observation missed descendants or captured the unrelated sibling", identifiers, err)
				}
			}
			select {
			case notification := <-rpc.Notifications():
				if notification.Method != "fixture/ready" {
					t.Fatal("real Content-Length notification not received", notification.Method)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("real helper protocol never became ready")
			}
			started := time.Now()
			if mode == "cancel" {
				callDone := make(chan error, 1)
				go func() { callDone <- rpc.Call(t.Context(), "fixture/held", nil, nil) }()
				cancel()
				select {
				case err := <-callDone:
					if err == nil {
						t.Fatal("cancelled server falsely completed its unanswered request")
					}
				case <-time.After(3 * time.Second):
					t.Fatal("parent cancellation retained a pending protocol request")
				}
			} else if mode == "close" {
				if err := rpc.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				select {
				case <-owner.closed:
				case <-time.After(3 * time.Second):
					t.Fatal("root exit did not close the descendant holding stdout")
				}
			}
			if err := owner.Close(); err != nil {
				t.Fatal("owned root/worker closure was not acknowledged", err)
			}
			if time.Since(started) > 3*time.Second {
				t.Fatal("actual transport close exceeded its three-second bound")
			}
			assertRoot(false)
			assertChild(false)
			assertSibling(true)
			client := &Client{RPC: rpc, process: owner}
			if !client.ProcessClosed() {
				t.Fatal("closed process/reader/context workers were not all acknowledged")
			}
			var repeated sync.WaitGroup
			for range 16 {
				repeated.Go(func() {
					if err := client.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			repeated.Wait()
			t.Logf("actual root%d/descendant%d reaped; sibling%d survives; %s close %.3fs", rootPID, childPID, siblingPID, mode, time.Since(started).Seconds())
		})
	}
}

func TestOwnedLanguageTransportRejectsBeforeProcessCreation(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "must-not-start.pid")
	command := languageProcessCommand(t, "hold", marker)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, owner, err := startOwned(ctx, command); err == nil || owner != nil {
		t.Fatal("already cancelled lifetime started a process")
	}
	if _, owner, err := startOwned(nil, command); err == nil || owner != nil {
		t.Fatal("missing lifetime started a process")
	}
	command.Arguments = append(command.Arguments, strings.Repeat("x", 32<<10))
	if _, owner, err := startOwned(context.Background(), command); err == nil || owner != nil {
		t.Fatal("oversized command started a process")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("rejected process actually ran", err)
	}
}
