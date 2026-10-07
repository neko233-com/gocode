package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gocode/internal/uidispatch"
)

type catalogExtension struct {
	Gallery        string                            `json:"-"`
	TargetPlatform string                            `json:"targetPlatform"`
	Name           string                            `json:"name"`
	Publisher      string                            `json:"namespace"`
	DisplayName    string                            `json:"displayName"`
	Description    string                            `json:"description"`
	Version        string                            `json:"version"`
	Files          struct{ Download, SHA256 string } `json:"files"`
}

func (e catalogExtension) id() string { return e.Publisher + "." + e.Name }
func (e catalogExtension) info() extensionInfo {
	name := e.DisplayName
	if name == "" {
		name = e.Name
	}
	return extensionInfo{name, e.id(), e.Description, e.Version}
}

type extensionCatalog struct {
	results    []catalogExtension
	busy       bool
	error      string
	generation uint64
	search     func(string)
}

type extensionQuerySpec struct {
	Text, ID                     string
	Installed, Enabled, Disabled bool
}

func validCatalogID(id string) bool {
	parts := strings.Split(id, ".")
	if len(parts) != 2 {
		return false
	}
	for _, part := range parts {
		if len(part) == 0 || len(part) > 100 {
			return false
		}
		for _, c := range part {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
				return false
			}
		}
	}
	return true
}

// Installed filters stay local. A full publisher.extension or @id query uses
// named metadata rather than assuming a free-text search indexes full IDs.
func parseExtensionQuery(raw string) (extensionQuerySpec, error) {
	var q extensionQuerySpec
	if len(raw) > 1024 {
		return q, errors.New("extension query exceeds limit")
	}
	var text []string
	for _, token := range strings.Fields(raw) {
		lower := strings.ToLower(token)
		switch lower {
		case "@installed":
			q.Installed = true
		case "@enabled":
			q.Enabled = true
		case "@disabled":
			q.Disabled = true
		default:
			if strings.HasPrefix(lower, "@id:") {
				id := strings.TrimPrefix(lower, "@id:")
				if q.ID != "" || !validCatalogID(id) {
					return q, errors.New("exact extension query requires one publisher.extension ID")
				}
				q.ID = id
			} else if strings.HasPrefix(token, "@") {
				return q, errors.New("unsupported extension search filter")
			} else {
				text = append(text, token)
			}
		}
	}
	q.Text = strings.Join(text, " ")
	if q.ID != "" && q.Text != "" {
		return q, errors.New("exact extension query cannot include search text")
	}
	if q.ID == "" && validCatalogID(q.Text) {
		q.ID, q.Text = strings.ToLower(q.Text), ""
	}
	return q, nil
}

func (q extensionQuerySpec) catalogVisible() bool {
	return !q.Installed && !q.Enabled && !q.Disabled && (q.Text != "" || q.ID != "")
}

func (q extensionQuerySpec) matchesInstalled(e extensionInfo, disabled bool) bool {
	return !(q.Enabled && disabled || q.Disabled && !disabled) &&
		(q.ID == "" || strings.EqualFold(q.ID, e.ID)) &&
		quickMatches(e.Name+" "+e.ID+" "+e.Description, q.Text)
}

func catalogHTTPS(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() != "open-vsx.org" || u.User != nil {
		return errors.New("invalid Open VSX resource URL")
	}
	return nil
}
func fetchCatalog(ctx context.Context, client *http.Client, query string) ([]catalogExtension, error) {
	q, err := parseExtensionQuery(query)
	if err != nil || !q.catalogVisible() {
		return nil, err
	}
	if q.ID != "" {
		return fetchCatalogExact(ctx, client, q.ID)
	}
	values := url.Values{"query": {q.Text}, "size": {"20"}, "targetPlatform": {"win32-x64"}}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://open-vsx.org/api/-/search?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Open VSX HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, errors.New("catalog response exceeds limit")
	}
	var result struct {
		Extensions []catalogExtension `json:"extensions"`
	}
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if len(result.Extensions) > 20 {
		return nil, errors.New("catalog returned too many entries")
	}
	for _, e := range result.Extensions {
		if len(e.id()) > 201 || len(e.Description) > 16<<10 || len(e.DisplayName) > 1024 || catalogHTTPS(e.Files.Download) != nil {
			return nil, errors.New("invalid catalog entry")
		}
	}
	return result.Extensions, nil
}

func validCatalogMetadata(e catalogExtension) error {
	if !validCatalogID(e.id()) || len(e.Description) > 16<<10 || len(e.DisplayName) > 1024 ||
		e.Version == "" || len(e.Version) > 128 || e.Version == "latest" ||
		strings.ContainsAny(e.Version, "/\\\x00 \t\r\n") || e.Version == "." || e.Version == ".." {
		return errors.New("invalid extension version metadata")
	}
	if catalogHTTPS(e.Files.Download) != nil || e.Files.SHA256 != "" && catalogHTTPS(e.Files.SHA256) != nil {
		return errors.New("invalid extension metadata resource URL")
	}
	return nil
}

