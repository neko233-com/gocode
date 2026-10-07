//go:build darwin || linux

package git

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

type unixProcess struct {
	cmd  *exec.Cmd
	once sync.Once
}

func startProcess(path string, args []string, directory string, environment []string, in, out, errout *os.File) (process, error) {
	cmd := exec.Command(path, args...)
	cmd.Dir = directory
	cmd.Env = environment
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = errout
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &unixProcess{cmd: cmd}, nil
}
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
	var err error
	p.once.Do(func() {
		err = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			err = nil
		}
	})
	return err
}
