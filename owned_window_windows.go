//go:build windows && cgo

package main

import (
	"fmt"
	"github.com/neko233-com/godesktop/testing/winprobe"
	"golang.org/x/sys/windows"
	"unsafe"
)

// Several workbench processes can show the same folder/title. FindWindow's
// first match is insufficient; enumerate same-title siblings and verify PID.
// FindWindowEx needs no per-call callback allocation and reads no other titles.
func ownedWorkbenchWindow(title string, pid uint32) (winprobe.Window, error) {
	name, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return 0, err
	}
	user := windows.NewLazySystemDLL("user32.dll")
	var previous uintptr
	for range 2048 {
		handle, _, _ := user.NewProc("FindWindowExW").Call(0, previous, 0, uintptr(unsafe.Pointer(name)))
		if handle == 0 {
			break
		}
		var owner uint32
		user.NewProc("GetWindowThreadProcessId").Call(handle, uintptr(unsafe.Pointer(&owner)))
		if owner == pid {
			return winprobe.Window(handle), nil
		}
		previous = handle
	}
	return 0, fmt.Errorf("owned workbench window for PID %d was not found", pid)
}
