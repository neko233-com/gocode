package update

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gocode/internal/installlayout"
)

type Config struct {
	Auto         bool     `json:"auto"`
	Mode         string   `json:"mode"` // auto, direct, mirror
	Mirror       string   `json:"mirror,omitempty"`
	Mirrors      []string `json:"mirrors,omitempty"`
	ManifestURLs []string `json:"manifestURLs,omitempty"`
}

func DefaultConfig() Config {
	return Config{Auto: true, Mode: "auto", Mirrors: []string{"https://ghfast.top/", "https://gh-proxy.com/"}, ManifestURLs: []string{"https://github.com/neko233-com/gocode/releases/latest/download/update-manifest.json", "https://cdn.jsdelivr.net/gh/neko233-com/gocode@main/distribution/update-manifest.json"}}
}

type Manager struct {
	HTTP       *http.Client
	Key        ed25519.PublicKey
	Config     Config
	Repository string
}
type Candidate struct {
	Manifest                Manifest
	Envelope                []byte
	MetadataURL             string
	GitHubMetadataReachable bool
}
type Route struct {
	URL     string
	Direct  bool
	Latency time.Duration
}
type Result struct {
	Version, MetadataURL, DownloadURL string
	GitHubReachable, Ready            bool
}

func validURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return errors.New("invalid update URL")
	}
	if u.Scheme == "https" {
		return nil
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme == "http" && (u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())) {
		return nil
	}
	return errors.New("update endpoints must use HTTPS (except an explicit loopback service)")
}
func (m *Manager) client() *http.Client {
	if m.HTTP != nil {
		return m.HTTP
	}
	return &http.Client{Timeout: 2 * time.Minute}
}
func (m *Manager) validate() error {
	if len(m.Key) != ed25519.PublicKeySize {
		return errors.New("publisher public key is missing")
	}
	if m.Config.Mode != "auto" && m.Config.Mode != "direct" && m.Config.Mode != "mirror" {
		return errors.New("update mode must be auto, direct or mirror")
	}
	if len(m.Config.ManifestURLs) == 0 || len(m.Config.ManifestURLs) > 8 || len(m.Config.Mirrors) > 8 {
		return errors.New("invalid update endpoint count")
	}
	for _, raw := range append(append([]string{}, m.Config.ManifestURLs...), m.Config.Mirrors...) {
		if err := validURL(raw); err != nil {
			return err
		}
	}
	if m.Config.Mirror != "" {
		if err := validURL(m.Config.Mirror); err != nil {
			return err
		}
	}
	if m.Config.Mode == "mirror" && m.Config.Mirror == "" {
		return errors.New("manual mirror mode needs a mirror URL")
	}
	return nil
}
func (m *Manager) request(ctx context.Context, raw string, probe bool) (*http.Response, error) {
	if err := validURL(raw); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gocode-updater/1")
	req.Header.Set("Accept-Encoding", "identity")
	if probe {
		req.Header.Set("Range", "bytes=0-0")
	}
	response, err := m.client().Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK && (!probe || response.StatusCode != http.StatusPartialContent) {
		response.Body.Close()
		return nil, fmt.Errorf("update endpoint returned HTTP %d", response.StatusCode)
	}
	return response, nil
}
func mirrorURL(prefix, source string) string { return strings.TrimRight(prefix, "/") + "/" + source }

