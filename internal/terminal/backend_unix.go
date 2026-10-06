//go:build darwin || linux

package terminal

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/creack/pty"
)

type processTerminal struct {
	master   *os.File
	command  *exec.Cmd
	once     sync.Once
	closeErr error
	exited   atomic.Bool
}

func startBackend(config Config, size Size) (*processTerminal, error) {
	master, slave, err := pty.Open()
	if err != nil {
		return nil, err
	}
	defer slave.Close()
	if err := pty.Setsize(master, &pty.Winsize{Cols: uint16(size.Columns), Rows: uint16(size.Rows)}); err != nil {
		master.Close()
		return nil, err
	}
	command := exec.Command(config.Command[0], config.Command[1:]...)
	command.Dir, command.Env = config.Directory, config.Environment
	command.Stdin, command.Stdout, command.Stderr = slave, slave, slave
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := command.Start(); err != nil {
		master.Close()
		return nil, err
	}
	return &processTerminal{master: master, command: command}, nil
}
func (t *processTerminal) Read(b []byte) (int, error)  { return t.master.Read(b) }
func (t *processTerminal) Write(b []byte) (int, error) { return t.master.Write(b) }
func (t *processTerminal) Resize(size Size) error {
	return pty.Setsize(t.master, &pty.Winsize{Cols: uint16(size.Columns), Rows: uint16(size.Rows)})
}
func (t *processTerminal) Wait() (int, error) {
	err := t.command.Wait()
	t.exited.Store(true)
	if t.command.ProcessState == nil {
		return -1, err
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		err = nil
	}
	return t.command.ProcessState.ExitCode(), err
}
func (t *processTerminal) PID() int { return t.command.Process.Pid }
func (t *processTerminal) End()     {}
func (t *processTerminal) Close() error {
	t.once.Do(func() {
		if !t.exited.Load() {
			t.closeErr = killTerminalTree(t.command.Process.Pid)
		}
		t.closeErr = errors.Join(t.closeErr, t.master.Close())
	})
	return t.closeErr
}
func isTerminalEOF(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) || errors.Is(err, syscall.EIO)
}
