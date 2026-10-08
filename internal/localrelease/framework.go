package localrelease

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neko233-com/gocode/internal/nativeguard"
)

type Framework struct {
	Path     string `json:"path"`
	Version  string `json:"version"`
	Source   string `json:"source"`
	Sum      string `json:"sum"`
	GoModSum string `json:"goModSum"`
	Info     File   `json:"info"`
	Module   File   `json:"module"`
	Zip      File   `json:"zip"`
}

// CheckFramework queries the actual consumer graph, cache Origin and module
// verification; a receipt's merely claimed version/hash is never sufficient.
func CheckFramework(ctx context.Context, project, version, source string) (Framework, error) {
	var result Framework
	environment := []string{}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(name, "GOWORK") && !strings.EqualFold(name, "GOFLAGS") {
			environment = append(environment, entry)
		}
	}
	environment = append(environment, "GOWORK=off", "GOFLAGS=")
	query, err := nativeguard.Run(ctx, nativeguard.Options{Command: []string{"go", "list", "-m", "-json", "github.com/neko233-com/godesktop"}, Directory: project, Environment: environment, Timeout: 60 * time.Second})
	if err != nil {
		return result, errors.New("actual published framework graph query failed")
	}
	var module struct {
		Path, Version, Dir, GoMod, Sum, GoModSum string
		Replace                                  json.RawMessage
	}
	if err := json.Unmarshal(query.Stdout, &module); err != nil {
		return result, err
	}
	if module.Path != "github.com/neko233-com/godesktop" || module.Version != version || len(module.Replace) != 0 || module.GoMod == "" || module.Dir == "" {
		return result, errors.New("release consumer must use the actual published framework without replace")
	}
	infoPath := strings.TrimSuffix(module.GoMod, ".mod") + ".info"
	infoData, err := readBounded(infoPath, 64<<10)
	if err != nil {
		return result, err
	}
	if err := validateFrameworkOrigin(infoData, version, source); err != nil {
		return result, err
	}
	zipPath := strings.TrimSuffix(module.GoMod, ".mod") + ".zip"
	ziphash, err := readBounded(strings.TrimSuffix(module.GoMod, ".mod")+".ziphash", 128)
	if err != nil || strings.TrimSpace(string(ziphash)) != module.Sum || !strings.HasPrefix(module.Sum, "h1:") || !strings.HasPrefix(module.GoModSum, "h1:") {
		return result, errors.New("actual framework cached checksum identity differs")
	}
	verified, err := nativeguard.Run(ctx, nativeguard.Options{Command: []string{"go", "mod", "verify"}, Directory: project, Environment: environment, Timeout: 60 * time.Second})
	if err != nil || strings.TrimSpace(string(verified.Stdout)) != "all modules verified" {
		return result, errors.Join(err, fmt.Errorf("release framework module verification did not pass: %s", verified.Stderr[:min(len(verified.Stderr), 4096)]))
	}
	result = Framework{Path: module.Path, Version: version, Source: source, Sum: module.Sum, GoModSum: module.GoModSum}
	for _, item := range []struct {
		path string
		file *File
		max  int64
	}{{infoPath, &result.Info, 64 << 10}, {module.GoMod, &result.Module, 64 << 10}, {zipPath, &result.Zip, 512 << 20}} {
		*item.file, err = HashFile(ctx, filepath.Clean(item.path), item.max)
		if err != nil {
			return Framework{}, err
		}
	}
	return result, nil
}

func validateFrameworkOrigin(data []byte, version, source string) error {
	var info struct {
		Version string
		Origin  struct{ VCS, URL, Hash, Ref string }
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return err
	}
	if info.Origin.VCS == "" || info.Origin.Hash == "" || info.Origin.Ref == "" {
		return errors.New("framework cache .info lacks actual Origin; preserve that record and fetch the published tag with GOPROXY=https://proxy.golang.org in a fresh private GOMODCACHE and GOWORK=off; checksum text cannot substitute for Origin")
	}
	if info.Version != version || info.Origin.VCS != "git" || info.Origin.URL != "https://github.com/neko233-com/godesktop" || info.Origin.Hash != source || info.Origin.Ref != "refs/tags/"+version {
		return errors.New("actual framework cache Origin differs from published tag/source")
	}
	return nil
}
