package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neko233-com/godesktop/extensions"
)

func TestExtensionQueryClassifiesExactIDsAndLocalFilters(t *testing.T) {
	for _, test := range []struct {
		raw, text, id string
		remote        bool
	}{
		{"", "", "", false},
		{"Go language", "Go language", "", true},
		{"user@host", "user@host", "", true},
		{" GoLang.Go ", "", "golang.go", true},
		{"@id:GoLang.Go", "", "golang.go", true},
		{"@installed", "", "", false},
		{"@enabled Go language", "Go language", "", false},
		{"@disabled @id:GoLang.Go", "", "golang.go", false},
		{"@installed GoLang.Go", "", "golang.go", false},
	} {
		t.Run(test.raw, func(t *testing.T) {
			q, err := parseExtensionQuery(test.raw)
			if err != nil || q.Text != test.text || q.ID != test.id || q.catalogVisible() != test.remote {
				t.Fatal(q, err)
			}
		})
	}
	for _, raw := range []string{"@unknown", "@id:", "@id:publisher", "@id:../escape", "@id:a.b @id:c.d", "@id:a.b extra", "@id:" + strings.Repeat("x", 101) + ".name", strings.Repeat("x", 1025)} {
		if _, err := parseExtensionQuery(raw); err == nil {
			t.Fatalf("invalid query accepted: %q", raw)
		}
	}
	e := extensionInfo{Name: "Go Tools", ID: "golang.Go", Description: "Go language"}
	for _, test := range []struct {
		query           string
		disabled, match bool
	}{
		{"@id:golang.go", false, true},
		{"golang.other", false, false},
		{"@enabled Go", false, true},
		{"@enabled Go", true, false},
		{"@disabled @id:golang.go", true, true},
		{"@disabled @id:golang.go", false, false},
		{"@enabled @disabled", false, false},
		{"@enabled @disabled", true, false},
	} {
		q, err := parseExtensionQuery(test.query)
		if err != nil || q.matchesInstalled(e, test.disabled) != test.match {
			t.Fatal(test, q, err)
		}
	}
}

// Route logical Open VSX URLs to a real TLS fixture. No live catalog request is
// made by tests; response bytes, contexts and platform fallback use HTTP/TLS.
type extensionQueryTLSTransport struct {
	base      *url.URL
	transport http.RoundTripper
}

func (r extensionQueryTLSTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if catalogHTTPS(req.URL.String()) != nil {
		return nil, errors.New("fixture rejected non-Open-VSX request")
	}
	copy := req.Clone(req.Context())
	address := *req.URL
	address.Host = r.base.Host
	copy.URL, copy.Host = &address, req.URL.Host
	response, err := r.transport.RoundTrip(copy)
	if response != nil {
		response.Request = req
	}
	return response, err
}

func extensionQueryTLSClient(t *testing.T, handler http.HandlerFunc) *http.Client {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	base, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := newCatalogClient()
	client.Timeout = 3 * time.Second
	client.Transport = extensionQueryTLSTransport{base, server.Client().Transport}
	return client
}

func extensionQueryMetadata(platform string) catalogExtension {
	e := catalogExtension{Publisher: "Fixture", Name: "Native", Version: "0.1.0", TargetPlatform: platform, DisplayName: "Native TLS extension"}
	e.Files.Download = "https://open-vsx.org/api/Fixture/Native/0.1.0/file/fixture.native-0.1.0.vsix"
	e.Files.SHA256 = "https://open-vsx.org/api/Fixture/Native/0.1.0/file/fixture.native-0.1.0.sha256"
	return e
}

