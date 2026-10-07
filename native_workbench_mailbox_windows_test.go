//go:build windows && cgo

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// These four functions are also copied from their parsed declarations into the
// private build overlay. Tests and the child observer execute the same helpers.
func nativeWorkbenchMailboxOpen(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}

func nativeWorkbenchMailboxRead(path string) ([]byte, error) {
	file, err := nativeWorkbenchMailboxOpen(path)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, 4097))
	err = errors.Join(readErr, file.Close())
	if err != nil {
		return nil, err
	}
	if len(data) > 4096 {
		return nil, errors.New("native geometry mailbox exceeds 4096 bytes")
	}
	return data, nil
}

func nativeWorkbenchMailboxTransient(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_ACCESS_DENIED)
}

func nativeWorkbenchMailboxReplace(ctx context.Context, path string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(data) > 4096 {
		return errors.New("native geometry mailbox exceeds 4096 bytes")
	}
	pending := path + ".pending"
	if err := os.WriteFile(pending, data, 0600); err != nil {
		return err
	}
	defer os.Remove(pending)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := os.Rename(pending, path)
		if err == nil || !nativeWorkbenchMailboxTransient(err) {
			return err
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func TestNativeWorkbenchMailboxShareDeletePreservesAtomicVersions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "request.json")
	const old = "old immutable request 世界"
	const next = "new immutable request 😀"
	if err := os.WriteFile(path, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	// Negative control: the original os.Open denies delete sharing while held.
	locked, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer locked.Close()
	if err = os.WriteFile(path+".pending", []byte(next), 0600); err != nil {
		t.Fatal(err)
	}
	err = os.Rename(path+".pending", path)
	if !nativeWorkbenchMailboxTransient(err) {
		t.Fatal("old reader did not reproduce Windows rename contention", err)
	}
	if err = locked.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := nativeWorkbenchMailboxOpen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	// MoveFileEx replacement can still reject an open destination even when
	// FILE_SHARE_DELETE is present. Do not assume sharing alone is sufficient:
	// the bounded writer must wait until this short owned reader closes.
	err = os.Rename(path+".pending", path)
	if err != nil && !nativeWorkbenchMailboxTransient(err) {
		t.Fatal("unexpected Windows replacement error", err)
	}
	var replacement chan error
	if err != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		replacement = make(chan error, 1)
		go func() { replacement <- nativeWorkbenchMailboxReplace(ctx, path, []byte(next)) }()
	}
	readOld, err := io.ReadAll(reader)
	if err != nil || string(readOld) != old {
		t.Fatal("held reader lost its immutable old version", string(readOld), err)
	}
	if err = reader.Close(); err != nil {
		t.Fatal(err)
	}
	if replacement != nil {
		select {
		case err = <-replacement:
			if err != nil {
				t.Fatal("bounded replacement did not survive held reader", err)
			}
		case <-time.After(time.Second):
			t.Fatal("held-reader replacement exceeded bound")
		}
	}
	readNew, err := nativeWorkbenchMailboxRead(path)
	if err != nil || string(readNew) != next {
		t.Fatal("new reader saw non-atomic content", string(readNew), err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "request.json" {
		t.Fatal("mailbox accumulated staging files", entries, err)
	}
}

func TestNativeWorkbenchMailboxRetryAndCancelledCleanup(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "release-held-reader", true: "cancel-held-reader"}[cancelled], func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "response.json")
			if err := os.WriteFile(path, []byte("previous"), 0600); err != nil {
				t.Fatal(err)
			}
			locked, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer locked.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- nativeWorkbenchMailboxReplace(ctx, path, []byte("latest")) }()
			deadline := time.Now().Add(500 * time.Millisecond)
			for {
				if _, err = os.Stat(path + ".pending"); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("writer did not reach held-reader rename")
				}
				time.Sleep(time.Millisecond)
			}
			select {
			case err = <-done:
				t.Fatal("writer completed while old non-share-delete reader still held", err)
			default:
			}
			if cancelled {
				cancel()
			} else if err = locked.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case err = <-done:
			case <-time.After(time.Second):
				t.Fatal("bounded mailbox writer did not stop")
			}
			want := "latest"
			if cancelled {
				want = "previous"
				if !errors.Is(err, context.Canceled) {
					t.Fatal("cancellation lost", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			body, readErr := nativeWorkbenchMailboxRead(path)
			if readErr != nil || string(body) != want {
				t.Fatal("cancel/replace changed atomic target", string(body), readErr)
			}
			if _, err = os.Stat(path + ".pending"); !os.IsNotExist(err) {
				t.Fatal("cancelled writer left pending file", err)
			}
		})
	}
}

func TestNativeWorkbenchMailboxLimitsAndUnknownErrors(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "request.json")
	if _, err := nativeWorkbenchMailboxRead(path); !os.IsNotExist(err) {
		t.Fatal("missing mailbox classification", err)
	}
	if err := nativeWorkbenchMailboxReplace(context.Background(), path, make([]byte, 4097)); err == nil {
		t.Fatal("oversized write accepted")
	}
	if _, err := os.Stat(path + ".pending"); !os.IsNotExist(err) {
		t.Fatal("oversized write created staging file", err)
	}
	if err := os.WriteFile(path, make([]byte, 4097), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := nativeWorkbenchMailboxRead(path); err == nil {
		t.Fatal("oversized actual file accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := nativeWorkbenchMailboxReplace(ctx, filepath.Join(root, "missing", "request.json"), []byte("data"))
	if !os.IsNotExist(err) {
		t.Fatal("unknown filesystem error was retried or swallowed", err)
	}
	if nativeWorkbenchMailboxTransient(errors.New("unknown native mailbox failure")) {
		t.Fatal("unknown errors classified as transient")
	}
}
