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

func catalogHTTPS(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() != "open-vsx.org" || u.User != nil {
		return errors.New("invalid Open VSX resource URL")
	}
	return nil
}
func fetchCatalog(ctx context.Context, client *http.Client, query string) ([]catalogExtension, error) {
	values := url.Values{"query": {query}, "size": {"20"}, "targetPlatform": {"win32-x64"}}
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
	for _, platform := range []string{"win32-x64", "universal"} {
		address := "https://open-vsx.org/api/" + url.PathEscape(e.Publisher) + "/" + url.PathEscape(e.Name) + "/" + platform + "/" + url.PathEscape(e.Version)
		req, err := http.NewRequestWithContext(ctx, "GET", address, nil)
		if err != nil {
			return e, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return e, err
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
		resp.Body.Close()
		if resp.StatusCode == 404 {
			continue
		}
		if err != nil {
			return e, err
		}
		if resp.StatusCode != 200 || len(data) > 1<<20 {
			return e, errors.New("invalid extension version metadata")
		}
		var resolved catalogExtension
		if err = json.Unmarshal(data, &resolved); err != nil {
			return e, err
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
		if strings.TrimSpace(query) == "" || strings.Contains(query, "@") {
			m.extensionsView.catalog.busy = false
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
			uidispatch.Retry(ctx, dispatch, func() {
				if ctx.Err() != nil || generation != m.extensionsView.catalog.generation {
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