func TestExtensionQueryOpenVSXExactLatestFreezesVersionAndPlatform(t *testing.T) {
	for _, platform := range []string{"win32-x64", "universal", "missing"} {
		t.Run(platform, func(t *testing.T) {
			var requests atomic.Int32
			client := extensionQueryTLSClient(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != http.MethodGet || !strings.HasPrefix(r.Header.Get("User-Agent"), "gocode/") {
					t.Error("wrong logical metadata method/identity", r.Method, r.Header)
				}
				if platform == "missing" || !strings.Contains(r.URL.Path, "/"+platform+"/") {
					http.NotFound(w, r)
					return
				}
				e := extensionQueryMetadata(platform)
				switch r.URL.Path {
				case "/api/fixture/native/" + platform + "/latest":
					e.Files.Download = "https://open-vsx.org/api/fixture/native/latest/file/mutable.vsix"
				case "/api/Fixture/Native/" + platform + "/0.1.0":
				default:
					t.Error("unexpected metadata route", r.URL.String())
					http.NotFound(w, r)
					return
				}
				_ = json.NewEncoder(w).Encode(e)
			})
			items, err := fetchCatalog(context.Background(), client, "@id:Fixture.Native")
			if err != nil {
				t.Fatal(err)
			}
			if platform == "missing" {
				if len(items) != 0 || requests.Load() != 2 {
					t.Fatal("legitimate missing ID became a package error", items, requests.Load())
				}
				return
			}
			wantRequests := int32(2)
			if platform == "universal" {
				wantRequests = 4
			}
			if len(items) != 1 || items[0].TargetPlatform != platform || items[0].Version != "0.1.0" || strings.Contains(items[0].Files.Download, "/latest/") || requests.Load() != wantRequests {
				t.Fatal("selected version/platform was not frozen", items, requests.Load())
			}
		})
	}
}

func TestExtensionQueryOpenVSXRejectsIdentityResourcesAndBounds(t *testing.T) {
	for _, scenario := range []string{"latest-id", "immutable-id", "immutable-version", "platform", "mutable-version", "foreign-download", "foreign-sha", "large-description", "oversize", "redirect", "status"} {
		t.Run(scenario, func(t *testing.T) {
			client := extensionQueryTLSClient(t, func(w http.ResponseWriter, r *http.Request) {
				latest := strings.HasSuffix(r.URL.Path, "/latest")
				e := extensionQueryMetadata("win32-x64")
				switch scenario {
				case "latest-id":
					e.Name = "other"
				case "immutable-id":
					if !latest {
						e.Publisher = "other"
					}
				case "immutable-version":
					if !latest {
						e.Version = "9.9.9"
					}
				case "platform":
					e.TargetPlatform = "darwin-arm64"
				case "mutable-version":
					e.Version = "latest"
				case "foreign-download":
					e.Files.Download = "https://foreign.example/package"
				case "foreign-sha":
					e.Files.SHA256 = "https://foreign.example/hash"
				case "large-description":
					e.Description = strings.Repeat("x", (16<<10)+1)
				case "oversize":
					_, _ = io.WriteString(w, strings.Repeat("x", (1<<20)+1))
					return
				case "redirect":
					http.Redirect(w, r, "https://foreign.example/api", http.StatusFound)
					return
				case "status":
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				_ = json.NewEncoder(w).Encode(e)
			})
			if items, err := fetchCatalog(context.Background(), client, "fixture.native"); err == nil || len(items) != 0 {
				t.Fatal("invalid exact metadata offered for installation", scenario, items, err)
			}
		})
	}
	var requests atomic.Int32
	client := extensionQueryTLSClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/api/-/search" || r.URL.Query().Get("query") != "Go tools" || r.URL.Query().Get("size") != "20" || r.URL.Query().Get("targetPlatform") != "win32-x64" {
			t.Error("ordinary search changed", r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"extensions": []catalogExtension{}})
	})
	if items, err := fetchCatalog(context.Background(), client, "Go tools"); err != nil || len(items) != 0 || requests.Load() != 1 {
		t.Fatal(items, err, requests.Load())
	}
	for _, query := range []string{"@installed", "@enabled Go", "@disabled @id:fixture.native"} {
		if items, err := fetchCatalog(context.Background(), client, query); err != nil || len(items) != 0 || requests.Load() != 1 {
			t.Fatal("local filter performed remote work", query, items, err, requests.Load())
		}
	}
}

