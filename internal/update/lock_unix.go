//go:build !windows

package update

import (
	"os"
	"path/filepath"
	"syscall"
)

func lock(root string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(root, ".gocode-update.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, err
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
