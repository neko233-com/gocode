//go:build windows

package filewatch

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// OpenRead permits atomic replacement while gocode observes a file. Go's
// ordinary os.Open shares read/write, but omits FILE_SHARE_DELETE on Windows.
func OpenRead(path string) (*os.File, error) {
	name, err := extendedPath(path)
	if err != nil {
		return nil, err
	}
	pointer, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(pointer, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_SEQUENTIAL_SCAN, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}
func extendedPath(path string) (string, error) {
	name, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	name = filepath.Clean(name)
	if !strings.HasPrefix(name, `\\?\`) {
		if strings.HasPrefix(name, `\\`) {
			name = `\\?\UNC\` + strings.TrimPrefix(name, `\\`)
		} else {
			name = `\\?\` + name
		}
	}
	return name, nil
}
func renameFile(source, target string) error {
	from, err := extendedPath(source)
	if err != nil {
		return err
	}
	to, err := extendedPath(target)
	if err != nil {
		return err
	}
	fromW, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	toW, err := windows.UTF16FromString(to)
	if err != nil {
		return err
	}
	// Respect unrelated readers which omit deletion sharing. The modern rename
	// then permits our deletion-sharing readers to retain their old descriptor
	// while the pathname atomically adopts the new file.
	targetHandle, targetErr := windows.CreateFile(&toW[0], windows.DELETE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if targetErr == nil {
		_ = windows.CloseHandle(targetHandle)
	} else if !errors.Is(targetErr, windows.ERROR_FILE_NOT_FOUND) && !errors.Is(targetErr, windows.ERROR_PATH_NOT_FOUND) {
		return &os.LinkError{Op: "rename", Old: source, New: target, Err: targetErr}
	}
	handle, err := windows.CreateFile(fromW, windows.DELETE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return &os.LinkError{Op: "rename", Old: source, New: target, Err: err}
	}
	type renameInfo struct {
		Flags  uint32
		Root   windows.Handle
		Length uint32
		Name   [1]uint16
	}
	var header renameInfo
	offset := int(unsafe.Offsetof(header.Name))
	// FileNameLength excludes NUL, but the Win32 API also requires the name
	// storage to be NUL terminated. Omitting it yielded intermittent invalid
	// path errors when native conversion read beyond an exact-size Go buffer.
	body := make([]byte, offset+2*len(toW))
	binary.LittleEndian.PutUint32(body, windows.FILE_RENAME_REPLACE_IF_EXISTS|windows.FILE_RENAME_POSIX_SEMANTICS)
	binary.LittleEndian.PutUint32(body[int(unsafe.Offsetof(header.Length)):], uint32(2*(len(toW)-1)))
	for i, c := range toW[:len(toW)-1] {
		binary.LittleEndian.PutUint16(body[offset+2*i:], c)
	}
	err = windows.SetFileInformationByHandle(handle, windows.FileRenameInfoEx, &body[0], uint32(len(body)))
	_ = windows.CloseHandle(handle)
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) || errors.Is(err, windows.ERROR_NOT_SUPPORTED) || errors.Is(err, windows.ERROR_INVALID_FUNCTION) {
		return os.Rename(source, target)
	}
	if err != nil {
		return &os.LinkError{Op: "rename", Old: source, New: target, Err: err}
	}
	return nil
}
func transientReplace(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_ACCESS_DENIED)
}
