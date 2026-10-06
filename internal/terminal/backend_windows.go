//go:build windows

package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

type processTerminal struct {
	input, output *os.File
	console, job  windows.Handle
	process       *os.Process
	once          sync.Once
	endOnce       sync.Once
	consoleMu     sync.Mutex
	closeErr      error
	driver        *conPTYDriver
}

func startBackend(ctx context.Context, config Config, size Size) (_ *processTerminal, failure error) {
	driver, err := loadConPTY(ctx)
	if err != nil {
		return nil, err
	}
	t := &processTerminal{driver: driver}
	inputRead, inputWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer inputRead.Close()
	outputRead, outputWrite, err := os.Pipe()
	if err != nil {
		inputWrite.Close()
		return nil, err
	}
	defer outputWrite.Close()
	t.input, t.output = inputWrite, outputRead
	defer func() {
		if failure != nil {
			go io.Copy(io.Discard, t.output)
			t.Close()
		}
	}()
	if err := driver.Create(size, windows.Handle(inputRead.Fd()), windows.Handle(outputWrite.Fd()), &t.console); err != nil {
		return nil, fmt.Errorf("ConPTY: %w", err)
	}
	t.job, err = windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(t.job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return nil, err
	}
	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, err
	}
	defer attrs.Delete()
	// HPCON is itself an opaque pointer, not a pointer to the handle variable.
	// Reinterpret the opaque native pointer bits without uintptr-to-Go-pointer
	// arithmetic. UpdateProcThreadAttribute expects the HPCON value itself.
	consolePointer := *(*unsafe.Pointer)(unsafe.Pointer(&t.console))
	if err := attrs.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, consolePointer, unsafe.Sizeof(t.console)); err != nil {
		return nil, err
	}
	path, err := exec.LookPath(config.Command[0])
	if err != nil {
		return nil, err
	}
	application, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	arguments := append([]string{path}, config.Command[1:]...)
	command, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(arguments))
	if err != nil {
		return nil, err
	}
	directory, err := windows.UTF16PtrFromString(config.Directory)
	if err != nil {
		return nil, err
	}
	environment, err := environmentBlock(config.Environment)
	if err != nil {
		return nil, err
	}
	startup := windows.StartupInfoEx{}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	startup.Flags, startup.ShowWindow = windows.STARTF_USESHOWWINDOW|windows.STARTF_USESTDHANDLES, windows.SW_HIDE
	startup.ProcThreadAttributeList = attrs.List()
	info := windows.ProcessInformation{}
	flags := uint32(windows.CREATE_UNICODE_ENVIRONMENT | windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_SUSPENDED)
	if err := windows.CreateProcess(application, command, nil, nil, false, flags, &environment[0], directory, &startup.StartupInfo, &info); err != nil {
		return nil, err
	}
	defer windows.CloseHandle(info.Thread)
	defer windows.CloseHandle(info.Process)
	// Assign while suspended so commands cannot escape cleanup by spawning early.
	if err := windows.AssignProcessToJobObject(t.job, info.Process); err != nil {
		windows.TerminateProcess(info.Process, 1)
		return nil, err
	}
	t.process, err = os.FindProcess(int(info.ProcessId))
	if err != nil {
		windows.TerminateProcess(info.Process, 1)
		return nil, err
	}
	if _, err := windows.ResumeThread(info.Thread); err != nil {
		t.process.Kill()
		t.process.Release()
		return nil, err
	}
	return t, nil
}
func isTerminalEOF(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) || errors.Is(err, windows.ERROR_BROKEN_PIPE)
}

func environmentBlock(entries []string) ([]uint16, error) {
	values := map[string]string{}
	for _, entry := range entries {
		if strings.ContainsRune(entry, 0) {
			return nil, errors.New("invalid terminal environment")
		}
		separator := strings.IndexByte(entry, '=')
		if separator == 0 {
			separator = strings.IndexByte(entry[1:], '=') + 1
		}
		if separator <= 0 {
			return nil, errors.New("invalid terminal environment entry")
		}
		values[strings.ToUpper(entry[:separator])] = entry
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var result []uint16
	for _, key := range keys {
		value, err := windows.UTF16FromString(values[key])
		if err != nil {
			return nil, err
		}
		result = append(result, value...)
	}
	result = append(result, 0)
	if len(result) == 1 {
		result = append(result, 0)
	}
	if len(result) > 1<<20 {
		return nil, errors.New("terminal environment exceeds limit")
	}
	return result, nil
}
func (t *processTerminal) Read(b []byte) (int, error)  { return t.output.Read(b) }
func (t *processTerminal) Write(b []byte) (int, error) { return t.input.Write(b) }
func (t *processTerminal) Resize(size Size) error {
	t.consoleMu.Lock()
	defer t.consoleMu.Unlock()
	if t.console == 0 {
		return os.ErrClosed
	}
	return t.driver.Resize(t.console, size)
}
func (t *processTerminal) Wait() (int, error) {
	state, err := t.process.Wait()
	if err != nil {
		return -1, err
	}
	return state.ExitCode(), nil
}
func (t *processTerminal) PID() int { return t.process.Pid }
func (t *processTerminal) End() {
	t.endOnce.Do(func() {
		if t.job != 0 {
			t.closeErr = windows.CloseHandle(t.job)
		}
		if t.input != nil {
			t.closeErr = errors.Join(t.closeErr, t.input.Close())
		}
		// The output reader remains alive during ClosePseudoConsole's final frame.
		t.consoleMu.Lock()
		if t.console != 0 {
			t.driver.Close(t.console)
			t.console = 0
		}
		t.consoleMu.Unlock()
	})
}
func (t *processTerminal) Close() error {
	t.End()
	t.once.Do(func() {
		if t.output != nil {
			t.closeErr = errors.Join(t.closeErr, t.output.Close())
		}
	})
	return t.closeErr
}
