package update

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupRecognizesSignedVersionsAndPreservesUnknownFiles(t *testing.T) {
	root, archive, envelope, key, platform := updateFixture(t, "")
	manifest, err := Stage(context.Background(), root, archive, envelope, key, platform)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "user-notes.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(root, "versions", "0.6.0")
	if err := os.Mkdir(foreign, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "notes.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "versions", manifest.Version, "notes.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Cleanup(root, key); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{filepath.Join(root, "user-notes.txt"), filepath.Join(foreign, "notes.txt"), filepath.Join(root, "versions", manifest.Version, "notes.txt")} {
		if _, err := os.Stat(file); err != nil {
			t.Fatal("user/unknown file removed", file, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "versions", manifest.Version, "update-manifest.json")); !os.IsNotExist(err) {
		t.Fatal("signed update was retained", err)
	}
}
