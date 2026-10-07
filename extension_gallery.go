package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Native VS Gallery protocol, derived from Code-OSS 1.141.0 (MIT).
// Microsoft Marketplace access requires its own service authorization; this
// adapter also supports authorized/private compatible galleries. No VS Code
// product identity or Microsoft client credential is impersonated.
func galleryURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("VS Gallery endpoint must be an HTTPS base URL")
	}
	return u, nil
}
func galleryAssetHTTPS(base, raw string) error {
	g, err := galleryURL(base)
	if err != nil {
		return err
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || len(raw) > 8192 {
		return errors.New("invalid Gallery asset URL")
	}
	if strings.EqualFold(u.Host, g.Host) {
		return nil
	}
	if g.Hostname() == "marketplace.visualstudio.com" && (strings.HasSuffix(u.Hostname(), ".gallery.vsassets.io") || strings.HasSuffix(u.Hostname(), ".gallerycdn.vsassets.io")) {
		return nil
	}
	return errors.New("Gallery asset host differs from the authorized gallery")
}
func catalogResourceHTTPS(e catalogExtension, raw string) error {
	if e.Gallery != "" {
		return galleryAssetHTTPS(e.Gallery, raw)
	}
	return catalogHTTPS(raw)
}

type galleryVersion struct {
	Version, TargetPlatform, AssetURI string
	Files                             []struct{ AssetType, Source string }
	Properties                        []struct{ Key, Value string }
}
type galleryEntry struct {
	ExtensionName, DisplayName, ShortDescription string
	Publisher                                    struct{ PublisherName string }
	Versions                                     []galleryVersion
}

func fetchGallery(ctx context.Context, client *http.Client, base, query string) ([]catalogExtension, error) {
	if _, err := galleryURL(base); err != nil {
		return nil, err
	}
	if len(query) > 1024 {
		return nil, errors.New("Gallery query exceeds limit")
	}
	// Source manifest values: Target=8, SearchText=10; versions/files/
	// properties/asset URI/latest platform versions = 1|2|16|128|512.
	body, err := json.Marshal(map[string]any{"filters": []any{map[string]any{"criteria": []any{map[string]any{"filterType": 8, "value": "Microsoft.VisualStudio.Code"}, map[string]any{"filterType": 10, "value": query}}, "pageNumber": 1, "pageSize": 20, "sortBy": 0, "sortOrder": 0}}, "assetTypes": []string{"Microsoft.VisualStudio.Services.VSIXPackage"}, "flags": 1 | 2 | 16 | 128 | 512})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(base, "/")+"/extensionquery", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json;api-version=3.0-preview.1")
	req.Header.Set("User-Agent", "gocode/"+appVersion())
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("VS Gallery HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, errors.New("Gallery response exceeds 1 MiB")
	}
	var result struct {
		Results []struct{ Extensions []galleryEntry }
	}
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if len(result.Results) != 1 || len(result.Results[0].Extensions) > 20 {
		return nil, errors.New("invalid Gallery result count")
	}
	var items []catalogExtension
	for _, entry := range result.Results[0].Extensions {
		if len(entry.Versions) > 128 || len(entry.Publisher.PublisherName) > 100 || len(entry.ExtensionName) > 100 || len(entry.DisplayName) > 1024 || len(entry.ShortDescription) > 16<<10 {
			return nil, errors.New("Gallery entry exceeds bounds")
		}
		var chosen *galleryVersion
		for i := range entry.Versions {
			v := &entry.Versions[i]
			if chosen != nil && chosen.Version != v.Version {
				break
			}
			preRelease := false
			for _, p := range v.Properties {
				if p.Key == "Microsoft.VisualStudio.Code.PreRelease" && strings.EqualFold(p.Value, "true") {
					preRelease = true
				}
			}
			if preRelease || v.Version == "" || len(v.Version) > 128 {
				continue
			}
			if v.TargetPlatform == "win32-x64" {
				chosen = v
				break
			}
			if chosen == nil && (v.TargetPlatform == "" || v.TargetPlatform == "universal") {
				chosen = v
			}
		}
		if chosen == nil {
			continue
		}
		e := catalogExtension{Name: entry.ExtensionName, Publisher: entry.Publisher.PublisherName, DisplayName: entry.DisplayName, Description: entry.ShortDescription, Version: chosen.Version, TargetPlatform: chosen.TargetPlatform, Gallery: base}
		if e.Name == "" || e.Publisher == "" {
			return nil, errors.New("Gallery extension identity missing")
		}
		for _, f := range chosen.Files {
			if f.AssetType == "Microsoft.VisualStudio.Services.VSIXPackage" {
				e.Files.Download = f.Source
				break
			}
		}
		if e.Files.Download == "" {
			e.Files.Download = strings.TrimRight(chosen.AssetURI, "/") + "/Microsoft.VisualStudio.Services.VSIXPackage"
		}
		if chosen.TargetPlatform != "" {
			u, parseErr := url.Parse(e.Files.Download)
			if parseErr != nil {
				return nil, parseErr
			}
			q := u.Query()
			q.Set("targetPlatform", chosen.TargetPlatform)
			u.RawQuery = q.Encode()
			e.Files.Download = u.String()
		}
		if err = galleryAssetHTTPS(base, e.Files.Download); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, nil
}
func (m *model) extensionStoreName() string {
	if m.extensionGallery != "" {
		if u, err := galleryURL(m.extensionGallery); err == nil && u.Hostname() == "marketplace.visualstudio.com" {
			return "Visual Studio Marketplace"
		}
		return "VS Gallery"
	}
	return "Open VSX"
}
