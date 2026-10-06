package update

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/neko233-com/gocode/internal/installlayout"
)

type boundedOutput struct {
	bytes.Buffer
	overflow bool
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	n := len(p)
	available := (64 << 10) - b.Len()
	if n > available {
		b.overflow = true
		p = p[:max(0, available)]
	}
	_, err := b.Buffer.Write(p)
	return n, err
}

func Probe(ctx context.Context, root string, manifest installlayout.Manifest) error {
	name := "gocode-app"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	command := filepath.Join(root, "versions", manifest.Version, name)
	c, stop := context.WithTimeout(ctx, 5*time.Second)
	defer stop()
	process := exec.CommandContext(c, command, "-version")
	process.Dir = root
	hideProcess(process)
	var output boundedOutput
	process.Stdout = &output
	process.Stderr = &output
	if err := process.Run(); err != nil {
		return fmt.Errorf("staged version health check failed: %w", err)
	}
	fields := strings.Fields(output.String())
	if output.overflow || len(fields) != 4 || fields[0] != "gocode" || fields[1] != manifest.Version || fields[2] != runtime.GOOS+"/"+runtime.GOARCH || fields[3] != manifest.Source {
		return errors.New("staged executable reported unexpected version/platform/source")
	}
	return nil
}

// Apply only changes next-launch selection after integrity and executable checks.
// Current windows/documents keep their existing process and unsaved state.
func (m *Manager) Apply(ctx context.Context, root, platform string) (Result, error) {
	if _, err := installlayout.ValidateRoot(root); err != nil {
		return Result{}, err
	}
	release, err := lock(root)
	if err != nil {
		return Result{}, fmt.Errorf("another update is active: %w", err)
	}
	defer release()
	result, err := m.Prepare(ctx, root, platform)
	if err != nil || !result.Ready {
		return result, err
	}
	manifest, err := StagedManifest(root, result.Version, m.Key)
	if err != nil {
		return result, err
	}
	if err := Probe(ctx, root, manifest); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := installlayout.Activate(root, manifest); err != nil {
		return result, err
	}
	return result, nil
}
func Rollback(ctx context.Context, root string, key ed25519.PublicKey) error {
	base, err := installlayout.ValidateRoot(root)
	if err != nil {
		return err
	}
	release, err := lock(root)
	if err != nil {
		return err
	}
	defer release()
	previous, err := installlayout.Read(filepath.Join(root, "previous.json"))
	if err != nil {
		return err
	}
	if previous.Version != base.Version {
		verified, err := StagedManifest(root, previous.Version, key)
		if err != nil {
			return err
		}
		if verified.Source != previous.Source {
			return errors.New("rollback source does not match signed metadata")
		}
	}
	if err := Probe(ctx, root, previous); err != nil {
		return err
	}
	return installlayout.Activate(root, previous)
}

func InstalledRoot(executable string) (string, error) {
	dir := filepath.Dir(executable)
	if filepath.Base(filepath.Dir(dir)) != "versions" || !installlayout.ValidVersion(filepath.Base(dir)) {
		return "", errors.New("automatic updates require an installed or extracted versioned package")
	}
	root := filepath.Dir(filepath.Dir(dir))
	_, err := installlayout.ValidateRoot(root)
	return root, err
}

func WriteConfig(path string, config Config) error {
	if len(config.ManifestURLs) == 0 {
		config.ManifestURLs = DefaultConfig().ManifestURLs
	}
	key, err := PublisherKey()
	if err != nil {
		return err
	}
	m := Manager{Key: key, Config: config}
	if err := m.validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := jsonMarshal(config)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".gocode-updates-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	_, writeErr := f.Write(data)
	err = errors.Join(writeErr, f.Sync(), f.Close())
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
