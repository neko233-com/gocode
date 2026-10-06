package update

import (
	"crypto/ed25519"
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/neko233-com/gocode/internal/installlayout"
)

// Cleanup removes only recognized signed update files and validated app pointers.
// MSI removes its own baseline files. Unknown files, workspaces and settings stay.
func Cleanup(root string, key ed25519.PublicKey) error {
	base, err := installlayout.ValidateRoot(root)
	if err != nil {
		return err
	}
	release, err := lock(root)
	if err != nil {
		return err
	}
	released := false
	defer func() {
		if !released {
			release()
		}
	}()
	versions := filepath.Join(root, "versions")
	if err := checkDirectory(versions); err != nil {
		return err
	}
	entries, err := os.ReadDir(versions)
	if err != nil {
		return err
	}
	var failures []error
	for _, entry := range entries {
		if !entry.IsDir() || !installlayout.ValidVersion(entry.Name()) || entry.Name() == base.Version {
			continue
		}
		manifest, err := StagedManifest(root, entry.Name(), key)
		if err != nil || manifest.Version != entry.Name() {
			continue
		}
		name := "gocode-app"
		names := []string{name}
		if runtime.GOOS == "windows" {
			names = []string{name + ".exe", name + "-gui.exe"}
		}
		for _, name := range append(names, "update-manifest.json") {
			file := filepath.Join(versions, entry.Name(), name)
			info, err := os.Lstat(file)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				failures = append(failures, err)
				continue
			}
			if !info.Mode().IsRegular() {
				continue
			}
			if err := os.Remove(file); err != nil {
				failures = append(failures, err)
			}
		}
		// A directory with any unrecognized user file is deliberately retained.
		_ = os.Remove(filepath.Join(versions, entry.Name()))
	}
	for _, name := range []string{"current.json", "previous.json"} {
		if _, err := installlayout.Read(filepath.Join(root, name)); err == nil {
			if err := os.Remove(filepath.Join(root, name)); err != nil {
				failures = append(failures, err)
			}
		}
	}
	release()
	released = true
	_ = os.Remove(filepath.Join(root, ".gocode-update.lock"))
	return errors.Join(failures...)
}
