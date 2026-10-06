package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neko233-com/godesktop/extensions"
)

func TestBuiltinUpgradePreservesRollbackExtensionStore(t *testing.T) {
	root := t.TempDir()
	manifest, err := bundled.ReadFile("bundled/package.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest = bytes.ReplaceAll(manifest, []byte(`"version": "`+bundledVersion+`"`), []byte(`"version": "0.3.0"`))
	var data bytes.Buffer
	w := zip.NewWriter(&data)
	for path, content := range map[string][]byte{"extension/package.json": manifest, "extension/extension.cjs": []byte("module.exports={};")} {
		f, err := w.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "old.vsix")
	if err := os.WriteFile(archive, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	old, err := extensions.Install(root, archive)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		current, err := workbenchExtensions(root)
		if err != nil || len(current) != 1 || current[0].Manifest.Version != bundledVersion || !strings.Contains(current[0].Path, ".gocode-bundled") {
			t.Fatal("managed builtin selection", current, err)
		}
		shared, err := extensions.List(root)
		if err != nil || len(shared) != 1 || shared[0].Manifest.Version != "0.3.0" || shared[0].Path != old.Path {
			t.Fatal("rollback's shared store was changed", shared, err)
		}
		if actual, err := os.ReadFile(filepath.Join(old.Path, "package.json")); err != nil || !bytes.Equal(actual, manifest) {
			t.Fatal("old builtin payload was overwritten", err)
		}
	}
}
