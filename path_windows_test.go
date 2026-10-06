//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

func TestWindowsShortPathPreservesUnsavedDocumentIdentity(t *testing.T) {
	m := testModel(t)
	d := m.current()
	path, err := syscall.UTF16PtrFromString(d.path)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]uint16, 32768)
	n, _, callErr := syscall.NewLazyDLL("kernel32.dll").NewProc("GetShortPathNameW").Call(uintptr(unsafe.Pointer(path)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if n == 0 || n >= uintptr(len(buffer)) {
		t.Fatalf("short path: %v", callErr)
	}
	short := syscall.UTF16ToString(buffer[:n])
	if strings.EqualFold(short, d.path) {
		t.Skip("volume has no 8.3 alias for this fixture")
	}
	text(m, "// unsaved\n")
	version, source := d.buffer.Version(), d.buffer.Text()
	m.open(short)
	if m.current() != d || len(m.docs) != 1 || d.buffer.Version() != version || d.buffer.Text() != source {
		t.Fatalf("8.3 alias reopened unsaved source: %s => %s", short, d.path)
	}
	if m.findDocument(short) != d {
		t.Fatal("canonical server URI could not find the live document")
	}
	if filepath.Base(d.path) != "main.go" {
		t.Fatal("file identity changed")
	}
	t.Logf("verified short-path alias %s", short)
}
