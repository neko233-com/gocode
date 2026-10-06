//go:build windows

package filewatch

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsObservationAllowsReplacementWhileDescriptorIsOpen(t *testing.T) {
	dir := t.TempDir()
	for range 5 {
		dir = filepath.Join(dir, strings.Repeat("long-path-", 7))
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "文件😀.txt")
	temp := filepath.Join(dir, "replace.txt")
	if err := os.WriteFile(path, []byte("old descriptor"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(temp, []byte("new pathname"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := OpenRead(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := Replace(context.Background(), temp, path, nil); err != nil {
		t.Fatal("observation prevented external atomic replacement", err)
	}
	old, err := io.ReadAll(f)
	if err != nil || string(old) != "old descriptor" {
		t.Fatal("descriptor identity changed", err)
	}
	result, _ := Read(context.Background(), Entry{Path: path})
	if result.Text != "new pathname" {
		t.Fatal("new pathname not observed", result.Err)
	}
}
