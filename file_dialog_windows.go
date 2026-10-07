//go:build windows && cgo

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The Windows shell common item dialog runs on its own STA. Its verified owner
// is this process's workbench HWND. No filesystem work or modal pump runs on the
// native rendering thread. Cancellation posts WM_CLOSE only to this STA's dialog.
func nativeChooseFile(ctx context.Context, workspace, kind, initial string) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	w, err := ownedWorkbenchWindow("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
	if err != nil {
		return "", err
	}
	ole := windows.NewLazySystemDLL("ole32.dll")
	hr, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2)
	if int32(hr) < 0 {
		return "", fmt.Errorf("CoInitializeEx: 0x%08x", uint32(hr))
	}
	defer ole.NewProc("CoUninitialize").Call()
	clsid := "{DC1C5A9C-E88A-4DDE-A5A1-60F82A20AEF7}"
	iid := "{D57C7288-D4AD-4768-BE02-9D969532D960}"
	if kind == "save" {
		clsid = "{C0B4E2F3-BA21-4773-8DBA-335EC946EB8B}"
		iid = "{84BCCD23-5FDE-4CDB-AEA4-AF64B83D78AB}"
	}
	class, _ := windows.GUIDFromString(clsid)
	iface, _ := windows.GUIDFromString(iid)
	var dialog *shellCOM
	hr, _, _ = ole.NewProc("CoCreateInstance").Call(uintptr(unsafe.Pointer(&class)), 0, 1, uintptr(unsafe.Pointer(&iface)), uintptr(unsafe.Pointer(&dialog)))
	if int32(hr) < 0 {
		return "", fmt.Errorf("create file dialog: 0x%08x", uint32(hr))
	}
	defer comDialogCall(dialog, 2)
	flags := uintptr(0x40 | 0x800 | 0x8 | 0x2000000) // FORCEFILESYSTEM, PATHMUSTEXIST, NOCHANGEDIR, DONTADDTORECENT
	if kind == "folder" {
		flags |= 0x20
	} else if kind == "save" {
		flags |= 0x2
	} else {
		flags |= 0x1000
	}
	if err := comDialogError(dialog, 9, flags); err != nil {
		return "", err
	}
	if kind == "save" {
		name, _ := windows.UTF16PtrFromString(filepath.Base(initial))
		if err := comDialogError(dialog, 15, uintptr(unsafe.Pointer(name))); err != nil {
			return "", err
		}
	}
	if kind == "vsix" {
		name, _ := windows.UTF16PtrFromString("VS Code Extension (*.vsix)")
		pattern, _ := windows.UTF16PtrFromString("*.vsix")
		filter := struct{ Name, Pattern *uint16 }{name, pattern}
		if err := comDialogError(dialog, 4, 1, uintptr(unsafe.Pointer(&filter))); err != nil {
			return "", err
		}
	}
	folder := initial
	if info, err := os.Stat(initial); err != nil || !info.IsDir() {
		folder = filepath.Dir(initial)
	}
	if path, _ := windows.UTF16PtrFromString(folder); path != nil {
		shellIID, _ := windows.GUIDFromString("{43826D1E-E718-42EE-BC55-A1E261C37BFE}")
		var item *shellCOM
		hr, _, _ = windows.NewLazySystemDLL("shell32.dll").NewProc("SHCreateItemFromParsingName").Call(uintptr(unsafe.Pointer(path)), 0, uintptr(unsafe.Pointer(&shellIID)), uintptr(unsafe.Pointer(&item)))
		if int32(hr) >= 0 && item != nil {
			comDialogCall(dialog, 12, uintptr(unsafe.Pointer(item)))
			comDialogCall(item, 2)
		}
	}
	thread, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetCurrentThreadId").Call()
	finished := make(chan struct{})
	monitorDone := make(chan struct{})
	defer func() { close(finished); <-monitorDone }()
	user := windows.NewLazySystemDLL("user32.dll")
	dialogClass, _ := windows.UTF16PtrFromString("#32770")
	go func() {
		defer close(monitorDone)
		select {
		case <-finished:
			return
		case <-ctx.Done():
		}
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			var previous uintptr
			for range 256 {
				hwnd, _, _ := user.NewProc("FindWindowExW").Call(0, previous, uintptr(unsafe.Pointer(dialogClass)), 0)
				if hwnd == 0 {
					break
				}
				var pid uint32
				ownerThread, _, _ := user.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&pid)))
				if ownerThread == thread && pid == uint32(os.Getpid()) {
					user.NewProc("PostMessageW").Call(hwnd, 0x10, 0, 0)
				}
				previous = hwnd
			}
			select {
			case <-finished:
				return
			case <-ticker.C:
			}
		}
	}()
	hr, _, _ = comDialogCall(dialog, 3, uintptr(w))
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if uint32(hr) == 0x800704c7 {
		return "", errDialogCancelled
	}
	if int32(hr) < 0 {
		return "", fmt.Errorf("file dialog: 0x%08x", uint32(hr))
	}
	var result *shellCOM
	if err := comDialogError(dialog, 20, uintptr(unsafe.Pointer(&result))); err != nil {
		return "", err
	}
	defer comDialogCall(result, 2)
	var name *uint16
	if err := comDialogError(result, 5, 0x80058000, uintptr(unsafe.Pointer(&name))); err != nil {
		return "", err
	}
	defer ole.NewProc("CoTaskMemFree").Call(uintptr(unsafe.Pointer(name)))
	return windows.UTF16PtrToString(name), nil
}

type shellCOM struct{ vtable *[27]uintptr }

// Preserve the syscall compiler's escape/pinning contract through forwarding.
//
//go:uintptrescapes
func comDialogCall(object *shellCOM, index int, args ...uintptr) (uintptr, uintptr, syscall.Errno) {
	return syscall.SyscallN(object.vtable[index], append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)...)
}

//go:uintptrescapes
func comDialogError(object *shellCOM, index int, args ...uintptr) error {
	hr, _, _ := comDialogCall(object, index, args...)
	if int32(hr) < 0 {
		return fmt.Errorf("file dialog method %d: 0x%08x", index, uint32(hr))
	}
	return nil
}
