// gocode-manifest signs release integrity metadata with a free Ed25519 key.
// Private keys must stay outside the checkout and must never be printed.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/neko233-com/gocode/internal/installlayout"
	"github.com/neko233-com/gocode/internal/update"
)

type input struct {
	Version string `json:"version"`
	Source  string `json:"source"`
	Assets  []struct {
		Path     string `json:"path"`
		Platform string `json:"platform"`
	} `json:"assets"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	generate := flag.Bool("generate-key", false, "Generate a private publisher key outside the checkout")
	keyPath := flag.String("key", "", "Private PEM file (or GOCODE_UPDATE_SIGNING_KEY PEM environment)")
	publicPath := flag.String("public-output", "", "Write the public key as Base64")
	inputPath := flag.String("input", "", "JSON version/source/assets input")
	outputPath := flag.String("output", "", "Signed update manifest output")
	flag.Parse()
	if *generate {
		if *keyPath == "" || *publicPath == "" {
			return errors.New("key generation needs -key and -public-output")
		}
		absolute, err := filepath.Abs(*keyPath)
		if err != nil {
			return err
		}
		checkout, err := os.Getwd()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(checkout, absolute)
		if err != nil && strings.EqualFold(filepath.VolumeName(checkout), filepath.VolumeName(absolute)) {
			return err
		}
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errors.New("private signing keys must be outside the checkout")
		}
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		der, err := x509.MarshalPKCS8PrivateKey(private)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
			return err
		}
		f, err := os.OpenFile(absolute, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
		err = errors.Join(writeErr, f.Sync(), f.Close())
		if err != nil {
			return err
		}
		return os.WriteFile(*publicPath, []byte(base64.StdEncoding.EncodeToString(public)+"\n"), 0644)
	}
	if *inputPath == "" || *outputPath == "" {
		return errors.New("signing needs -input and -output")
	}
	keyData := []byte(os.Getenv("GOCODE_UPDATE_SIGNING_KEY"))
	if *keyPath != "" {
		var err error
		keyData, err = os.ReadFile(*keyPath)
		if err != nil {
			return err
		}
	}
	block, _ := pem.Decode(keyData)
	if block == nil || block.Type != "PRIVATE KEY" {
		return errors.New("invalid private key PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	private, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return errors.New("publisher key must be Ed25519")
	}
	public, err := update.PublisherKey()
	if err != nil {
		return err
	}
	if !private.Public().(ed25519.PublicKey).Equal(public) {
		return errors.New("signing key does not match the application's publisher public key")
	}
	f, err := os.Open(*inputPath)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, update.MaxManifestBytes+1))
	if err != nil {
		return err
	}
	if len(data) > update.MaxManifestBytes {
		return errors.New("manifest input too large")
	}
	var config input
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	m := update.Manifest{Owner: installlayout.Owner, Schema: 1, Layout: installlayout.Schema, Version: config.Version, Source: config.Source}
	for _, asset := range config.Assets {
		f, err := os.Open(asset.Path)
		if err != nil {
			return err
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > update.MaxArchiveBytes {
			f.Close()
			return errors.New("invalid release asset")
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, f)
		err = errors.Join(copyErr, f.Close())
		if err != nil {
			return err
		}
		m.Assets = append(m.Assets, update.Asset{Name: filepath.Base(asset.Path), Platform: asset.Platform, Size: info.Size(), SHA256: hex.EncodeToString(hash.Sum(nil))})
	}
	envelope, err := update.Sign(m, private)
	if err != nil {
		return err
	}
	return os.WriteFile(*outputPath, envelope, 0644)
}
