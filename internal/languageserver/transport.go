package languageserver

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/neko233-com/godesktop/lsp"
)

type process interface {
	PID() int
	Wait() (int, error)
	Close() error
}

type serverTransport struct {
	child                       process
	done, watchDone, stderrDone chan struct{}
	closed                      chan struct{}
	stdin, stdout, stderr       *os.File
	mu                          sync.Mutex
	err                         error
	closeOnce, streamsOnce      sync.Once
	closeErr                    error
}

// Actual process ownership is established before its first instruction. Unlike
// an acceptance guard this service has no fixed runtime ceiling; context/Close
// kills its job/group, closes streams and reaps its root with a bounded join.
func startOwned(ctx context.Context, command lsp.Command) (*lsp.Client, *serverTransport, error) {
	if ctx == nil {
		return nil, nil, errors.New("language server context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if command.Executable == "" || len(command.Arguments) > 128 {
		return nil, nil, errors.New("invalid language server command")
	}
	commandBytes := len(command.Executable)
	for _, argument := range command.Arguments {
		commandBytes += len(argument) + 1
	}
	if commandBytes > 32<<10 {
		return nil, nil, errors.New("language server command exceeds 32 KiB")
	}
	path, err := exec.LookPath(command.Executable)
	if err != nil {
		return nil, nil, err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	directory := command.Directory
	if directory == "" {
		directory, err = os.Getwd()
	} else {
		directory, err = filepath.Abs(directory)
	}
	if err != nil {
		return nil, nil, err
	}
	environment := command.Environment
	if environment == nil {
		environment = os.Environ()
	}
	if len(environment) > 4096 {
		return nil, nil, errors.New("language server environment exceeds limit")
	}
	var size int
	for _, value := range environment {
		size += len(value) + 1
	}
	if size > 1<<20 {
		return nil, nil, errors.New("language server environment exceeds limit")
	}
	inRead, inWrite, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		inRead.Close()
		inWrite.Close()
		return nil, nil, err
	}
	errRead, errWrite, err := os.Pipe()
	if err != nil {
		inRead.Close()
		inWrite.Close()
		outRead.Close()
		outWrite.Close()
		return nil, nil, err
	}
	child, err := startServerProcess(path, command.Arguments, directory, append([]string(nil), environment...), inRead, outWrite, errWrite)
	inRead.Close()
	outWrite.Close()
	errWrite.Close()
	if err != nil {
		inWrite.Close()
		outRead.Close()
		errRead.Close()
		return nil, nil, err
	}
	owner := &serverTransport{child: child, done: make(chan struct{}), watchDone: make(chan struct{}), stderrDone: make(chan struct{}), closed: make(chan struct{}), stdin: inWrite, stdout: outRead, stderr: errRead}
	rpc := lsp.Connect(outRead, inWrite, func() { _ = owner.Close() })
	go func() { defer close(owner.stderrDone); _, _ = io.Copy(io.Discard, errRead) }()
	go func() {
		_, err := child.Wait()
		err = errors.Join(err, child.Close())
		owner.mu.Lock()
		owner.err = err
		owner.mu.Unlock()
		owner.closeStreams()
		close(owner.done)
		_ = rpc.Close()
	}()
	go func() {
		defer close(owner.watchDone)
		select {
		case <-ctx.Done():
			_ = child.Close()
			owner.closeStreams()
		case <-owner.done:
		}
	}()
	return rpc, owner, nil
}

func (t *serverTransport) closeStreams() {
	t.streamsOnce.Do(func() { t.stdin.Close(); t.stdout.Close(); t.stderr.Close() })
}
func (t *serverTransport) Close() error {
	t.closeOnce.Do(func() {
		defer close(t.closed)
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		err := t.child.Close()
		t.closeStreams()
		for _, done := range []<-chan struct{}{t.done, t.watchDone, t.stderrDone} {
			select {
			case <-done:
			case <-timer.C:
				t.closeErr = errors.Join(err, errors.New("owned language server shutdown exceeded three seconds"))
				return
			}
		}
		t.mu.Lock()
		t.closeErr = errors.Join(err, t.err)
		t.mu.Unlock()
	})
	return t.closeErr
}
