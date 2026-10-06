//go:build windows

package update

import (
	"path/filepath"
	"syscall"
)

// Windows share mode zero releases the update lock even if a process crashes.
func lock(root string) (func(), error) {
	path, err := syscall.UTF16PtrFromString(filepath.Join(root, ".gocode-update.lock"))
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(path, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return func() { _ = syscall.CloseHandle(handle) }, nil
}