func TestExtensionQueryOpenVSXNetworkCancellation(t *testing.T) {
	started := make(chan struct{}, 1)
	client := extensionQueryTLSClient(t, func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := fetchCatalog(ctx, client, "fixture.native"); done <- err }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("TLS query never started")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("network query cancellation lost", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled TLS query did not stop")
	}
}

func TestExtensionQueryWorkerSupersedesRejectedReceiptAndStops(t *testing.T) {
	var allowed atomic.Bool
	rejected, second := make(chan struct{}, 1), make(chan struct{}, 2)
	mailbox := make(chan func(), 4)
	client := extensionQueryTLSClient(t, func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) != 6 {
			t.Error("unexpected worker metadata path", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		e := extensionQueryMetadata("win32-x64")
		e.Name = parts[3]
		if e.Name == "second" {
			second <- struct{}{}
		}
		_ = json.NewEncoder(w).Encode(e)
	})
	m := testModel(t)
	stop := m.startExtensionCatalog(context.Background(), func(fn func()) bool {
		if !allowed.Load() {
			select {
			case rejected <- struct{}{}:
			default:
			}
			return false
		}
		mailbox <- fn
		return true
	}, client)
	defer stop()
	m.extensionsView.catalog.search("fixture.first")
	select {
	case <-rejected:
	case <-time.After(2 * time.Second):
		t.Fatal("first genuine receipt was never rejected")
	}
	m.extensionsView.catalog.search("@id:fixture.second")
	select {
	case <-second:
	case <-time.After(2 * time.Second):
		t.Fatal("new query remained behind an obsolete rejected receipt")
	}
	allowed.Store(true)
	select {
	case receipt := <-mailbox:
		if !m.extensionsView.catalog.busy || len(m.extensionsView.catalog.results) != 0 {
			t.Fatal("worker changed live UI before receipt")
		}
		receipt()
	case <-time.After(2 * time.Second):
		t.Fatal("latest worker receipt missing")
	}
	if m.extensionsView.catalog.busy || len(m.extensionsView.catalog.results) != 1 || !strings.EqualFold(m.extensionsView.catalog.results[0].id(), "fixture.second") {
		t.Fatal("latest exact result not admitted", m.extensionsView.catalog)
	}
	m.extensionsView.catalog.search("@installed")
	if m.extensionsView.catalog.busy || len(m.extensionsView.catalog.results) != 0 {
		t.Fatal("local query retained remote cards")
	}
	m.extensionsView.catalog.search("@id:invalid")
	if m.extensionsView.catalog.busy || m.extensionsView.catalog.error == "" {
		t.Fatal("invalid query left worker busy")
	}
	allowed.Store(false)
	select {
	case <-rejected:
	default:
	}
	m.extensionsView.catalog.search("fixture.first")
	select {
	case <-rejected:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown receipt not rejected")
	}
	done := make(chan struct{})
	go func() { stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("catalog shutdown retained a rejected query")
	}
}

func TestExtensionQueryWorkerDropsQueuedResultAndAdmitsError(t *testing.T) {
	mailbox := make(chan func(), 4)
	client := extensionQueryTLSClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/second/") {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		e := extensionQueryMetadata("win32-x64")
		e.Name = "first"
		_ = json.NewEncoder(w).Encode(e)
	})
	m := testModel(t)
	stop := m.startExtensionCatalog(context.Background(), func(fn func()) bool { mailbox <- fn; return true }, client)
	defer stop()
	wait := func() func() {
		t.Helper()
		select {
		case receipt := <-mailbox:
			return receipt
		case <-time.After(2 * time.Second):
			t.Fatal("real worker receipt missing")
			return nil
		}
	}
	m.extensionsView.catalog.search("fixture.first")
	stale := wait()
	m.extensionsView.catalog.search("fixture.second")
	latest := wait()
	stale()
	if !m.extensionsView.catalog.busy || len(m.extensionsView.catalog.results) != 0 || m.extensionsView.catalog.error != "" {
		t.Fatal("cancelled queued receipt changed the new query", m.extensionsView.catalog)
	}
	latest()
	if m.extensionsView.catalog.busy || len(m.extensionsView.catalog.results) != 0 || !strings.Contains(m.extensionsView.catalog.error, "503") {
		t.Fatal("real failed query did not leave busy with its error", m.extensionsView.catalog)
	}
}