// API metadata must remain on Open VSX. Package redirects keep their existing
// separate download policy; this copy does not mutate a shared HTTP client.
func readCatalogMetadata(ctx context.Context, client *http.Client, address string) (catalogExtension, bool, error) {
	var e catalogExtension
	copy := *client
	previous := client.CheckRedirect
	copy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 || catalogHTTPS(req.URL.String()) != nil {
			return errors.New("invalid Open VSX metadata redirect")
		}
		if previous != nil {
			return previous(req, via)
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return e, false, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "gocode/"+appVersion())
	resp, err := copy.Do(req)
	if err != nil {
		return e, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return e, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return e, false, fmt.Errorf("Open VSX HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return e, false, err
	}
	if len(data) > 1<<20 {
		return e, false, errors.New("catalog response exceeds limit")
	}
	if err = json.Unmarshal(data, &e); err != nil {
		return e, false, err
	}
	return e, true, validCatalogMetadata(e)
}

func fetchCatalogExact(ctx context.Context, client *http.Client, id string) ([]catalogExtension, error) {
	parts := strings.Split(id, ".")
	for _, platform := range []string{"win32-x64", "universal"} {
		address := "https://open-vsx.org/api/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]) + "/" + platform + "/latest"
		latest, found, err := readCatalogMetadata(ctx, client, address)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		if !strings.EqualFold(latest.id(), id) || latest.TargetPlatform != platform {
			return nil, errors.New("extension latest platform/identity mismatch")
		}
		// Freeze the returned version and re-read its immutable metadata. Never
		// offer a mutable latest download URL as the selected installation.
		resolved, err := resolveCatalogWindows(ctx, client, latest)
		if err != nil {
			return nil, err
		}
		return []catalogExtension{resolved}, nil
	}
	return nil, nil // Both named platform endpoints legitimately returned 404.
}
func downloadCatalogVSIX(ctx context.Context, client *http.Client, root string, e catalogExtension) (string, func(), error) {
	var err error
	if e.Gallery != "" {
		copy := *client
		previous := client.CheckRedirect
		copy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) > 5 {
				return errors.New("too many Gallery redirects")
			}
			if err := galleryAssetHTTPS(e.Gallery, req.URL.String()); err != nil {
				return err
			}
			if previous != nil {
				return previous(req, via)
			}
			return nil
		}
		client = &copy
	}
	if e.Gallery == "" {
		e, err = resolveCatalogWindows(ctx, client, e)
		if err != nil {
			return "", nil, err
		}
	}
	if err := catalogResourceHTTPS(e, e.Files.Download); err != nil {
		return "", nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", e.Files.Download, nil)
	if err != nil {
		return "", nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", nil, fmt.Errorf("VSIX download HTTP %d", resp.StatusCode)
	}
	if resp.Request.URL.Scheme != "https" {
		return "", nil, errors.New("VSIX redirect must use HTTPS")
	}
	if resp.ContentLength > 64<<20 {
		return "", nil, errors.New("VSIX exceeds download limit")
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return "", nil, err
	}
	f, err := os.CreateTemp(root, ".download-*.vsix")
	if err != nil {
		return "", nil, err
	}
	name := f.Name()
	cleanup := func() { _ = os.Remove(name) }
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, (64<<20)+1))
	err = errors.Join(err, f.Close())
	if err != nil || n > 64<<20 {
		cleanup()
		return "", nil, errors.Join(err, errors.New("VSIX download failed or exceeded limit"))
	}
	if e.Files.SHA256 != "" {
		if err = catalogResourceHTTPS(e, e.Files.SHA256); err != nil {
			cleanup()
			return "", nil, err
		}
		req, err = http.NewRequestWithContext(ctx, "GET", e.Files.SHA256, nil)
		if err != nil {
			cleanup()
			return "", nil, err
		}
		sum, fetchErr := client.Do(req)
		if fetchErr != nil {
			cleanup()
			return "", nil, fetchErr
		}
		data, readErr := io.ReadAll(io.LimitReader(sum.Body, 1025))
		sum.Body.Close()
		fields := strings.Fields(string(data))
		if readErr != nil || sum.StatusCode != 200 || len(data) > 1024 || len(fields) == 0 || !strings.EqualFold(fields[0], hex.EncodeToString(h.Sum(nil))) {
			cleanup()
			return "", nil, errors.New("VSIX SHA256 verification failed")
		}
	}
	return filepath.Clean(name), cleanup, nil
}

