package localrelease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const MaxReceiptBytes = 512 << 10

type File struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

func HashFile(ctx context.Context, path string, limit int64) (File, error) {
	var result File
	info, err := os.Lstat(path)
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return result, errors.New("release input must be a bounded regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer file.Close()
	hash := sha256.New()
	buffer := make([]byte, 64<<10)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		n, readErr := file.Read(buffer)
		total += int64(n)
		if total > limit {
			return result, errors.New("release input grew beyond its bound")
		}
		_, _ = hash.Write(buffer[:n])
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return result, readErr
		}
	}
	if total != info.Size() {
		return result, errors.New("release input changed during hashing")
	}
	return File{Path: path, Bytes: total, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func VerifyFile(ctx context.Context, base string, expected File, limit int64) error {
	if expected.Path == "" || filepath.IsAbs(expected.Path) {
		return errors.New("receipt paths must be relative to their exact release root")
	}
	path, err := Within(base, expected.Path)
	if err != nil {
		return err
	}
	actual, err := HashFile(ctx, path, limit)
	if err != nil {
		return err
	}
	if actual.Bytes != expected.Bytes || actual.SHA256 != expected.SHA256 {
		return fmt.Errorf("tested release bytes changed: %s", expected.Path)
	}
	return nil
}

func Within(root, relative string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if relative == "" || filepath.IsAbs(relative) {
		return "", errors.New("expected a relative owned path")
	}
	path := filepath.Join(root, relative)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("release path escaped the exact owned root")
	}
	cursor := path
	for {
		if info, err := os.Lstat(cursor); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return "", errors.New("release path contains a symlink")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(cursor)
		if parent == cursor {
			break
		}
		cursor = parent
	}
	return path, nil
}
