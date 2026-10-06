package update

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neko233-com/gocode/internal/installlayout"
)

func healthFixture(t *testing.T, reported string) (string, Config, ed25519.PublicKey) {
	t.Helper()
	root := t.TempDir()
	source := strings.Repeat("a", 40)
	base := installlayout.Manifest{Owner: installlayout.Owner, Schema: 1, Version: "0.4.0", Source: source}
	data, _ := json.Marshal(base)
	if err := os.WriteFile(filepath.Join(root, "base.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	build := func(version string) []byte {
		file := filepath.Join(t.TempDir(), "main.go")
		text := fmt.Sprintf("package main\nimport \"fmt\"\nfunc main(){fmt.Println(%q)}\n", "gocode "+version+" "+runtime.GOOS+"/"+runtime.GOARCH+" "+source)
		if err := os.WriteFile(file, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(filepath.Dir(file), "health.exe")
		cmd := exec.Command("go", "build", "-o", out, file)
		if result, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("health executable build %v %s", err, result)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	names := []string{"gocode-app"}
	platform := "darwin/" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		names = []string{"gocode-app.exe", "gocode-app-gui.exe"}
		platform = "windows/amd64"
	}
	// Linux protocol CI uses the Unix payload format but still probes the real
	// host executable; production publishes only Windows/macOS manifest targets.
	if runtime.GOOS == "linux" {
		platform = "darwin/arm64"
	}
	baseDir := filepath.Join(root, "versions", "0.4.0")
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		t.Fatal(err)
	}
	baseline, next := build("0.4.0"), build(reported)
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(baseDir, name), baseline, 0700); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.CreateTemp(t.TempDir(), "app-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for _, name := range names {
		header := &zip.FileHeader{Name: "versions/0.5.0/" + name, Method: zip.Deflate}
		header.SetMode(0700)
		w, err := z.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(next); err != nil {
			t.Fatal(err)
		}
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
	sum := sha256.Sum256(archive)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := Sign(Manifest{Owner: installlayout.Owner, Schema: 1, Layout: 1, Version: "0.5.0", Source: source, Assets: []Asset{{Name: "app.zip", Platform: platform, Size: int64(len(archive)), SHA256: hex.EncodeToString(sum[:])}}}, private)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manifest" {
			_, _ = w.Write(envelope)
		} else if r.Header.Get("Range") != "" {
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(archive[:1])
		} else {
			_, _ = w.Write(archive)
		}
	}))
	t.Cleanup(server.Close)
	config := Config{Mode: "mirror", Mirror: server.URL + "/mirror", ManifestURLs: []string{server.URL + "/manifest"}}
	return root, config, public
}
func testPlatform() string {
	if runtime.GOOS == "windows" {
		return "windows/amd64"
	}
	if runtime.GOOS == "darwin" {
		return "darwin/" + runtime.GOARCH
	}
	return "darwin/arm64"
}

func TestApplyUsesRealHealthCheckAndRollback(t *testing.T) {
	root, config, key := healthFixture(t, "0.5.0")
	m := Manager{Key: key, Config: config}
	result, err := m.Apply(context.Background(), root, testPlatform())
	if err != nil || !result.Ready {
		t.Fatal(result, err)
	}
	_, active, err := installlayout.Resolve(root, false)
	if err != nil || active.Version != "0.5.0" {
		t.Fatal(active, err)
	}
	if err := Rollback(context.Background(), root, key); err != nil {
		t.Fatal(err)
	}
	_, active, err = installlayout.Resolve(root, false)
	if err != nil || active.Version != "0.4.0" {
		t.Fatal("rollback", active, err)
	}
}
func TestFailedExecutableHealthKeepsOriginalSelection(t *testing.T) {
	root, config, key := healthFixture(t, "0.9.9")
	m := Manager{Key: key, Config: config}
	if _, err := m.Apply(context.Background(), root, testPlatform()); err == nil {
		t.Fatal("mismatched executable activated")
	}
	_, active, err := installlayout.Resolve(root, false)
	if err != nil || active.Version != "0.4.0" {
		t.Fatal("failed health changed active version", active, err)
	}
}
func TestUpdateLockIsExclusiveAndReleased(t *testing.T) {
	root := t.TempDir()
	release, err := lock(root)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := lock(root); err == nil {
		second()
		release()
		t.Fatal("concurrent lock succeeded")
	}
	release()
	release, err = lock(root)
	if err != nil {
		t.Fatal("released lock stayed busy", err)
	}
	release()
}