// Search can return another platform's latest file even with targetPlatform.
// Resolve the selected immutable version explicitly; never install that URL.
func resolveCatalogWindows(ctx context.Context, client *http.Client, e catalogExtension) (catalogExtension, error) {
	if !validCatalogID(e.id()) || e.Version == "" || len(e.Version) > 128 || e.Version == "latest" || strings.ContainsAny(e.Version, "/\\\x00 \t\r\n") || e.Version == "." || e.Version == ".." {
		return e, errors.New("invalid selected extension identity/version")
	}
	for _, platform := range []string{"win32-x64", "universal"} {
		address := "https://open-vsx.org/api/" + url.PathEscape(e.Publisher) + "/" + url.PathEscape(e.Name) + "/" + platform + "/" + url.PathEscape(e.Version)
		resolved, found, err := readCatalogMetadata(ctx, client, address)
		if err != nil {
			return e, err
		}
		if !found {
			continue
		}
		if !strings.EqualFold(resolved.id(), e.id()) || resolved.Version != e.Version || resolved.TargetPlatform != platform {
			return e, errors.New("extension platform/identity mismatch")
		}
		return resolved, nil
	}
	return e, errors.New("this extension has no Windows x64 or universal package")
}
func (m *model) startExtensionCatalog(parent context.Context, dispatch func(func()) bool, client *http.Client) func() {
	if client == nil {
		client = newCatalogClient()
	}
	ctx, cancel := context.WithCancel(parent)
	gallery := m.extensionGallery
	var workers sync.WaitGroup
	requests := make(chan func(context.Context), 1)
	var activeCancel context.CancelFunc
	workers.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case work := <-requests:
				work(ctx)
			}
		}
	})
	m.extensionsView.catalog.search = func(query string) {
		if activeCancel != nil {
			activeCancel()
		}
		m.extensionsView.catalog.generation++
		generation := m.extensionsView.catalog.generation
		m.extensionsView.catalog.results = nil
		m.extensionsView.catalog.error = ""
		q, err := parseExtensionQuery(query)
		if err != nil || !q.catalogVisible() {
			m.extensionsView.catalog.busy = false
			if err != nil {
				m.extensionsView.catalog.error = err.Error()
			}
			return
		}
		request, stop := context.WithCancel(ctx)
		activeCancel = stop
		m.extensionsView.catalog.busy = true
		work := func(context.Context) {
			timer := time.NewTimer(300 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-request.Done():
				return
			case <-timer.C:
			}
			c, end := context.WithTimeout(request, 15*time.Second)
			defer end()
			var result []catalogExtension
			var err error
			if gallery != "" {
				result, err = fetchGallery(c, client, gallery, query)
			} else {
				result, err = fetchCatalog(c, client, query)
			}
			uidispatch.Retry(request, dispatch, func() {
				if request.Err() != nil || generation != m.extensionsView.catalog.generation {
					return
				}
				m.extensionsView.catalog.busy = false
				if err != nil {
					m.extensionsView.catalog.error = err.Error()
				} else {
					m.extensionsView.catalog.results = result
				}
			})
		}
		select {
		case requests <- work:
		default:
			select {
			case <-requests:
			default:
			}
			select {
			case requests <- work:
			case <-ctx.Done():
			}
		}
	}
	return func() {
		cancel()
		if activeCancel != nil {
			activeCancel()
		}
		workers.Wait()
	}
}
func newCatalogClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 || req.URL.Scheme != "https" {
			return errors.New("invalid catalog redirect")
		}
		return nil
	}}
}
func checkLiveExtensionCatalog(query string) error {
	if len(query) > 1024 {
		return errors.New("extension query exceeds limit")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := newCatalogClient()
	items, err := fetchCatalog(ctx, client, query)
	if err != nil {
		return err
	}
	var resolved []catalogExtension
	for _, e := range items {
		item, err := resolveCatalogWindows(ctx, client, e)
		if err == nil {
			resolved = append(resolved, item)
			if len(resolved) == 3 {
				break
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	if len(resolved) == 0 {
		return errors.New("no actual Windows x64/universal extension versions resolved")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"catalog": "Open VSX", "query": query, "target": "win32-x64", "installed": false, "extensions": resolved})
}
func validateCatalogVSIX(path string, e catalogExtension) error {
	z, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer z.Close()
	if len(z.File) > 4096 {
		return errors.New("VSIX entry limit exceeded")
	}
	for _, file := range z.File {
		if file.Name == "extension/package.json" {
			if file.UncompressedSize64 > 256<<10 {
				return errors.New("VSIX manifest exceeds limit")
			}
			r, err := file.Open()
			if err != nil {
				return err
			}
			data, err := io.ReadAll(io.LimitReader(r, (256<<10)+1))
			r.Close()
			if err != nil {
				return err
			}
			if len(data) > 256<<10 {
				return errors.New("VSIX manifest exceeds limit")
			}
			var m struct{ Publisher, Name, Version string }
			if err = json.Unmarshal(data, &m); err != nil {
				return err
			}
			if !strings.EqualFold(m.Publisher+"."+m.Name, e.id()) || m.Version != e.Version {
				return errors.New("VSIX manifest differs from selected catalog identity/version")
			}
			return nil
		}
	}
	return errors.New("VSIX manifest is missing")
}
func (m *model) extensionQueryChanged() {
	m.extensionsView.scroll = 0
	if m.extensionsView.catalog.search != nil {
		m.extensionsView.catalog.search(m.extensionsView.query)
	}
}