// Check accepts metadata from any configured route only after its publisher
// signature succeeds. An untrusted mirror cannot replace both checksum and app.
func (m *Manager) Check(ctx context.Context) (Candidate, error) {
	if err := m.validate(); err != nil {
		return Candidate{}, err
	}
	var failures []error
	for _, source := range m.Config.ManifestURLs {
		urls := []string{source}
		if strings.HasPrefix(source, "https://github.com/") {
			if m.Config.Mode == "mirror" {
				urls = []string{mirrorURL(m.Config.Mirror, source)}
			} else if m.Config.Mode == "auto" {
				if m.Config.Mirror != "" {
					urls = append(urls, mirrorURL(m.Config.Mirror, source))
				}
				for _, mirror := range m.Config.Mirrors {
					urls = append(urls, mirrorURL(mirror, source))
				}
			}
		}
		for _, raw := range urls {
			c, stop := context.WithTimeout(ctx, 5*time.Second)
			response, err := m.request(c, raw, false)
			if err != nil {
				stop()
				failures = append(failures, err)
				continue
			}
			data, err := io.ReadAll(io.LimitReader(response.Body, MaxManifestBytes+1))
			closeErr := response.Body.Close()
			stop()
			if err = errors.Join(err, closeErr); err != nil {
				failures = append(failures, err)
				continue
			}
			manifest, err := Verify(data, m.Key)
			if err != nil {
				failures = append(failures, err)
				continue
			}
			return Candidate{Manifest: manifest, Envelope: data, MetadataURL: raw, GitHubMetadataReachable: strings.HasPrefix(raw, "https://github.com/")}, nil
		}
	}
	return Candidate{}, fmt.Errorf("no verified release metadata: %w", errors.Join(failures...))
}

