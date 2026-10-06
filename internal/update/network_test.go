package update

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMetadataFallbackAndMirrorTamperRecovery(t *testing.T) {
	root, archive, envelope, key, _ := updateFixture(t, "")
	body, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte{}, body...)
	tampered[len(tampered)/2] ^= 1
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/bad-metadata":
			_, _ = w.Write([]byte(`{"payload":{},"signature":"untrusted"}`))
		case r.URL.Path == "/good-metadata":
			_, _ = w.Write(envelope)
		case strings.HasPrefix(r.URL.Path, "/bad-mirror/"):
			if r.Header.Get("Range") != "" {
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(tampered[:1])
			} else {
				_, _ = w.Write(tampered)
			}
		case strings.HasPrefix(r.URL.Path, "/good-mirror/"):
			if r.Header.Get("Range") != "" {
				time.Sleep(5 * time.Millisecond)
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(body[:1])
			} else {
				_, _ = w.Write(body)
			}
		default:
			http.Error(w, "offline", http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	m := &Manager{HTTP: server.Client(), Key: key, Config: Config{Mode: "auto", ManifestURLs: []string{server.URL + "/bad-metadata", server.URL + "/good-metadata"}, Mirrors: []string{server.URL + "/bad-mirror", server.URL + "/good-mirror"}}}
	candidate, err := m.Check(context.Background())
	if err != nil || candidate.MetadataURL != server.URL+"/good-metadata" {
		t.Fatal(candidate, err)
	}
	routes, err := m.Routes(context.Background(), server.URL+"/offline")
	if err != nil || len(routes) != 2 {
		t.Fatal(routes, err)
	}
	asset := candidate.Manifest.Assets[0]
	file, route, err := m.Download(context.Background(), root, asset, []Route{{URL: mirrorURL(server.URL+"/bad-mirror", server.URL+"/offline")}, {URL: mirrorURL(server.URL+"/good-mirror", server.URL+"/offline")}})
	if err != nil || !strings.Contains(route.URL, "/good-mirror/") {
		t.Fatal(route, err)
	}
	defer os.Remove(file)
	if err := verifyFileHash(context.Background(), file, asset); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".gocode-download-") {
			count++
		}
	}
	if count != 1 {
		t.Fatal("failed mirror retained download", count)
	}
}

func TestManualRouteAndCancelledProbe(t *testing.T) {
	_, _, envelope, key, _ := updateFixture(t, "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/mirror/") {
			w.WriteHeader(http.StatusPartialContent)
			_, _ = io.WriteString(w, "x")
		} else {
			http.Error(w, "unreachable", 503)
		}
	}))
	defer server.Close()
	m := &Manager{HTTP: server.Client(), Key: key, Config: Config{Mode: "mirror", Mirror: server.URL + "/mirror", ManifestURLs: []string{server.URL + "/metadata"}}}
	routes, err := m.Routes(context.Background(), "https://github.com/neko233-com/gocode/releases/download/v0.5.0/gocode.zip")
	if err != nil || len(routes) != 1 || routes[0].Direct || !strings.Contains(routes[0].URL, "/mirror/https://github.com/") {
		t.Fatal(routes, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.Routes(ctx, server.URL+"/offline"); err == nil {
		t.Fatal("cancelled probe succeeded")
	}
	if _, err := Verify(envelope, make([]byte, 32)); err == nil {
		t.Fatal("wrong publisher key accepted")
	}
	for _, raw := range []string{"http://example.com/", "https://user:secret@example.com/", "file:///C:/app.zip"} {
		if validURL(raw) == nil {
			t.Fatal("unsafe endpoint", raw)
		}
	}
}
