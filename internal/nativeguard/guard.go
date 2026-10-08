// Package nativeguard supervises an owned acceptance process tree. It does not
// change the native fixture's cooperative work deadline or application APIs.
package nativeguard

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const MaxRuntime = 120 * time.Second
const MaxOutputBytes = 512 << 10 // Combined raw stdout and stderr.

var ErrOutputLimit = errors.New("native acceptance output exceeds 512 KiB")

type Options struct {
	Command     []string
	Directory   string
	Environment []string
	Timeout     time.Duration
}

type Report struct {
	Executable   string   `json:"executable"`
	Arguments    []string `json:"arguments"`
	PID          int      `json:"pid"`
	ExitCode     int      `json:"exit_code"`
	DeadlineMS   int64    `json:"deadline_ms"`
	ElapsedMS    int64    `json:"elapsed_ms"`
	TimedOut     bool     `json:"timed_out"`
	OutputLimit  bool     `json:"output_limit"`
	RootReaped   bool     `json:"root_reaped"`
	TreeClosed   bool     `json:"tree_closed"`
	StdoutBytes  int      `json:"stdout_bytes"`
	StderrBytes  int      `json:"stderr_bytes"`
	StdoutSHA256 string   `json:"stdout_sha256"`
	StderrSHA256 string   `json:"stderr_sha256"`
	Error        string   `json:"error,omitempty"`
}

type Result struct {
	Report Report
	Stdout []byte
	Stderr []byte
}

type process interface {
	PID() int
	Wait() (int, error)
	Close() error
}

// Both pipe readers share one budget. Each retained byte is counted once;
// overflow fails the invocation instead of permitting a truncated success.
type capture struct {
	mu       sync.Mutex
	stdout   []byte
	stderr   []byte
	overflow bool
}

func (c *capture) write(data []byte, stderr bool) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	remaining := MaxOutputBytes - len(c.stdout) - len(c.stderr)
	n := min(remaining, len(data))
	if stderr {
		c.stderr = append(c.stderr, data[:n]...)
	} else {
		c.stdout = append(c.stdout, data[:n]...)
	}
	if n < len(data) {
		c.overflow = true
		return n, ErrOutputLimit
	}
	return n, nil
}

type streamWriter struct {
	capture *capture
	stderr  bool
}

func (w streamWriter) Write(data []byte) (int, error) { return w.capture.write(data, w.stderr) }

// Run starts the tree with ownership established before the executable runs.
// The process ceiling covers startup, native execution and final fixture hash;
// afterwards it closes the owned tree and reaps the root before returning.
func Run(parent context.Context, options Options) (result Result, failure error) {
	result.Report.ExitCode = -1
	defer func() {
		if failure != nil {
			result.Report.Error = failure.Error()
		}
	}()
	if parent == nil || len(options.Command) == 0 || len(options.Command) > 128 || options.Timeout <= 0 || options.Timeout > MaxRuntime {
		return result, errors.New("invalid native guard command or deadline (maximum 120s)")
	}
	bytes := 0
	for _, arg := range options.Command {
		bytes += len(arg)
	}
	if bytes > 32<<10 {
		return result, errors.New("native guard command exceeds 32 KiB")
	}
	started := time.Now()
	defer func() {
		result.Report.ElapsedMS = time.Since(started).Milliseconds()
	}()
	ctx, cancel := context.WithTimeout(parent, options.Timeout)
	defer cancel()
	result.Report.DeadlineMS = options.Timeout.Milliseconds()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	executable, err := exec.LookPath(options.Command[0])
	if err != nil {
		return result, err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return result, err
	}
	result.Report.Executable = executable
	result.Report.Arguments = append([]string(nil), options.Command[1:]...)
	directory := options.Directory
	if directory == "" {
		directory, err = os.Getwd()
	} else {
		directory, err = filepath.Abs(directory)
	}
	if err != nil {
		return result, err
	}
	environment := options.Environment
	if options.Environment == nil {
		environment = os.Environ()
	}
	if len(environment) > 4096 {
		return result, errors.New("native guard environment exceeds 4096 entries")
	}
	remainingEnvironmentBytes := 1 << 20
	for _, entry := range environment {
		if len(entry)+1 > remainingEnvironmentBytes {
			return result, errors.New("native guard environment exceeds 1 MiB")
		}
		remainingEnvironmentBytes -= len(entry) + 1
	}
	environment = append([]string(nil), environment...)
	in, err := os.Open(os.DevNull)
	if err != nil {
		return result, err
	}
	defer in.Close()
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		return result, err
	}
	defer outRead.Close()
	defer outWrite.Close()
	errRead, errWrite, err := os.Pipe()
	if err != nil {
		return result, err
	}
	defer errRead.Close()
	defer errWrite.Close()
	child, err := startProcess(executable, options.Command[1:], directory, environment, in, outWrite, errWrite)
	if err != nil {
		return result, err
	}
	result.Report.PID = child.PID()
	_ = outWrite.Close()
	_ = errWrite.Close()
	defer child.Close()
	// Join cancellation explicitly; no delayed timer can close another invocation.
	finished, cancellationDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(cancellationDone)
		select {
		case <-ctx.Done():
			_ = child.Close()
		case <-finished:
		}
	}()
	output := &capture{}
	var workers sync.WaitGroup
	readErrors := make(chan error, 2)
	for _, stream := range []struct {
		file   *os.File
		stderr bool
	}{{outRead, false}, {errRead, true}} {
		workers.Go(func() {
			defer stream.file.Close()
			_, err := io.Copy(streamWriter{output, stream.stderr}, stream.file)
			readErrors <- err
			if err != nil {
				_ = child.Close()
			}
		})
	}
	code, waitErr := child.Wait()
	result.Report.ExitCode, result.Report.RootReaped = code, waitErr == nil
	closeErr := child.Close()
	result.Report.TreeClosed = closeErr == nil
	close(finished)
	<-cancellationDone
	workersDone := make(chan struct{})
	go func() { workers.Wait(); close(workersDone) }()
	select {
	case <-workersDone:
	case <-time.After(time.Second):
		// A Unix descendant that deliberately leaves the owned group cannot keep
		// inherited pipe readers blocked. This is not a sandbox for hostile code.
		_ = outRead.Close()
		_ = errRead.Close()
		<-workersDone
	}
	result.Stdout, result.Stderr = output.stdout, output.stderr
	result.Report.StdoutBytes, result.Report.StderrBytes = len(result.Stdout), len(result.Stderr)
	result.Report.StdoutSHA256 = fmt.Sprintf("%x", sha256.Sum256(result.Stdout))
	result.Report.StderrSHA256 = fmt.Sprintf("%x", sha256.Sum256(result.Stderr))
	result.Report.OutputLimit = output.overflow
	var copyErr error
	for range 2 {
		copyErr = errors.Join(copyErr, <-readErrors)
	}
	if err := ctx.Err(); err != nil {
		result.Report.TimedOut = errors.Is(err, context.DeadlineExceeded)
		return result, errors.Join(err, waitErr, closeErr)
	}
	if output.overflow {
		return result, errors.Join(ErrOutputLimit, waitErr, closeErr)
	}
	if err := errors.Join(waitErr, closeErr, copyErr); err != nil {
		return result, err
	}
	if code != 0 {
		return result, fmt.Errorf("native acceptance exited with code %d", code)
	}
	return result, nil
}
