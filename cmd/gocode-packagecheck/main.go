// Verify the actual generated archive through the production updater and, for
// clean releases, execute its staged native version/source health check.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/neko233-com/gocode/internal/installlayout"
	"github.com/neko233-com/gocode/internal/update"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	archive := flag.String("archive", "", "Actual generated updater ZIP")
	platform := flag.String("platform", "", "Archive platform")
	version := flag.String("version", "", "Expected version")
	source := flag.String("source", "", "Expected immutable source SHA")
	probe := flag.Bool("probe", true, "Execute native version/source health check")
	flag.Parse()
	f, err := os.Open(*archive)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	hash := sha256.New()
	_, err = io.Copy(hash, f)
	f.Close()
	if err != nil {
		return err
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	metadata := update.Manifest{Owner: installlayout.Owner, Schema: 1, Layout: 1, Version: *version, Source: *source, Assets: []update.Asset{{Name: filepath.Base(*archive), Platform: *platform, Size: info.Size(), SHA256: hex.EncodeToString(hash.Sum(nil))}}}
	envelope, err := update.Sign(metadata, private)
	if err != nil {
		return err
	}
	root, err := os.MkdirTemp("", "gocode-generated-package-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root) // exclusively this call's newly created fixture
	if err := os.Mkdir(filepath.Join(root, "versions"), 0700); err != nil {
		return err
	}
	base, _ := json.Marshal(installlayout.Manifest{Owner: installlayout.Owner, Schema: 1, Version: "0.0.0", Source: *source})
	if err := os.WriteFile(filepath.Join(root, "base.json"), base, 0600); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	staged, err := update.Stage(ctx, root, *archive, envelope, public, *platform)
	if err != nil {
		return err
	}
	if *probe {
		if *platform == "windows/amd64" {
			app := filepath.Join(root, "versions", staged.Version, "gocode-app.exe")
			if data, err := exec.CommandContext(ctx, app, "-terminal-runtime-check").CombinedOutput(); err != nil {
				return fmt.Errorf("packaged embedded ConPTY: %w %s", err, data)
			}
		}
		if err := update.Probe(ctx, root, staged); err != nil {
			return err
		}
	}
	fmt.Printf("Actual %s ZIP passed updater extraction and health=%t: %s %s\n", *platform, *probe, *version, *source)
	return nil
}
