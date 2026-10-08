package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRollbackExtensionDirectoryUsesExplicitAndActualUserDefault(t *testing.T) {
	private := t.TempDir()
	t.Setenv("APPDATA", private)
	t.Setenv("XDG_CONFIG_HOME", private)
	config, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	actual, err := extensionDirectory("")
	if err != nil || actual != filepath.Join(config, "gocode", "extensions") {
		t.Fatal("default rollback extension root differs from workbench", actual, err)
	}
	explicit := filepath.Join(t.TempDir(), "explicit 世界")
	actual, err = extensionDirectory(explicit)
	if err != nil || actual != explicit {
		t.Fatal("explicit rollback directory was ignored", actual, err)
	}
}
