//go:build darwin || linux

package nativeguard

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// Ownership follows internal/git's process-group design. No shell/global PID
// search is used; descendants that remain in this group receive its final kill.
type unixProcess struct {
	cmd      *exec.Cmd
	once     sync.Once
	closeErr error
}

func startProcess(path string, args []string, directory string, environment []string, in, out, errout *os.File) (process, error) {
	cmd := exec.Command(path, args...)
	cmd.Dir, cmd.Env = directory, environment
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, errout
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &unixProcess{cmd: cmd}, nil
}

func (p *unixProcess) PID() int { return p.cmd.Process.Pid }
func (p *unixProcess) Wait() (int, error) {
	err := p.cmd.Wait()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		err = nil
	}
	if p.cmd.ProcessState == nil {
		return -1, err
	}
	return p.cmd.ProcessState.ExitCode(), err
}
func (p *unixProcess) Close() error {
	p.once.Do(func() {
		p.closeErr = syscall.Kill(-p.PID(), syscall.SIGKILL)
		if p.closeErr == syscall.ESRCH {
			p.closeErr = nil
		}
	})
	return p.closeErr
}
