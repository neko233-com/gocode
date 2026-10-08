//go:build windows

package nativeguard

import (
	"errors"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Derived from this repository's internal/git ownership path: suspended create,
// explicit inherited-pipe handle list, job assignment, then ResumeThread. The
// child cannot create an unowned descendant in a post-Start assignment window.
type windowsProcess struct {
	process  *os.Process
	job      windows.Handle
	once     sync.Once
	closeErr error
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
	defer func() {
		for _, handle := range handles {
			_ = windows.SetHandleInformation(handle, windows.HANDLE_FLAG_INHERIT, 0)
		}
	}()
	for _, handle := range handles {
		if err = windows.SetHandleInformation(handle, windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT); err != nil {
			return nil, err
		}
	}
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
		return nil, errors.New("native guard command exceeds Windows command-line limit")
	}
	cwd, err := windows.UTF16PtrFromString(directory)
	if err != nil {
		return nil, err
	}
	block, err := environmentBlock(environment)
	if err != nil {
		return nil, err
	}
	startup := windows.StartupInfoEx{}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	startup.Flags = windows.STARTF_USESTDHANDLES
	startup.StdInput, startup.StdOutput, startup.StdErr = handles[0], handles[1], handles[2]
	startup.ProcThreadAttributeList = attrs.List()
	info := windows.ProcessInformation{}
	flags := uint32(windows.CREATE_UNICODE_ENVIRONMENT | windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_SUSPENDED | windows.CREATE_NO_WINDOW)
	if err = windows.CreateProcess(application, &command[0], nil, nil, true, flags, &block[0], cwd, &startup.StartupInfo, &info); err != nil {
		return nil, err
	}
	runtime.KeepAlive(handles)
	defer windows.CloseHandle(info.Thread)
	defer windows.CloseHandle(info.Process)
	// A failure before resume still owns a suspended process. Terminate and wait
	// on that exact handle before releasing it; do not leave a suspended child.
	failCreated := func(cause error) (process, error) {
		terminateErr := windows.TerminateProcess(info.Process, 1)
		wait, waitErr := windows.WaitForSingleObject(info.Process, 5000)
		if waitErr == nil && wait != windows.WAIT_OBJECT_0 {
			waitErr = errors.New("native guard could not reap suspended startup failure")
		}
		return nil, errors.Join(cause, terminateErr, waitErr)
	}
	if err = windows.AssignProcessToJobObject(job, info.Process); err != nil {
		return failCreated(err)
	}
	child, err := os.FindProcess(int(info.ProcessId))
	if err != nil {
		return failCreated(err)
	}
	if _, err = windows.ResumeThread(info.Thread); err != nil {
		_, failure = failCreated(err)
		_ = child.Release()
		return nil, failure
	}
	return &windowsProcess{process: child, job: job}, nil
}

func (p *windowsProcess) PID() int { return p.process.Pid }
func (p *windowsProcess) Wait() (int, error) {
	state, err := p.process.Wait()
	if err != nil {
		return -1, err
	}
	return state.ExitCode(), nil
}
func (p *windowsProcess) Close() error {
	p.once.Do(func() {
		p.closeErr = errors.Join(windows.TerminateJobObject(p.job, 1), windows.CloseHandle(p.job))
	})
	return p.closeErr
}

var compareStringOrdinal = windows.NewLazySystemDLL("kernel32.dll").NewProc("CompareStringOrdinal")

func compareEnvironmentNames(left, right []uint16) (int, error) {
	order, _, failure := compareStringOrdinal.Call(uintptr(unsafe.Pointer(&left[0])), uintptr(len(left)-1), uintptr(unsafe.Pointer(&right[0])), uintptr(len(right)-1), 1)
	runtime.KeepAlive(left)
	runtime.KeepAlive(right)
	if order == 0 {
		return 0, errors.Join(errors.New("CompareStringOrdinal could not order environment names"), failure)
	}
	return int(order) - 2, nil // CSTR_LESS_THAN/EQUAL/GREATER_THAN.
}

// CreateProcess requires Unicode, case-insensitive, locale-independent name
// order. Use the OS uppercase table rather than Go's versioned Unicode folding.
// Preserve drive-current-directory entries such as =C:=C:\\work verbatim.
// https://learn.microsoft.com/en-us/windows/win32/procthread/changing-environment-variables
func environmentBlock(environment []string) ([]uint16, error) {
	type entry struct{ name, value []uint16 }
	entries := make([]entry, 0, len(environment))
	for _, value := range environment {
		separator := strings.IndexByte(value, '=')
		if separator == 0 {
			separator = strings.IndexByte(value[1:], '=') + 1
		}
		if separator <= 0 || strings.ContainsRune(value, 0) {
			return nil, errors.New("invalid native guard environment")
		}
		name, err := windows.UTF16FromString(value[:separator])
		if err != nil {
			return nil, err
		}
		encoded, err := windows.UTF16FromString(value)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry{name, encoded})
	}
	var compareErr error
	sort.SliceStable(entries, func(i, j int) bool {
		order, err := compareEnvironmentNames(entries[i].name, entries[j].name)
		if err != nil {
			compareErr = err
		}
		return order < 0
	})
	if compareErr != nil {
		return nil, compareErr
	}
	var block []uint16
	for i := 0; i < len(entries); {
		end := i + 1
		for end < len(entries) {
			order, err := compareEnvironmentNames(entries[i].name, entries[end].name)
			if err != nil {
				return nil, err
			}
			if order != 0 {
				break
			}
			end++
		}
		// Stable sorting retains input order for equal names. Match exec.Cmd's
		// last-value rule so appended private TMP/APPDATA overrides take effect.
		block = append(block, entries[end-1].value...)
		i = end
	}
	block = append(block, 0)
	if len(block) == 1 {
		block = append(block, 0)
	}
	return block, nil
}
