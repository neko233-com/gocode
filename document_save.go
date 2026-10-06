package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	textbuffer "github.com/neko233-com/godesktop/editor"
)

// Refuse to overwrite a file changed by another program since open/last save.
// Large files never reach this writer; reads/writes remain bounded and cancellable.
func checkDiskVersion(ctx context.Context, path string, expected *[32]byte) error {
	if expected == nil {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > editableFileLimit {
		return errors.New("file changed on disk; reopen it before saving")
	}
	hash := sha256.New()
	block := make([]byte, 128<<10)
	total := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := f.Read(block)
		if n > 0 {
			total += n
			if total > editableFileLimit {
				return errors.New("file grew beyond the editable save policy")
			}
			_, _ = hash.Write(block[:n])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	var actual [32]byte
	copy(actual[:], hash.Sum(nil))
	if actual != *expected {
		return errors.New("file changed on disk; your unsaved buffer was preserved")
	}
	return nil
}

func writeDocumentSnapshot(ctx context.Context, path string, snapshot textbuffer.Snapshot, expected *[32]byte) ([32]byte, error) {
	var hash [32]byte
	if err := ctx.Err(); err != nil {
		return hash, err
	}
	if err := checkDiskVersion(ctx, path, expected); err != nil {
		return hash, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return hash, err
	}
	text := snapshot.Text()
	if len(text) > editableFileLimit {
		return hash, errors.New("editable snapshot exceeds the bounded save policy")
	}
	hash = sha256.Sum256([]byte(text))
	f, err := os.CreateTemp(filepath.Dir(path), ".gocode-save-")
	if err != nil {
		return hash, err
	}
	name := f.Name()
	defer os.Remove(name)
	reader := strings.NewReader(text)
	block := make([]byte, 128<<10)
	for {
		if err := ctx.Err(); err != nil {
			f.Close()
			return hash, err
		}
		n, readErr := reader.Read(block)
		if n > 0 {
			written, writeErr := f.Write(block[:n])
			if writeErr != nil || written != n {
				f.Close()
				return hash, fmt.Errorf("save write: %w", errors.Join(writeErr, io.ErrShortWrite))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			return hash, readErr
		}
	}
	if err := errors.Join(f.Sync(), f.Close()); err != nil {
		return hash, err
	}
	if err := os.Chmod(name, info.Mode().Perm()); err != nil {
		return hash, err
	}
	if err := checkDiskVersion(ctx, path, expected); err != nil {
		return hash, err
	}
	if err := ctx.Err(); err != nil {
		return hash, err
	}
	return hash, os.Rename(name, path)
}
