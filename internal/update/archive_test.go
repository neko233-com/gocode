package update

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neko233-com/gocode/internal/installlayout"
)

func updateFixture(t *testing.T, extra string) (string, string, []byte, ed25519.PublicKey, string) {
	t.Helper()
	root := t.TempDir()
	data, _ := json.Marshal(installlayout.Manifest{Owner: installlayout.Owner, Schema: 1, Version: "0.4.0"})
	if err := os.WriteFile(filepath.Join(root, "base.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "versions"), 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.CreateTemp(t.TempDir(), "update-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	platform := "darwin/arm64"
	names := []string{"gocode-app"}
	if runtime.GOOS == "windows" {
		platform = "windows/amd64"
		names = []string{"gocode-app.exe", "gocode-app-gui.exe"}
	}
	for _, name := range names {
		w, err := z.Create("versions/0.5.0/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("verified app")); err != nil {
			t.Fatal(err)
		}
	}
	if extra != "" {
		w, err := z.Create(extra)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("must not escape"))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	archive, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(archive)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	m := Manifest{Owner: installlayout.Owner, Schema: 1, Layout: 1, Version: "0.5.0", Source: strings.Repeat("a", 40), Assets: []Asset{{Name: "gocode.zip", Platform: platform, Size: int64(len(archive)), SHA256: hex.EncodeToString(hash[:])}}}
	envelope, err := Sign(m, private)
	if err != nil {
		t.Fatal(err)
	}
	return root, f.Name(), envelope, public, platform
}

func TestSignedStageAndAtomicActivation(t *testing.T) {
	root, archive, envelope, key, platform := updateFixture(t, "")
	m, err := Stage(context.Background(), root, archive, envelope, key, platform)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "current.json")); !os.IsNotExist(err) {
		t.Fatal("staging switched the active pointer")
	}
	if err := installlayout.Activate(root, m); err != nil {
		t.Fatal(err)
	}
	_, active, err := installlayout.Resolve(root, false)
	if err != nil || active.Version != "0.5.0" {
		t.Fatal(active, err)
	}
	if _, err := Stage(context.Background(), root, archive, envelope, key, platform); err == nil {
		t.Fatal("existing version overwritten")
	}
}
func TestRejectTamperingAndTraversalBeforeActivation(t *testing.T) {
	for _, name := range []string{"../outside", "versions/0.5.0/../../outside", "C:/outside", "versions/0.5.0/gocode-app/extra"} {
		root, archive, envelope, key, platform := updateFixture(t, name)
		if _, err := Stage(context.Background(), root, archive, envelope, key, platform); err == nil {
			t.Fatalf("accepted %s", name)
		}
		if _, err := os.Stat(filepath.Join(root, "versions", "0.5.0")); !os.IsNotExist(err) {
			t.Fatal("rejected stage published a directory")
		}
	}
	root, archive, envelope, key, platform := updateFixture(t, "")
	if err := os.WriteFile(archive, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Stage(context.Background(), root, archive, envelope, key, platform); err == nil {
		t.Fatal("tampered archive accepted")
	}
	envelope[len(envelope)/2] ^= 1
	if _, err := Verify(envelope, key); err == nil {
		t.Fatal("tampered metadata accepted")
	}
}
func TestCancelledStageLeavesNoPublishedPayload(t *testing.T) {
	root, archive, envelope, key, platform := updateFixture(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Stage(ctx, root, archive, envelope, key, platform); err == nil {
		t.Fatal("cancelled stage succeeded")
	}
	entries, err := os.ReadDir(filepath.Join(root, "versions"))
	if err != nil || len(entries) != 0 {
		t.Fatal("partial stage retained", entries, err)
	}
}
