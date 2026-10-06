package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConPTYRuntimeRejectsSameSizeCorruptionAndMissingFiles(t *testing.T) {
	root := t.TempDir()
	for _, file := range conPTYFiles {
		if err := os.WriteFile(filepath.Join(root, file.name), make([]byte, file.size), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := VerifyConPTY(root); err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatal("same-size native DLL corruption accepted", err)
	}
	if err := VerifyConPTY(t.TempDir()); err == nil {
		t.Fatal("absent native DLL accepted")
	}
}
