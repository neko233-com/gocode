package languageextension

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
	"sort"
	"strings"
	"sync"

	"github.com/neko233-com/godesktop/extensions"
)

var installMu sync.Mutex

type File struct {
	Path, SHA256 string
	Bytes        int64
}
type Receipt struct {
	Kind          string `json:"kind"`
	ID            string `json:"id"`
	Version       string `json:"version"`
	ArchiveSHA256 string `json:"archive_sha256"`
	Files         []File `json:"files"`
	Reused        bool   `json:"reused"`
}

func receiptPath(root, kind string) string {
	return filepath.Join(root, ".gocode-language", kind+".json")
}

func inventory(files []File) string {
	copy := append([]File(nil), files...)
	sort.Slice(copy, func(i, j int) bool { return filepath.ToSlash(copy[i].Path) < filepath.ToSlash(copy[j].Path) })
	h := sha256.New()
	for _, file := range copy {
		fmt.Fprintf(h, "%s\x00%d\x00%s\n", filepath.ToSlash(file.Path), file.Bytes, file.SHA256)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func expectedInventory(kind string) string {
	if kind == "go" {
		return "295f83601de947c1e057cb1de787e361e769f2185f59f22f3fba89dbb82852c9"
	}
	return "89806dd3685da5012779fec123fa12310f3613077ebba5355de6625435254cb6"
}

// regular opens only direct regular files under a selected, non-symlink root.
func regular(root, name string, limit int64) (*os.File, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if !filepath.IsLocal(name) {
		return nil, errors.New("non-local language package path")
	}
	path := root
	parts := append([]string{""}, strings.Split(filepath.Clean(name), string(os.PathSeparator))...)
	for i, part := range parts {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || i < len(parts)-1 && !info.IsDir() || i == len(parts)-1 && (!info.Mode().IsRegular() || info.Size() > limit) {
			return nil, errors.New("language package path is linked, special or exceeds limit")
		}
	}
	return os.Open(path)
}

func hashFile(root, name string, limit int64) (File, error) {
	r, err := regular(root, name, limit)
	if err != nil {
		return File{}, err
	}
	defer r.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(r, limit+1))
	if err != nil || n > limit {
		return File{}, errors.Join(err, errors.New("language package file exceeds limit"))
	}
	return File{name, hex.EncodeToString(h.Sum(nil)), n}, nil
}

func readJSON(root, name string, limit int64, out any) error {
	r, err := regular(root, name, limit)
	if err != nil {
		return err
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > limit {
		return errors.New("language receipt exceeds limit")
	}
	return json.Unmarshal(data, out)
}

func installedPath(root string, pin Pin) string { return filepath.Join(root, pin.ID+"-"+pin.Version) }

func VerifyInstalled(root, kind string) (Receipt, error) {
	var receipt Receipt
	pin, err := Package(kind)
	if err != nil {
		return receipt, err
	}
	if err = readJSON(root, filepath.Join(".gocode-language", kind+".json"), 1<<20, &receipt); err != nil {
		return receipt, fmt.Errorf("install %s language support to create a verified native adapter: %w", kind, err)
	}
	if receipt.Kind != kind {
		return Receipt{}, errors.New("language installation kind mismatch")
	}
	if err := ValidateReceipt(receipt); err != nil {
		return Receipt{}, err
	}
	for _, expected := range receipt.Files {
		actual, err := hashFile(installedPath(root, pin), expected.Path, 16<<20)
		if err != nil || actual != expected {
			return Receipt{}, errors.Join(err, fmt.Errorf("installed language package changed: %s", expected.Path))
		}
	}
	return receipt, nil
}

// ValidateReceipt checks the compiled, hash-pinned complete upstream inventory.
// It is also used to verify retained observations after an owned stage is gone;
// only VerifyInstalled observes the actual on-disk package bytes.
func ValidateReceipt(receipt Receipt) error {
	pin, err := Package(receipt.Kind)
	if err != nil {
		return err
	}
	if receipt.ID != pin.ID || receipt.Version != pin.Version || receipt.ArchiveSHA256 != pin.SHA256 || len(receipt.Files) == 0 || len(receipt.Files) > 4096 {
		return errors.New("language installation receipt identity mismatch")
	}
	if inventory(receipt.Files) != expectedInventory(receipt.Kind) {
		return errors.New("language receipt differs from pinned complete file inventory")
	}
	seen := map[string]bool{}
	var total int64
	for _, expected := range receipt.Files {
		if !filepath.IsLocal(expected.Path) || seen[strings.ToLower(expected.Path)] || expected.Bytes < 0 || expected.Bytes > 16<<20 || len(expected.SHA256) != 64 {
			return errors.New("invalid language installation file receipt")
		}
		seen[strings.ToLower(expected.Path)] = true
		total += expected.Bytes
		if total > 64<<20 {
			return errors.New("language installation exceeds extraction limit")
		}
	}
	if !seen["package.json"] {
		return errors.New("language receipt has no manifest")
	}
	return nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".receipt-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	err = errors.Join(func() error { _, e := f.Write(append(data, '\n')); return e }(), f.Close())
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

// InstallArchive accepts the original, hash-pinned public VSIX only. The public
// framework's 256 KiB manifest/64 MiB extraction guards remain unchanged.
func InstallArchive(ctx context.Context, root, tools, kind, archive string) (Receipt, error) {
	installMu.Lock()
	defer installMu.Unlock()
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	pin, err := Package(kind)
	if err != nil {
		return Receipt{}, err
	}
	archive, err = filepath.Abs(archive)
	if err != nil {
		return Receipt{}, err
	}
	sum, err := hashFile(filepath.Dir(archive), filepath.Base(archive), 64<<20)
	if err != nil || sum.SHA256 != pin.SHA256 {
		return Receipt{}, errors.Join(err, errors.New("language VSIX differs from pinned original bytes"))
	}
	if receipt, err := VerifyInstalled(root, kind); err == nil {
		if _, err = EnsureRuntime(ctx, tools, kind); err != nil {
			return Receipt{}, err
		}
		receipt.Reused = true
		return receipt, nil
	}
	z, err := zip.OpenReader(archive)
	if err != nil {
		return Receipt{}, err
	}
	defer z.Close()
	receipt := Receipt{Kind: kind, ID: pin.ID, Version: pin.Version, ArchiveSHA256: pin.SHA256}
	for _, file := range z.File {
		if !strings.HasPrefix(file.Name, "extension/") || file.FileInfo().IsDir() {
			continue
		}
		name := filepath.FromSlash(strings.TrimPrefix(file.Name, "extension/"))
		if !filepath.IsLocal(name) || file.UncompressedSize64 > 16<<20 || len(receipt.Files) >= 4096 {
			return Receipt{}, errors.New("invalid pinned language archive")
		}
		r, err := file.Open()
		if err != nil {
			return Receipt{}, err
		}
		h := sha256.New()
		n, e := io.Copy(h, io.LimitReader(r, (16<<20)+1))
		e = errors.Join(e, r.Close())
		if e != nil || n > 16<<20 {
			return Receipt{}, errors.Join(e, errors.New("pinned language file exceeds limit"))
		}
		receipt.Files = append(receipt.Files, File{name, hex.EncodeToString(h.Sum(nil)), n})
	}
	if inventory(receipt.Files) != expectedInventory(kind) {
		return Receipt{}, errors.New("pinned language archive inventory mismatch")
	}
	if _, err = EnsureRuntime(ctx, tools, kind); err != nil {
		return Receipt{}, err
	}
	path := installedPath(root, pin)
	if _, err = os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		e, err := extensions.Install(root, archive)
		if err != nil {
			return Receipt{}, err
		}
		if !strings.EqualFold(e.ID(), pin.ID) || e.Manifest.Version != pin.Version {
			return Receipt{}, errors.New("installed language archive identity changed")
		}
	} else if err != nil {
		return Receipt{}, err
	}
	// Existing versions are reused only if every original archive byte matches.
	for _, file := range receipt.Files {
		actual, err := hashFile(path, file.Path, 16<<20)
		if err != nil || actual != file {
			return Receipt{}, errors.Join(err, errors.New("existing language installation differs; no files replaced"))
		}
	}
	if err = ctx.Err(); err != nil {
		return Receipt{}, err
	}
	if err = writeJSON(receiptPath(root, kind), receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func downloadAllowed(u *url.URL) bool {
	return u.Scheme == "https" && u.User == nil && (u.Hostname() == "open-vsx.org" || u.Hostname() == "openvsx.eclipsecontent.org") && (u.Port() == "" || u.Port() == "443")
}

// Install is idempotent before network access. Downloads and dependencies use
// workers/finite contexts; it never changes global npm or user LSP settings.
func Install(ctx context.Context, client *http.Client, root, tools, kind string) (Receipt, error) {
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	if receipt, err := VerifyInstalled(root, kind); err == nil {
		if _, err = EnsureRuntime(ctx, tools, kind); err != nil {
			return Receipt{}, err
		}
		receipt.Reused = true
		return receipt, nil
	}
	pin, err := Package(kind)
	if err != nil {
		return Receipt{}, err
	}
	copy := *client
	copy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 || !downloadAllowed(req.URL) {
			return errors.New("invalid pinned language VSIX redirect")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, "GET", pin.URL, nil)
	if err != nil {
		return Receipt{}, err
	}
	response, err := copy.Do(req)
	if err != nil {
		return Receipt{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > 64<<20 {
		return Receipt{}, errors.New("pinned language VSIX download failed or exceeds limit")
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return Receipt{}, err
	}
	f, err := os.CreateTemp(root, ".language-download-*.vsix")
	if err != nil {
		return Receipt{}, err
	}
	defer os.Remove(f.Name())
	n, err := io.Copy(f, io.LimitReader(response.Body, (64<<20)+1))
	err = errors.Join(err, f.Close())
	if err != nil || n > 64<<20 {
		return Receipt{}, errors.Join(err, errors.New("pinned language VSIX exceeds limit"))
	}
	return InstallArchive(ctx, root, tools, kind, f.Name())
}