func TestExtensionQueryGalleryExactTLSArchiveAndActivation(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for name, body := range map[string]string{"package.json": `{"name":"exact","publisher":"fixture","version":"0.1.0","main":"extension.cjs","contributes":{"commands":[{"command":"exact.proof","title":"Exact proof"}]}}`, "extension.cjs": `exports.activate=c=>c.subscriptions.push(require('vscode').commands.registerCommand('exact.proof',()=> 'exact TLS process activation 世界'));`} {
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
	var responseID atomic.Value
	responseID.Store("exact")
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/extensionquery" {
			var body struct {
				Flags   int
				Filters []struct {
					PageNumber, PageSize int
					Criteria             []struct {
						FilterType int
						Value      string
					}
				}
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || r.Method != "POST" || !strings.HasPrefix(r.Header.Get("User-Agent"), "gocode/") || body.Flags != 659 || len(body.Filters) != 1 || body.Filters[0].PageNumber != 1 || body.Filters[0].PageSize != 20 || len(body.Filters[0].Criteria) != 2 || body.Filters[0].Criteria[0].FilterType != 8 || body.Filters[0].Criteria[1].FilterType != 7 || body.Filters[0].Criteria[1].Value != "fixture.exact" {
				t.Error("primary exact-name query/identity/bounds differ", body, err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"extensions": []any{map[string]any{"extensionName": responseID.Load(), "publisher": map[string]string{"publisherName": "fixture"}, "versions": []any{map[string]any{"version": "0.1.0", "targetPlatform": "win32-x64", "files": []any{map[string]string{"assetType": "Microsoft.VisualStudio.Services.VSIXPackage", "source": server.URL + "/package"}}}}}}}}})
			return
		}
		if r.URL.Path == "/package" && r.URL.Query().Get("targetPlatform") == "win32-x64" {
			_, _ = w.Write(archive.Bytes())
			return
		}
		t.Error("unexpected private gallery route", r.URL.String())
		http.NotFound(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	items, err := fetchGallery(ctx, server.Client(), server.URL, "@id:Fixture.Exact")
	if err != nil || len(items) != 1 || items[0].id() != "fixture.exact" {
		t.Fatal(items, err)
	}
	root := t.TempDir()
	path, cleanup, err := downloadCatalogVSIX(ctx, server.Client(), root, items[0])
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := validateCatalogVSIX(path, items[0]); err != nil {
		t.Fatal(err)
	}
	installed, err := extensions.Install(root, path)
	if err != nil {
		t.Fatal(err)
	}
	host, err := extensions.Start(ctx, root, []extensions.Extension{installed})
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	var result string
	if err := host.Call(ctx, "execute", map[string]string{"command": "exact.proof"}, &result); err != nil || result != "exact TLS process activation 世界" {
		t.Fatal(result, err)
	}
	wrong := items[0]
	wrong.Version = "9.9.9"
	if validateCatalogVSIX(path, wrong) == nil {
		t.Fatal("different immutable archive version accepted")
	}
	wrong = items[0]
	wrong.Publisher = "other"
	if validateCatalogVSIX(path, wrong) == nil {
		t.Fatal("different archive identity accepted")
	}
	if _, err := os.Stat(installed.Path); err != nil {
		t.Fatal("rejected archive changed installation", err)
	}
	responseID.Store("unrelated")
	if items, err := fetchGallery(ctx, server.Client(), server.URL, "fixture.exact"); err == nil || len(items) != 0 {
		t.Fatal("wrong exact gallery identity accepted", items, err)
	}
}

