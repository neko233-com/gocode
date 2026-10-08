package localrelease

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/neko233-com/gocode/internal/installlayout"
	"github.com/neko233-com/gocode/internal/update"
)

type Staged struct {
	Version  string `json:"version"`
	Source   string `json:"source"`
	Platform string `json:"platform"`
	Archive  File   `json:"archive"`
	Envelope File   `json:"envelope"`
	Console  File   `json:"console"`
	GUI      File   `json:"gui"`
}

// StageSigned uses the production publisher identity and archive staging path.
// There is no network Check/Apply and no signing/test-key override in the CLI.
func StageSigned(ctx context.Context, root, archive, envelopePath, version, source, platform string) (Staged, error) {
	key, err := update.PublisherKey()
	if err != nil {
		return Staged{}, err
	}
	return stageWithKey(ctx, root, archive, envelopePath, version, source, platform, key)
}

func stageWithKey(ctx context.Context, root, archive, envelopePath, version, source, platform string, key ed25519.PublicKey) (Staged, error) {
	var result Staged
	envelope, err := readBounded(envelopePath, update.MaxManifestBytes)
	if err != nil {
		return result, err
	}
	manifest, err := update.Verify(envelope, key)
	if err != nil {
		return result, err
	}
	if manifest.Version != version || manifest.Source != source {
		return result, errors.New("signed local package source/version mismatch")
	}
	asset, err := manifest.Asset(platform)
	if err != nil {
		return result, err
	}
	if filepath.Base(archive) != asset.Name {
		return result, errors.New("signed local archive name mismatch")
	}
	archiveHash, err := HashFile(ctx, archive, update.MaxArchiveBytes)
	if err != nil {
		return result, err
	}
	if archiveHash.Bytes != asset.Size || archiveHash.SHA256 != asset.SHA256 {
		return result, fmt.Errorf("signed local archive bytes mismatch: %s", filepath.Base(archive))
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		return result, errors.New("local stage requires an existing empty owned directory")
	}
	if err := os.Mkdir(filepath.Join(root, "versions"), 0700); err != nil {
		return result, err
	}
	base, _ := json.Marshal(installlayout.Manifest{Owner: installlayout.Owner, Schema: 1, Version: "0.0.0", Source: source})
	if err := os.WriteFile(filepath.Join(root, "base.json"), base, 0600); err != nil {
		return result, err
	}
	staged, err := update.Stage(ctx, root, archive, envelope, key, platform)
	if err != nil {
		return result, err
	}
	result.Version, result.Source, result.Platform = staged.Version, staged.Source, platform
	result.Archive = archiveHash
	result.Envelope, err = HashFile(ctx, envelopePath, update.MaxManifestBytes)
	if err != nil {
		return result, err
	}
	console, gui := "gocode-app", "gocode-app"
	if platform == "windows/amd64" {
		console, gui = "gocode-app.exe", "gocode-app-gui.exe"
	}
	result.Console, err = HashFile(ctx, filepath.Join(root, "versions", staged.Version, console), update.MaxArchiveBytes)
	if err != nil {
		return result, err
	}
	result.GUI, err = HashFile(ctx, filepath.Join(root, "versions", staged.Version, gui), update.MaxArchiveBytes)
	return result, err
}

func ProbeStaged(ctx context.Context, root string, staged Staged) error {
	if !installlayout.ValidVersion(staged.Version) || !ValidSource(staged.Source) || staged.Platform != "windows/amd64" {
		return errors.New("invalid staged Windows source/version/platform")
	}
	for i, program := range []File{staged.Console, staged.GUI} {
		name := []string{"gocode-app.exe", "gocode-app-gui.exe"}[i]
		expected, err := Within(root, filepath.Join("versions", staged.Version, name))
		if err != nil || expected != program.Path {
			return errors.New("staged program is outside the exact versioned root")
		}
		actual, err := HashFile(ctx, expected, update.MaxArchiveBytes)
		if err != nil || actual != program {
			return errors.New("staged program bytes differ before production health probe")
		}
	}
	return update.Probe(ctx, root, installlayout.Manifest{Owner: installlayout.Owner, Schema: 1, Version: staged.Version, Source: staged.Source})
}

func ReadStaged(path string) (Staged, error) {
	var staged Staged
	err := decodeBounded(path, &staged)
	return staged, err
}
