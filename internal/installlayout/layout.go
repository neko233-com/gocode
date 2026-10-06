// Package installlayout defines the owned versioned installation shared by the
// stable launcher, portable installer and updater. User data lives elsewhere.
package installlayout

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const Owner = "neko233-com/gocode"
const Schema = 1

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[a-zA-Z0-9][a-zA-Z0-9.-]*)?$`)

type Manifest struct {
	Owner   string `json:"owner"`
	Schema  int    `json:"schema"`
	Version string `json:"version"`
	Source  string `json:"source"`
}

func ValidVersion(version string) bool {
	if len(version) > 80 || !versionPattern.MatchString(version) {
		return false
	}
	_, pre, hasPre := strings.Cut(version, "-")
	if hasPre {
		for _, part := range strings.Split(pre, ".") {
			if part == "" {
				return false
			}
			if numeric(part) && len(part) > 1 && part[0] == '0' {
				return false
			}
		}
	}
	return true
}
func Read(path string) (Manifest, error) {
	var result Manifest
	info, err := os.Lstat(path)
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return result, errors.New("manifest must be a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil {
		return result, err
	}
	if len(data) > 65536 {
		return result, errors.New("installation manifest is too large")
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	if result.Owner != Owner || result.Schema != Schema || !ValidVersion(result.Version) {
		return result, errors.New("unrecognized gocode installation manifest")
	}
	return result, nil
}
func ValidateRoot(root string) (Manifest, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Manifest{}, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return Manifest{}, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return Manifest{}, errors.New("installation root must be an owned directory, without a symlink or junction")
	}
	// Windows 8.3 aliases are legitimate identities, not reparse points. Check
	// actual ancestors instead of comparing the spelling returned by EvalSymlinks.
	if runtime.GOOS == "windows" {
		for dir := root; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
			info, err := os.Lstat(dir)
			if err != nil {
				return Manifest{}, err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return Manifest{}, errors.New("installation root must not traverse a symlink or junction")
			}
		}
	}
	return Read(filepath.Join(root, "base.json"))
}
func Resolve(root string, gui bool) (string, Manifest, error) {
	base, err := ValidateRoot(root)
	if err != nil {
		return "", base, err
	}
	active, err := Read(filepath.Join(root, "current.json"))
	if errors.Is(err, os.ErrNotExist) {
		active = base
	} else if err != nil {
		return "", base, err
	}
	// An MSI upgrade can remove its previous baseline payload. An older mutable
	// update pointer must never make the new installer launch a removed version.
	if CompareVersion(base.Version, active.Version) > 0 {
		active = base
	}
	if err := validatePayloadDirectory(root, active.Version); err != nil {
		return "", active, err
	}
	name := "gocode-app"
	if runtime.GOOS == "windows" {
		if gui {
			name += "-gui"
		}
		name += ".exe"
	}
	path := filepath.Join(root, "versions", active.Version, name)
	info, err := os.Lstat(path)
	if err != nil {
		return "", active, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", active, errors.New("application payload must be a regular file")
	}
	return path, active, nil
}
func validatePayloadDirectory(root, version string) error {
	for _, dir := range []string{filepath.Join(root, "versions"), filepath.Join(root, "versions", version)} {
		info, err := os.Lstat(dir)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("payload directory must not be a symlink or junction")
		}
	}
	return nil
}

// Activate only changes the small pointer after both payload variants exist.
// Replacing an app version never overwrites a running executable on Windows.
func Activate(root string, manifest Manifest) error {
	base, err := ValidateRoot(root)
	if err != nil {
		return err
	}
	if manifest.Owner != Owner || manifest.Schema != Schema || !ValidVersion(manifest.Version) {
		return errors.New("invalid activation manifest")
	}
	if err := validatePayloadDirectory(root, manifest.Version); err != nil {
		return err
	}
	for _, name := range []string{"gocode-app", "gocode-app-gui"} {
		if runtime.GOOS == "windows" {
			name += ".exe"
		} else if name == "gocode-app-gui" {
			continue
		}
		path := filepath.Join(root, "versions", manifest.Version, name)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("invalid staged application")
		}
	}
	previous, err := Read(filepath.Join(root, "current.json"))
	if errors.Is(err, os.ErrNotExist) {
		previous, err = Read(filepath.Join(root, "base.json"))
	}
	if err != nil {
		return err
	}
	if CompareVersion(base.Version, previous.Version) > 0 {
		previous = base
	}
	if err := writePointer(root, "previous.json", previous); err != nil {
		return err
	}
	return writePointer(root, "current.json", manifest)
}

func writePointer(root, name string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(root, ".gocode-activate-")
	if err != nil {
		return err
	}
	temporary := f.Name()
	defer os.Remove(temporary)
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	return os.Rename(temporary, filepath.Join(root, name))
}