func TestExtensionQueryGalleryExactResponseBoundsAndRedirects(t *testing.T) {
	var foreignRequests atomic.Int32
	foreign := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignRequests.Add(1)
		t.Error("query followed an unauthorized metadata host")
	}))
	defer foreign.Close()
	for _, scenario := range []string{"missing", "duplicate", "versions", "oversize", "foreign-asset", "http-asset", "credential-asset", "foreign-redirect", "redirect-limit", "status"} {
		t.Run(scenario, func(t *testing.T) {
			var server *httptest.Server
			server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				version := galleryVersion{Version: "0.1.0", TargetPlatform: "universal", AssetURI: server.URL + "/immutable/0.1.0"}
				entry := galleryEntry{ExtensionName: "native", Versions: []galleryVersion{version}}
				entry.Publisher.PublisherName = "fixture"
				entries := []galleryEntry{entry}
				switch scenario {
				case "missing":
					entries = nil
				case "duplicate":
					entries = append(entries, entry)
				case "versions":
					entries[0].Versions = make([]galleryVersion, 129)
				case "oversize":
					_, _ = io.WriteString(w, strings.Repeat("x", (1<<20)+1))
					return
				case "foreign-asset":
					entries[0].Versions[0].AssetURI = foreign.URL
				case "http-asset":
					entries[0].Versions[0].AssetURI = "http://" + strings.TrimPrefix(server.URL, "https://")
				case "credential-asset":
					entries[0].Versions[0].AssetURI = "https://user:secret@" + strings.TrimPrefix(server.URL, "https://")
				case "foreign-redirect":
					http.Redirect(w, r, foreign.URL, http.StatusTemporaryRedirect)
					return
				case "redirect-limit":
					http.Redirect(w, r, server.URL+"/extensionquery", http.StatusTemporaryRedirect)
					return
				case "status":
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"extensions": entries}}})
			}))
			defer server.Close()
			client := server.Client()
			client.Timeout = 2 * time.Second
			items, err := fetchGallery(context.Background(), client, server.URL, "@id:fixture.native")
			if scenario == "missing" {
				if err != nil || len(items) != 0 {
					t.Fatal("missing exact Gallery result became an error", items, err)
				}
			} else if err == nil || len(items) != 0 {
				t.Fatal("invalid Gallery exact response was offered", scenario, items, err)
			}
			if client.CheckRedirect != nil {
				t.Fatal("query mutated shared HTTP client")
			}
		})
	}
	if foreignRequests.Load() != 0 {
		t.Fatal("foreign metadata server was contacted", foreignRequests.Load())
	}
}

func TestExtensionQueryGalleryNetworkCancellationAndDownloadSize(t *testing.T) {
	started := make(chan struct{}, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/package" {
			w.Header().Set("Content-Length", "67108865")
			_, _ = io.WriteString(w, "not a 64 MiB package")
			return
		}
		started <- struct{}{}
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := fetchGallery(ctx, server.Client(), server.URL, "fixture.native"); done <- err }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("Gallery TLS query never started")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("Gallery query cancellation lost", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled Gallery query did not stop")
	}
	e := catalogExtension{Name: "native", Publisher: "fixture", Version: "0.1.0", Gallery: server.URL}
	e.Files.Download = server.URL + "/package"
	root := t.TempDir()
	if path, cleanup, err := downloadCatalogVSIX(context.Background(), server.Client(), root, e); err == nil || path != "" || cleanup != nil {
		t.Fatal("over-limit package offered", path, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("rejected package left temporary downloads", entries, err)
	}
}
