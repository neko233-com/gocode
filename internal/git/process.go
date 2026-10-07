// Package git runs bounded, cancellable Git operations on background workers.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var ErrOutputLimit = errors.New("Git output exceeds the operation limit")

type commandError struct {
	Code    int
	Message string
}

func (e *commandError) Error() string {
	return fmt.Sprintf("Git exited with code %d: %s", e.Code, e.Message)
}

type process interface {
	Wait() (int, error)
	Close() error
}

type limitedOutput struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *limitedOutput) Write(data []byte) (int, error) {
	if len(data) > b.limit-b.Len() {
		b.overflow = true
		_, _ = b.Buffer.Write(data[:max(0, b.limit-b.Len())])
		return 0, ErrOutputLimit
	}
	return b.Buffer.Write(data)
}

func gitEnvironment() []string {
	result := make([]string, 0, len(os.Environ())+4)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		if key == "GIT_DIR" || key == "GIT_WORK_TREE" || key == "GIT_INDEX_FILE" || key == "GIT_COMMON_DIR" || key == "GIT_OBJECT_DIRECTORY" || key == "GIT_ALTERNATE_OBJECT_DIRECTORIES" || key == "GIT_CONFIG" || key == "GIT_CONFIG_COUNT" || key == "GIT_CONFIG_PARAMETERS" || strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_") || key == "GIT_OPTIONAL_LOCKS" || key == "GIT_TERMINAL_PROMPT" || key == "LC_ALL" {
			continue
		}
		result = append(result, entry)
	}
	return append(result, "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
}

func runCommand(parent context.Context, directory string, argv []string, input []byte, limit int) ([]byte, error) {
	if parent == nil || len(argv) == 0 || len(argv) > 256 || len(input) > 1<<20 || limit < 1 || limit > 16<<20 {
		return nil, errors.New("invalid Git operation")
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	executable, err := exec.LookPath(argv[0])
	if err != nil {
		return nil, err
	}
	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer stdinRead.Close()
	defer stdinWrite.Close()
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer stdoutRead.Close()
	defer stdoutWrite.Close()
	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer stderrRead.Close()
	defer stderrWrite.Close()
	child, err := startProcess(executable, argv[1:], directory, gitEnvironment(), stdinRead, stdoutWrite, stderrWrite)
	if err != nil {
		return nil, err
	}
	defer child.Close()
	_ = stdinRead.Close()
	_ = stdoutWrite.Close()
	_ = stderrWrite.Close()
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-ctx.Done():
			_ = child.Close()
		case <-finished:
		}
	}()
	var workers sync.WaitGroup
	output := &limitedOutput{limit: limit}
	diagnostic := &limitedOutput{limit: 64 << 10}
	for _, item := range []struct {
		reader *os.File
		target *limitedOutput
	}{{stdoutRead, output}, {stderrRead, diagnostic}} {
		workers.Go(func() {
			defer item.reader.Close()
			// Do not expose bytes.Buffer.ReadFrom: it bypasses our bounded Write.
			if _, err := io.Copy(struct{ io.Writer }{item.target}, item.reader); err != nil {
				_ = child.Close()
			}
		})
	}
	workers.Go(func() { defer stdinWrite.Close(); _, _ = io.Copy(stdinWrite, bytes.NewReader(input)) })
	code, waitErr := child.Wait()
	_ = child.Close()
	// Closing readers also unblocks any inherited pipe held by detached Unix jobs.
	workersDone := make(chan struct{})
	go func() { workers.Wait(); close(workersDone) }()
	select {
	case <-workersDone:
	case <-time.After(time.Second):
		_ = stdoutRead.Close()
		_ = stderrRead.Close()
		_ = stdinWrite.Close()
		<-workersDone
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if output.overflow || diagnostic.overflow {
		return nil, ErrOutputLimit
	}
	if waitErr != nil {
		return nil, waitErr
	}
	if code != 0 {
		return output.Bytes(), &commandError{Code: code, Message: strings.TrimSpace(diagnostic.String())}
	}
	return output.Bytes(), nil
}

func runGit(ctx context.Context, root string, input []byte, limit int, args ...string) ([]byte, error) {
	argv := []string{"git", "--literal-pathspecs", "--no-optional-locks", "-c", "core.quotepath=false", "-c", "core.fsmonitor=false", "-C", root}
	return runCommand(ctx, root, append(argv, args...), input, limit)
}
