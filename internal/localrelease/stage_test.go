package localrelease

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neko233-com/gocode/internal/installlayout"
	"github.com/neko233-com/gocode/internal/update"
)

func signedFixture(t *testing.T) (string, string, ed25519.PublicKey) {
	t.Helper()
	root := t.TempDir()
	archive := filepath.Join(root, "gocode-0.24.0-windows-amd64.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(file)
	for name, body := range map[string]string{
		"versions/0.24.0/gocode-app.exe":     "real fixture console bytes",
		"versions/0.24.0/gocode-app-gui.exe": "real distinct fixture GUI bytes",
		"LICENSE.txt":                        "fixture license",
	} {
		entry, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := HashFile(context.Background(), archive, update.MaxArchiveBytes)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := update.Sign(update.Manifest{Owner: installlayout.Owner, Schema: 1, Layout: 1, Version: "0.24.0", Source: strings.Repeat("a", 40), Assets: []update.Asset{{Name: filepath.Base(archive), Platform: "windows/amd64", Size: digest.Bytes, SHA256: digest.SHA256}}}, private)
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, "update-manifest.json")
	if err := os.WriteFile(manifest, envelope, 0600); err != nil {
		t.Fatal(err)
	}
	return archive, manifest, public
}

func TestActualSignedLocalStageUsesProductionIntegrityPath(t *testing.T) {
	archive, manifest, key := signedFixture(t)
	root := t.TempDir()
	staged, err := stageWithKey(context.Background(), root, archive, manifest, "0.24.0", strings.Repeat("a", 40), "windows/amd64", key)
	if err != nil {
		t.Fatal(err)
	}
	if staged.Console.SHA256 == staged.GUI.SHA256 || staged.Source != strings.Repeat("a", 40) || staged.Version != "0.24.0" {
		t.Fatalf("actual signed local files were not independently staged: %+v", staged)
	}
	data, err := os.ReadFile(staged.GUI.Path)
	if err != nil || string(data) != "real distinct fixture GUI bytes" {
		t.Fatal("production staging did not extract exact GUI bytes", err)
	}
	// Repeating a stage must refuse the occupied owned root; no version overwrite.
	if _, err := stageWithKey(context.Background(), root, archive, manifest, "0.24.0", strings.Repeat("a", 40), "windows/amd64", key); err == nil {
		t.Fatal("occupied signed version was overwritten")
	}
}

func TestActualLocalStageRejectsWrongSignatureSourceAndMutatedArchive(t *testing.T) {
	archive, manifest, key := signedFixture(t)
	wrong, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		key    ed25519.PublicKey
		source string
	}{{"wrong-key", wrong, strings.Repeat("a", 40)}, {"wrong-source", key, strings.Repeat("b", 40)}} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if _, err := stageWithKey(context.Background(), root, archive, manifest, "0.24.0", test.source, "windows/amd64", test.key); err == nil {
				t.Fatal("untrusted metadata was staged")
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatal("rejected input mutated owned stage", err)
			}
		})
	}
	file, err := os.OpenFile(archive, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.Write([]byte("late real mutation"))
	err = errors.Join(err, file.Close())
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if _, err := stageWithKey(context.Background(), root, archive, manifest, "0.24.0", strings.Repeat("a", 40), "windows/amd64", key); err == nil {
		t.Fatal("mutated real archive matched signed digest")
	}
}

func TestProductionPublisherIdentityNeverAcceptsEphemeralUnitKey(t *testing.T) {
	archive, manifest, _ := signedFixture(t)
	root := t.TempDir()
	if _, err := StageSigned(context.Background(), root, archive, manifest, "0.24.0", strings.Repeat("a", 40), "windows/amd64"); err == nil {
		t.Fatal("test signature replaced installed production publisher trust")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("untrusted publisher mutated actual owned stage", err)
	}
}
