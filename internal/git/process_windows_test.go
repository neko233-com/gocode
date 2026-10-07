//go:build windows

package git

import (
	"golang.org/x/sys/windows"
	"testing"
)

func TestOwnedWindowsCancellationKillsDescendant(t *testing.T) {
	cancelOwnedTree(t, func(pid int) func() {
		handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = windows.CloseHandle(handle) })
		return func() {
			result, err := windows.WaitForSingleObject(handle, 1000)
			if err != nil || result != windows.WAIT_OBJECT_0 {
				t.Fatalf("owned descendant survived job cancellation: %v %d", err, result)
			}
		}
	})
}
