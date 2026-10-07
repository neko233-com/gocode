//go:build windows

package git

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unsafe"
)

type windowsProcess struct {
	process *os.Process
	job     windows.Handle
	once    sync.Once
}

func startProcess(path string, args []string, directory string, environment []string, in, out, errout *os.File) (_ process, failure error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if failure != nil {
			_ = windows.CloseHandle(job)
		}
	}()
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return nil, err
	}
	handles := []windows.Handle{windows.Handle(in.Fd()), windows.Handle(out.Fd()), windows.Handle(errout.Fd())}
	for _, handle := range handles {
		if err = windows.SetHandleInformation(handle, windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT); err != nil {
			return nil, err
		}
	}
	defer func() {
		for _, handle := range handles {
			_ = windows.SetHandleInformation(handle, windows.HANDLE_FLAG_INHERIT, 0)
		}
	}()
	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, err
	}
	defer attrs.Delete()
	if err = attrs.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&handles[0]), uintptr(len(handles))*unsafe.Sizeof(handles[0])); err != nil {
		return nil, err
	}
	application, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	command, err := windows.UTF16FromString(windows.ComposeCommandLine(append([]string{path}, args...)))
	if err != nil || len(command) > 32767 {
		return nil, errors.New("Git command line exceeds Windows limit")
	}
	cwd, err := windows.UTF16PtrFromString(directory)
	if err != nil {
		return nil, err
	}
	sort.Strings(environment)
	var block []uint16
	for _, entry := range environment {
		if strings.ContainsRune(entry, 0) {
			return nil, errors.New("invalid process environment")
		}
		value, err := windows.UTF16FromString(entry)
		if err != nil {
			return nil, err
		}
		block = append(block, value...)
	}
	block = append(block, 0)
	if len(block) == 1 {
		block = append(block, 0)
	}
	startup := windows.StartupInfoEx{}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	startup.Flags = windows.STARTF_USESTDHANDLES
	startup.StdInput = handles[0]
	startup.StdOutput = handles[1]
	startup.StdErr = handles[2]
	startup.ProcThreadAttributeList = attrs.List()
	info := windows.ProcessInformation{}
	flags := uint32(windows.CREATE_UNICODE_ENVIRONMENT | windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_SUSPENDED | windows.CREATE_NO_WINDOW)
	if err = windows.CreateProcess(application, &command[0], nil, nil, true, flags, &block[0], cwd, &startup.StartupInfo, &info); err != nil {
		return nil, err
	}
	runtime.KeepAlive(handles)
	defer windows.CloseHandle(info.Thread)
	defer windows.CloseHandle(info.Process)
	if err = windows.AssignProcessToJobObject(job, info.Process); err != nil {
		_ = windows.TerminateProcess(info.Process, 1)
		return nil, err
	}
	child, err := os.FindProcess(int(info.ProcessId))
	if err != nil {
		_ = windows.TerminateProcess(info.Process, 1)
		return nil, err
	}
	if _, err = windows.ResumeThread(info.Thread); err != nil {
		_ = child.Kill()
		_ = child.Release()
		return nil, err
	}
	return &windowsProcess{process: child, job: job}, nil
}
func (p *windowsProcess) Wait() (int, error) {
	state, err := p.process.Wait()
	if err != nil {
		return -1, err
	}
	return state.ExitCode(), nil
}
func (p *windowsProcess) Close() error {
	var err error
	p.once.Do(func() { _ = windows.TerminateJobObject(p.job, 1); err = windows.CloseHandle(p.job) })
	return err
}
