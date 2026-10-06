package terminal

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const ConPTYVersion = "1.25.260930003"
const ConPTYPackageSHA = "6d6f8b008c655d814c498d8205d4012dcd4d0b6dfc1096461d0a185d1e3caf03"
const conPTYLicense = `Copyright (c) Microsoft Corporation. All rights reserved.

MIT License

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED *AS IS*, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`

type runtimeFile struct {
	entry, name, sha string
	size             int64
}

var conPTYFiles = []runtimeFile{
	{"runtimes/win-x64/native/conpty.dll", "conpty.dll", "feeef341d891643c62d30b6b07800bc70f0bc148f44cb8c3bee84aa557ae805a", 119608},
	{"build/native/runtimes/x64/OpenConsole.exe", "OpenConsole.exe", "3d66b23d0a71bb8eed2b77edc8b9df9bf54ce6c8fb74c863a30e760997f80586", 1089376},
}
var runtimeInstallLock = make(chan struct{}, 1)

func VerifyConPTY(directory string) error {
	for _, file := range conPTYFiles {
		path := filepath.Join(directory, file.name)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() != file.size {
			return errors.New("invalid native ConPTY file size/type")
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, err = io.CopyN(hash, f, file.size)
		err = errors.Join(err, f.Close())
		if err != nil {
			return err
		}
		if hex.EncodeToString(hash.Sum(nil)) != file.sha {
			return fmt.Errorf("ConPTY integrity mismatch: %s", file.name)
		}
	}
	return nil
}

func VerifyEmbeddedConPTY() error {
	files, err := embeddedConPTY()
	if err != nil {
		return err
	}
	if len(files) != len(conPTYFiles) {
		return errors.New("official ConPTY resources missing from executable")
	}
	return nil
}

// EnsureConPTY installs Microsoft's pinned redistributable in an owned folder.
// Installers bundle it beside the app; development builds have a verified cache.
// No system conhost/DLL, registry, user shell profile or OS component is changed.
func EnsureConPTY(ctx context.Context, destination string) (string, error) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return "", errors.New("native ConPTY redistribution targets Windows amd64")
	}
	if destination == "" {
		if explicit := os.Getenv("GOCODE_CONPTY_DIR"); explicit != "" {
			path, err := filepath.Abs(explicit)
			if err != nil {
				return "", err
			}
			return path, VerifyConPTY(path)
		}
		if exe, err := os.Executable(); err == nil {
			path := filepath.Join(filepath.Dir(exe), "conpty")
			if _, err := os.Stat(path); err == nil {
				return path, VerifyConPTY(path)
			}
		}
		cache, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		destination = filepath.Join(cache, "gocode", "conpty", ConPTYVersion)
	}
	destination, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	select {
	case runtimeInstallLock <- struct{}{}:
		defer func() { <-runtimeInstallLock }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if _, err := os.Stat(destination); err == nil {
		if err := VerifyConPTY(destination); err != nil {
			return "", err
		}
		return destination, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(filepath.Dir(destination), ".conpty-stage-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	files, err := embeddedConPTY()
	if err != nil {
		return "", err
	}
	if files != nil {
		for name, data := range files {
			if err := os.WriteFile(filepath.Join(stage, name), data, 0600); err != nil {
				return "", err
			}
		}
		if err := finishConPTY(stage, destination); err != nil {
			return "", err
		}
		return destination, nil
	}
	urls := []string{"https://github.com/microsoft/terminal/releases/download/v1.25.2733.0/Microsoft.Windows.Console.ConPTY." + ConPTYVersion + ".nupkg", "https://api.nuget.org/v3-flatcontainer/microsoft.windows.console.conpty/" + ConPTYVersion + "/microsoft.windows.console.conpty." + ConPTYVersion + ".nupkg"}
	var failures []error
	for _, url := range urls {
		if err := fetchConPTY(ctx, url, stage); err != nil {
			failures = append(failures, err)
			continue
		}
		if err := VerifyConPTY(stage); err != nil {
			failures = append(failures, err)
			continue
		}
		if err := finishConPTY(stage, destination); err != nil {
			return "", err
		}
		return destination, nil
	}
	return "", errors.Join(failures...)
}

func finishConPTY(stage, destination string) error {
	if err := VerifyConPTY(stage); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "LICENSE.txt"), []byte(conPTYLicense), 0600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "gocode-conpty.json"), []byte(`{"owner":"neko233-com/gocode","version":"`+ConPTYVersion+`"}`), 0600); err != nil {
		return err
	}
	if err := os.Rename(stage, destination); err != nil {
		if verified := VerifyConPTY(destination); verified != nil {
			return err
		}
	}
	return nil
}
func fetchConPTY(parent context.Context, url, stage string) error {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := http.Client{CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if r.URL.Scheme != "https" || len(via) > 5 {
			return errors.New("invalid ConPTY redirect")
		}
		return nil
	}}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("ConPTY download HTTP %d", response.StatusCode)
	}
	archive := filepath.Join(stage, "runtime.nupkg")
	f, err := os.Create(archive)
	if err != nil {
		return err
	}
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, hash), io.LimitReader(response.Body, (8<<20)+1))
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	defer os.Remove(archive)
	if n > 8<<20 {
		return errors.New("ConPTY package exceeds limit")
	}
	if strings.Contains(url, "github.com") && hex.EncodeToString(hash.Sum(nil)) != ConPTYPackageSHA {
		return errors.New("ConPTY package SHA256 mismatch")
	}
	z, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer z.Close()
	for _, expected := range conPTYFiles {
		var found *zip.File
		for _, entry := range z.File {
			if entry.Name == expected.entry {
				if found != nil {
					return errors.New("duplicate native ConPTY archive entry")
				}
				found = entry
			}
		}
		if found == nil || found.UncompressedSize64 != uint64(expected.size) || !found.Mode().IsRegular() {
			return errors.New("invalid ConPTY package entry")
		}
		r, err := found.Open()
		if err != nil {
			return err
		}
		w, err := os.Create(filepath.Join(stage, expected.name))
		if err != nil {
			r.Close()
			return err
		}
		_, err = io.CopyN(w, r, expected.size)
		err = errors.Join(err, r.Close(), w.Close())
		if err != nil {
			return err
		}
	}
	// NuGet may repository-sign its archive. Each native file remains pinned to
	// the same exact official release bytes, independently of archive signing.
	return VerifyConPTY(stage)
}
