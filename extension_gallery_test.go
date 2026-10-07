package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/godesktop/extensions"
)

func TestNativeGalleryQueryWindowsVSIXInstallAndActivation(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, body := range map[string]string{"package.json": `{"name":"gallery","publisher":"fixture","version":"0.1.0","main":"extension.cjs","contributes":{"commands":[{"command":"gallery.proof","title":"Gallery proof"}]}}`, "extension.cjs": `exports.activate=c=>c.subscriptions.push(require('vscode').commands.registerCommand('gallery.proof',()=> 'actual gallery extension'));`} {
		part, err := writer.Create("extension/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/extensionquery" {
			if r.Method != "POST" || !strings.HasPrefix(r.Header.Get("User-Agent"), "gocode/") || r.Header.Get("Accept") != "application/json;api-version=3.0-preview.1" || r.Header.Get("Content-Type") != "application/json" {
				t.Error("Gallery query impersonated a different product or used wrong protocol")
			}
			var request struct {
				Flags   int
				Filters []struct {
					PageSize int
					Criteria []struct {
						FilterType int
						Value      string
					}
				}
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Flags != 659 || len(request.Filters) != 1 || request.Filters[0].PageSize != 20 || len(request.Filters[0].Criteria) != 2 || request.Filters[0].Criteria[0].FilterType != 8 || request.Filters[0].Criteria[0].Value != "Microsoft.VisualStudio.Code" || request.Filters[0].Criteria[1].FilterType != 10 || request.Filters[0].Criteria[1].Value != "native" {
				t.Error("Gallery source query contract differs", err)
			}
			json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"extensions": []any{map[string]any{"extensionName": "gallery", "publisher": map[string]string{"publisherName": "fixture"}, "displayName": "Native Gallery", "versions": []any{map[string]any{"version": "0.1.0", "targetPlatform": "darwin-arm64", "files": []any{map[string]string{"assetType": "Microsoft.VisualStudio.Services.VSIXPackage", "source": server.URL + "/must-not-fetch-mac"}}}, map[string]any{"version": "0.1.0", "targetPlatform": "universal", "files": []any{map[string]string{"assetType": "Microsoft.VisualStudio.Services.VSIXPackage", "source": server.URL + "/must-not-fetch-universal"}}}, map[string]any{"version": "0.1.0", "targetPlatform": "win32-x64", "files": []any{map[string]string{"assetType": "Microsoft.VisualStudio.Services.VSIXPackage", "source": server.URL + "/fixture/gallery/0.1.0/package"}}}}}}}}})
			return
		}
		if r.URL.Path == "/fixture/gallery/0.1.0/package" && r.URL.Query().Get("targetPlatform") == "win32-x64" {
			w.Write(buffer.Bytes())
			return
		}
		t.Error("unexpected platform/resource fetched", r.URL.String())
		http.NotFound(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	items, err := fetchGallery(ctx, server.Client(), server.URL, "native")
	if err != nil || len(items) != 1 || items[0].TargetPlatform != "win32-x64" {
		t.Fatal("native platform query failed", items, err)
	}
	root := t.TempDir()
	archive, cleanup, err := downloadCatalogVSIX(ctx, server.Client(), root, items[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = validateCatalogVSIX(archive, items[0]); err != nil {
		cleanup()
		t.Fatal(err)
	}
	installed, err := extensions.Install(root, archive)
	cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if entries, _ := filepath.Glob(filepath.Join(root, ".download-*")); len(entries) != 0 {
		t.Fatal("Gallery install left downloads")
	}
	host, err := extensions.Start(ctx, root, []extensions.Extension{installed})
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	var result string
	if err = host.Call(ctx, "execute", map[string]string{"command": "gallery.proof"}, &result); err != nil || result != "actual gallery extension" {
		t.Fatal("Gallery VSIX did not activate in real Node host", result, err)
	}
	items[0].Version = "9.9.9"
	archive, cleanup, err = downloadCatalogVSIX(ctx, server.Client(), root, items[0])
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err = validateCatalogVSIX(archive, items[0]); err == nil {
		t.Fatal("Gallery installed different selected version")
	}
	if _, err = os.Stat(installed.Path); err != nil {
		t.Fatal("rejected package changed installed extension", err)
	}
}
func TestGalleryURLsRejectCredentialsAndForeignAssets(t *testing.T) {
	for _, raw := range []string{"http://gallery.example", "https://user:secret@gallery.example", "https://gallery.example?token=secret"} {
		if _, err := galleryURL(raw); err == nil {
			t.Fatal("invalid Gallery endpoint accepted")
		}
	}
	if err := galleryAssetHTTPS("https://gallery.example", "https://foreign.example/payload"); err == nil {
		t.Fatal("unrelated Gallery asset host accepted")
	}
	if err := galleryAssetHTTPS("https://marketplace.visualstudio.com/_apis/public/gallery", "https://fixture.gallery.vsassets.io/package"); err != nil {
		t.Fatal(err)
	}
}
