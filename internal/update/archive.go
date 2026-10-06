package update

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/neko233-com/gocode/internal/installlayout"
)

func checkDirectory(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("update directory is not an owned regular directory")
	}
	return nil
}

func verifyFileHash(ctx context.Context, name string, a Asset) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() != a.Size {
		return errors.New("archive size mismatch")
	}
	hash := sha256.New()
	if _, err := copyContext(ctx, hash, f, a.Size); err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), a.SHA256) {
		return errors.New("archive SHA-256 verification failed")
	}
	return nil
}

// Stage verifies the complete archive before extracting only the versioned app.
// Existing versions and the fixed launcher are never overwritten by an update.
func Stage(ctx context.Context, root, archivePath string, envelope []byte, key ed25519.PublicKey, platform string) (installlayout.Manifest, error) {
	var result installlayout.Manifest
	if _, err := installlayout.ValidateRoot(root); err != nil {
		return result, err
	}
	m, err := Verify(envelope, key)
	if err != nil {
		return result, err
	}
	a, err := m.Asset(platform)
	if err != nil {
		return result, err
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return result, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return result, err
	}
	if info.Size() != a.Size {
		return result, errors.New("archive size mismatch")
	}
	hash := sha256.New()
	if _, err := copyContext(ctx, hash, f, a.Size); err != nil {
		return result, err
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), a.SHA256) {
		return result, errors.New("archive SHA-256 verification failed")
	}
	z, err := zip.NewReader(f, a.Size)
	if err != nil {
		return result, err
	}
	if len(z.File) > 1000 {
		return result, errors.New("too many archive entries")
	}
	names := []string{"gocode-app"}
	if platform == "windows/amd64" {
		names = []string{"gocode-app.exe", "gocode-app-gui.exe"}
	}
	wanted := map[string]string{}
	for _, name := range names {
		wanted["versions/"+m.Version+"/"+name] = name
	}
	rootFiles := map[string]bool{"gocode.exe": true, "gocode-launch.exe": true, "gocode-maintenance.exe": true, "gocode": true, "gocode-launch": true, "base.json": true, "LICENSE.txt": true, "README.md": true, "CODE-OSS-LICENSE.txt": true, "gocode.ico": true}
	seen := map[string]bool{}
	var expanded uint64
	for _, entry := range z.File {
		name := strings.TrimSuffix(entry.Name, "/")
		if name == "" || path.Clean(name) != name || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") || name == ".." || strings.HasPrefix(name, "../") || entry.Mode()&os.ModeSymlink != 0 {
			return result, errors.New("unsafe archive path or symbolic link")
		}
		if seen[name] {
			return result, errors.New("duplicate archive entry")
		}
		seen[name] = true
		if entry.FileInfo().IsDir() {
			if name != "versions" && name != "versions/"+m.Version {
				return result, errors.New("unexpected archive directory")
			}
			continue
		}
		if _, ok := wanted[name]; !ok && !rootFiles[name] {
			return result, errors.New("unexpected archive payload")
		}
		if entry.UncompressedSize64 > MaxExpandedBytes || expanded > MaxExpandedBytes-entry.UncompressedSize64 {
			return result, errors.New("expanded update exceeds size policy")
		}
		expanded += entry.UncompressedSize64
	}
	for name := range wanted {
		if !seen[name] {
			return result, errors.New("required application variant missing")
		}
	}
	versions := filepath.Join(root, "versions")
	if err := checkDirectory(versions); err != nil {
		return result, err
	}
	final := filepath.Join(versions, m.Version)
	if _, err := os.Lstat(final); !errors.Is(err, os.ErrNotExist) {
		return result, errors.New("update version already exists; no payload was replaced")
	}
	stage, err := os.MkdirTemp(versions, ".gocode-stage-")
	if err != nil {
		return result, err
	}
	committed := false
	defer func() {
		if !committed { // stage is a freshly created direct child of the validated versions directory.
			for _, name := range append(names, "update-manifest.json") {
				_ = os.Remove(filepath.Join(stage, name))
			}
			_ = os.Remove(stage)
		}
	}()
	for _, entry := range z.File {
		name, ok := wanted[entry.Name]
		if !ok {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			return result, err
		}
		out, err := os.OpenFile(filepath.Join(stage, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
		if err != nil {
			reader.Close()
			return result, err
		}
		n, copyErr := copyContext(ctx, out, reader, int64(entry.UncompressedSize64)+1)
		err = errors.Join(copyErr, reader.Close(), out.Sync(), out.Close())
		if err != nil {
			return result, err
		}
		if n != int64(entry.UncompressedSize64) {
			return result, errors.New("expanded application length mismatch")
		}
	}
	if err := os.WriteFile(filepath.Join(stage, "update-manifest.json"), envelope, 0600); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := os.Rename(stage, final); err != nil {
		return result, err
	}
	committed = true
	return installlayout.Manifest{Owner: m.Owner, Schema: m.Layout, Version: m.Version, Source: m.Source}, nil
}

func copyContext(ctx context.Context, out io.Writer, in io.Reader, limit int64) (int64, error) {
	reader := io.LimitReader(in, limit)
	buf := make([]byte, 128<<10)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := reader.Read(buf)
		if n > 0 {
			written, writeErr := out.Write(buf[:n])
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
			if written != n {
				return total, io.ErrShortWrite
			}
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, io.ErrNoProgress
		}
	}
}
