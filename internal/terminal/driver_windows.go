//go:build windows

package terminal

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows"
	"path/filepath"
	"sync"
	"unsafe"
)

type conPTYDriver struct {
	create, resize, close *windows.Proc
	dll                   *windows.DLL
}

var driverLock sync.Mutex
var loadedDriver *conPTYDriver

func loadConPTY(ctx context.Context) (*conPTYDriver, error) {
	path, err := EnsureConPTY(ctx, "")
	if err != nil {
		return nil, err
	}
	driverLock.Lock()
	defer driverLock.Unlock()
	if loadedDriver != nil {
		return loadedDriver, nil
	}
	name := filepath.Join(path, "conpty.dll")
	h, err := windows.LoadLibraryEx(name, 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
	if err != nil {
		return nil, err
	}
	dll := &windows.DLL{Name: name, Handle: h}
	success := false
	defer func() {
		if !success {
			dll.Release()
		}
	}()
	create, err := dll.FindProc("ConptyCreatePseudoConsole")
	if err != nil {
		return nil, err
	}
	resize, err := dll.FindProc("ConptyResizePseudoConsole")
	if err != nil {
		return nil, err
	}
	closeProc, err := dll.FindProc("ConptyClosePseudoConsole")
	if err != nil {
		return nil, err
	}
	loadedDriver = &conPTYDriver{create, resize, closeProc, dll}
	success = true
	return loadedDriver, nil
}
func packedSize(size Size) uintptr {
	return uintptr(uint32(uint16(size.Columns)) | uint32(uint16(size.Rows))<<16)
}
func conPTYResult(value uintptr) error {
	if uint32(value)&0x80000000 != 0 {
		return fmt.Errorf("ConPTY HRESULT 0x%08x", uint32(value))
	}
	return nil
}
func (d *conPTYDriver) Create(size Size, input, output windows.Handle, target *windows.Handle) error {
	value, _, _ := d.create.Call(packedSize(size), uintptr(input), uintptr(output), 0, uintptr(unsafe.Pointer(target)))
	return conPTYResult(value)
}
func (d *conPTYDriver) Resize(handle windows.Handle, size Size) error {
	value, _, _ := d.resize.Call(uintptr(handle), packedSize(size))
	return conPTYResult(value)
}
func (d *conPTYDriver) Close(handle windows.Handle) { d.close.Call(uintptr(handle)) }
