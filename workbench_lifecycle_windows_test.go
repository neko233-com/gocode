//go:build windows && cgo

package main

import (
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/neko233-com/godesktop/extensions"
	"github.com/neko233-com/godesktop/testing/winprobe"
	"golang.org/x/sys/windows"
)

func ownedWorkbenchChild(parent uint32, base string) uint32 {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if entry.ParentProcessID == parent && strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), base) {
			return entry.ProcessID
		}
	}
	return 0
}
func TestNativeWorkbenchNewWindowAndDisabledExtensionReload(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	restore := winprobe.Awareness()
	defer restore()
	root := t.TempDir()
	source := filepath.Join(root, "main.go")
	os.WriteFile(source, []byte("package main\nfunc main() {}\n"), 0600)
	extRoot := filepath.Join(root, "extensions")
	fixture := fixtureExtension(t, extRoot)
	exe := filepath.Join(t.TempDir(), "gocode-lifecycle.exe")
	if data, err := exec.Command("go", "build", "-race", "-o", exe, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, data)
	}
	var output safeOutput
	cmd := exec.Command(exe, "-workspace", root, "-extensions-dir", extRoot, "-copilot=false", "-lsp=false")
	cmd.Env = append(os.Environ(), "GODESKTOP_READBACK=1", "APPDATA="+t.TempDir())
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Every replacement, extension host and terminal belongs to this test's
	// kill-on-close job, including assertion failures before normal shutdown.
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		windows.CloseHandle(job)
		cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, process)
		windows.CloseHandle(process)
	}
	if err != nil {
		windows.CloseHandle(job)
		cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		windows.CloseHandle(job)
		if cmd.ProcessState == nil {
			_ = cmd.Wait()
		}
	})
	title := "gocode — " + filepath.Base(root)
	var first winprobe.Window
	until(t, func() bool {
		var err error
		first, err = ownedWorkbenchWindow(title, uint32(cmd.Process.Pid))
		return err == nil
	})
	until(t, func() bool { color, err := first.Pixel(600, 105); return err == nil && color == 0x282828 })
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		t.Logf("owned lifecycle output: %s", output.String())
		if pixels, err := first.Capture(); err == nil {
			os.MkdirAll(".cache/windows-workbench", 0700)
			if f, err := os.Create(".cache/windows-workbench/lifecycle-failure.png"); err == nil {
				_ = png.Encode(f, pixels)
				_ = f.Close()
			}
		}
	})
	key := func(w winprobe.Window, k int) {
		t.Helper()
		if err := w.Send(0x100, uintptr(k), 0); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	click := func(w winprobe.Window, x, y int) {
		t.Helper()
		if err := w.Pointer(0x201, x, y); err != nil {
			t.Fatal(err)
		}
		if err := w.Pointer(0x202, x, y); err != nil {
			t.Fatal(err)
		}
		time.Sleep(70 * time.Millisecond)
	}
	// Real pointer File > New Window starts an actual child EXE.
	click(first, 53, 18)
	click(first, 130, 75)
	var secondPID uint32
	until(t, func() bool {
		secondPID = ownedWorkbenchChild(uint32(cmd.Process.Pid), filepath.Base(exe))
		return secondPID != 0
	})
	secondProcess, err := os.FindProcess(int(secondPID))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { secondProcess.Kill() })
	var second winprobe.Window
	until(t, func() bool { var err error; second, err = ownedWorkbenchWindow(title, secondPID); return err == nil })
	until(t, func() bool { color, err := second.Pixel(600, 105); return err == nil && color == 0x282828 })
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	until(t, func() bool { _, err := ownedWorkbenchWindow(title, secondPID); return err != nil })
	// The actual installed VSIX is disabled through its native detail action.
	click(first, 24, 250)
	click(first, 130, 170)
	click(first, 470, 207)
	until(t, func() bool {
		state, err := readExtensionSettings(extRoot)
		return err == nil && containsExtension(state.Disabled, fixture.ID())
	})
	// Open View > Command Palette using real menu keyboard events, then invoke
	// Reload Window. No physical/global keyboard state is changed by the probe.
	for _, k := range []int{121, 39, 39, 39, 40, 13} {
		key(first, k)
	}
	for _, ch := range "Reload Window" {
		if err := first.Send(0x102, uintptr(ch), 0); err != nil {
			t.Fatal(err)
		}
	}
	key(first, 13)
	var replacementPID uint32
	until(t, func() bool {
		replacementPID = ownedWorkbenchChild(uint32(cmd.Process.Pid), filepath.Base(exe))
		return replacementPID != 0 && replacementPID != secondPID
	})
	replacementProcess, err := os.FindProcess(int(replacementPID))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { replacementProcess.Kill() })
	if err := cmd.Wait(); err != nil {
		t.Fatalf("old process did not close cleanly: %v %s", err, output.String())
	}
	var replacement winprobe.Window
	until(t, func() bool {
		var err error
		replacement, err = ownedWorkbenchWindow(title, replacementPID)
		return err == nil
	})
	until(t, func() bool { color, err := replacement.Pixel(600, 105); return err == nil && color == 0x282828 })
	state, err := readExtensionSettings(extRoot)
	if err != nil || len(enabledExtensions([]extensions.Extension{fixture}, state)) != 0 {
		t.Fatal("replacement did not preserve actual disabled state")
	}
	if body, err := os.ReadFile(source); err != nil || string(body) != "package main\nfunc main() {}\n" {
		t.Fatal("new window/reload changed source file")
	}
	// Actual uninstall is deferred until the old process/host have shut down.
	click(replacement, 24, 250)
	click(replacement, 130, 170)
	click(replacement, 535, 207)
	until(t, func() bool {
		state, err := readExtensionSettings(extRoot)
		return err == nil && containsExtension(state.Uninstall, fixture.ID())
	})
	for _, k := range []int{121, 39, 39, 39, 40, 13} {
		key(replacement, k)
	}
	for _, ch := range "Reload Window" {
		if err := replacement.Send(0x102, uintptr(ch), 0); err != nil {
			t.Fatal(err)
		}
	}
	key(replacement, 13)
	var thirdPID uint32
	until(t, func() bool { thirdPID = ownedWorkbenchChild(replacementPID, filepath.Base(exe)); return thirdPID != 0 })
	thirdProcess, err := os.FindProcess(int(thirdPID))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { thirdProcess.Kill() })
	until(t, func() bool { _, err := ownedWorkbenchWindow(title, replacementPID); return err != nil })
	var third winprobe.Window
	until(t, func() bool { var err error; third, err = ownedWorkbenchWindow(title, thirdPID); return err == nil })
	until(t, func() bool { color, err := third.Pixel(600, 105); return err == nil && color == 0x282828 })
	if _, err := os.Stat(fixture.Path); !os.IsNotExist(err) {
		t.Fatal("actual replacement process did not complete VSIX uninstall")
	}
	// Real File > Open Folder uses the OS picker, then starts the selected
	// workspace only after the old workbench exits. It retains the extension root.
	folder := filepath.Join(root, "opened-folder")
	os.Mkdir(folder, 0700)
	os.WriteFile(filepath.Join(folder, "main.go"), []byte("package main\n// selected folder\n"), 0600)
	for _, k := range []int{121, 40, 40, 40, 40, 13} {
		key(third, k)
	}
	if err := chooseOwnedWindowsDialogFor(root, folder, false, thirdPID); err != nil {
		t.Fatal(err)
	}
	var folderPID uint32
	until(t, func() bool { folderPID = ownedWorkbenchChild(thirdPID, filepath.Base(exe)); return folderPID != 0 })
	folderProcess, err := os.FindProcess(int(folderPID))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { folderProcess.Kill() })
	var folderWindow winprobe.Window
	until(t, func() bool {
		var err error
		folderWindow, err = ownedWorkbenchWindow("gocode — "+filepath.Base(folder), folderPID)
		return err == nil
	})
	until(t, func() bool { color, err := folderWindow.Pixel(600, 105); return err == nil && color == 0x282828 })
	if err := folderWindow.Close(); err != nil {
		t.Fatal(err)
	}
	until(t, func() bool {
		_, err := ownedWorkbenchWindow("gocode — "+filepath.Base(folder), folderPID)
		return err != nil
	})
}