// Routes probes a bounded set concurrently and orders usable routes by latency.
// Manual mode uses exactly the chosen prefix; direct mode never uses a mirror.
func (m *Manager) Routes(ctx context.Context, source string) ([]Route, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	if err := validURL(source); err != nil {
		return nil, err
	}
	urls := []Route{{URL: source, Direct: true}}
	if m.Config.Mode == "mirror" {
		urls = []Route{{URL: mirrorURL(m.Config.Mirror, source)}}
	} else if m.Config.Mode == "auto" {
		if m.Config.Mirror != "" {
			urls = append(urls, Route{URL: mirrorURL(m.Config.Mirror, source)})
		}
		for _, prefix := range m.Config.Mirrors {
			urls = append(urls, Route{URL: mirrorURL(prefix, source)})
		}
	}
	var workers sync.WaitGroup
	results := make(chan Route, len(urls))
	seen := map[string]bool{}
	for _, route := range urls {
		if seen[route.URL] {
			continue
		}
		seen[route.URL] = true
		route := route
		workers.Go(func() {
			c, stop := context.WithTimeout(ctx, 3*time.Second)
			defer stop()
			start := time.Now()
			response, err := m.request(c, route.URL, true)
			if err != nil {
				return
			}
			_, err = io.CopyN(io.Discard, response.Body, 1)
			response.Body.Close()
			if err != nil {
				return
			}
			route.Latency = time.Since(start)
			results <- route
		})
	}
	workers.Wait()
	close(results)
	var routes []Route
	for route := range results {
		routes = append(routes, route)
	}
	sort.SliceStable(routes, func(i, j int) bool { return routes[i].Latency < routes[j].Latency })
	if len(routes) == 0 {
		return nil, errors.New("no reachable download route")
	}
	// Keep a comparably fast direct route first. Mirrors are still probed and
	// available for fallback; their bytes always require publisher verification.
	for i, route := range routes {
		if route.Direct && route.Latency <= routes[0].Latency+100*time.Millisecond {
			copy(routes[1:i+1], routes[:i])
			routes[0] = route
			break
		}
	}
	return routes, nil
}
func (m *Manager) Download(ctx context.Context, root string, asset Asset, routes []Route) (string, Route, error) {
	if _, err := installlayout.ValidateRoot(root); err != nil {
		return "", Route{}, err
	}
	var failures []error
	for _, route := range routes {
		response, err := m.request(ctx, route.URL, false)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if response.ContentLength > 0 && response.ContentLength != asset.Size {
			response.Body.Close()
			failures = append(failures, errors.New("download Content-Length mismatch"))
			continue
		}
		f, err := os.CreateTemp(root, ".gocode-download-")
		if err != nil {
			response.Body.Close()
			return "", Route{}, err
		}
		name := f.Name()
		n, copyErr := copyContext(ctx, f, response.Body, asset.Size+1)
		err = errors.Join(copyErr, response.Body.Close(), f.Sync(), f.Close())
		if err != nil || n != asset.Size {
			_ = os.Remove(name)
			failures = append(failures, errors.Join(err, errors.New("download size mismatch")))
			continue
		}
		// A successful HTTP route can still be a tampered mirror. Verify the hash
		// here so an alternate route can be tried without activating any bytes.
		if err := verifyFileHash(ctx, name, asset); err != nil {
			_ = os.Remove(name)
			failures = append(failures, err)
			continue
		}
		return name, route, nil
	}
	return "", Route{}, fmt.Errorf("all downloads failed integrity/availability checks: %w", errors.Join(failures...))
}
func (m *Manager) AssetURL(candidate Candidate, asset Asset) (string, error) {
	repo := m.Repository
	if repo == "" {
		repo = "neko233-com/gocode"
	}
	if repo != "neko233-com/gocode" {
		return "", errors.New("unexpected release repository")
	}
	return "https://github.com/" + repo + "/releases/download/v" + candidate.Manifest.Version + "/" + asset.Name, nil
}
func (m *Manager) Prepare(ctx context.Context, root, platform string) (Result, error) {
	_, active, err := installlayout.Resolve(root, false)
	if err != nil {
		return Result{}, err
	}
	candidate, err := m.Check(ctx)
	if err != nil {
		return Result{}, err
	}
	result := Result{Version: candidate.Manifest.Version, MetadataURL: candidate.MetadataURL, GitHubReachable: candidate.GitHubMetadataReachable}
	if installlayout.CompareVersion(candidate.Manifest.Version, active.Version) <= 0 {
		return result, nil
	}
	asset, err := candidate.Manifest.Asset(platform)
	if err != nil {
		return result, err
	}
	source, err := m.AssetURL(candidate, asset)
	if err != nil {
		return result, err
	}
	routes, err := m.Routes(ctx, source)
	if err != nil {
		return result, err
	}
	for _, route := range routes {
		if route.Direct {
			result.GitHubReachable = true
		}
	}
	archive, route, err := m.Download(ctx, root, asset, routes)
	if err != nil {
		return result, err
	}
	defer os.Remove(archive)
	manifest, err := Stage(ctx, root, archive, candidate.Envelope, m.Key, platform)
	if err != nil {
		return result, err
	}
	result.DownloadURL = route.URL
	// Activation follows a bounded executable health check in the application
	// integration. Prepare deliberately leaves the running/next pointer unchanged.
	_ = manifest
	result.Ready = true
	return result, nil
}
func StagedManifest(root, version string, key ed25519.PublicKey) (installlayout.Manifest, error) {
	if !installlayout.ValidVersion(version) {
		return installlayout.Manifest{}, errors.New("invalid version")
	}
	if _, err := installlayout.ValidateRoot(root); err != nil {
		return installlayout.Manifest{}, err
	}
	if err := checkDirectory(filepath.Join(root, "versions")); err != nil {
		return installlayout.Manifest{}, err
	}
	if err := checkDirectory(filepath.Join(root, "versions", version)); err != nil {
		return installlayout.Manifest{}, err
	}
	file := filepath.Join(root, "versions", version, "update-manifest.json")
	info, err := os.Lstat(file)
	if err != nil {
		return installlayout.Manifest{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxManifestBytes {
		return installlayout.Manifest{}, errors.New("invalid staged metadata file")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return installlayout.Manifest{}, err
	}
	m, err := Verify(data, key)
	if err != nil {
		return installlayout.Manifest{}, err
	}
	if m.Version != version {
		return installlayout.Manifest{}, errors.New("staged manifest version mismatch")
	}
	return installlayout.Manifest{Owner: m.Owner, Schema: m.Layout, Version: m.Version, Source: m.Source}, nil
}
