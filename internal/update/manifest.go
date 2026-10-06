// Package update verifies publisher metadata and stages owned versioned payloads.
// Its integrity signatures are free Ed25519 metadata, not OS code certificates.
package update

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/neko233-com/gocode/internal/installlayout"
)

const MaxManifestBytes = 128 << 10
const MaxArchiveBytes = 512 << 20
const MaxExpandedBytes = 512 << 20

type Asset struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
}
type Manifest struct {
	Owner   string  `json:"owner"`
	Schema  int     `json:"schema"`
	Layout  int     `json:"layout"`
	Version string  `json:"version"`
	Source  string  `json:"source"`
	Assets  []Asset `json:"assets"`
}
type Envelope struct {
	Payload   json.RawMessage `json:"payload"`
	Signature string          `json:"signature"`
}

func decode(data []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("trailing metadata data")
	}
	return nil
}
func (m Manifest) Validate() error {
	if m.Owner != installlayout.Owner || m.Schema != 1 || m.Layout != installlayout.Schema || !installlayout.ValidVersion(m.Version) {
		return errors.New("unsupported update manifest identity, schema or version")
	}
	if _, err := hex.DecodeString(m.Source); err != nil || len(m.Source) != 40 {
		return errors.New("release source must be an immutable Git commit")
	}
	if len(m.Assets) < 1 || len(m.Assets) > 16 {
		return errors.New("invalid release asset count")
	}
	names, platforms := map[string]bool{}, map[string]bool{}
	for _, a := range m.Assets {
		if a.Name == "" || len(a.Name) > 180 || strings.ContainsAny(a.Name, "/\\:\x00") || !strings.HasSuffix(a.Name, ".zip") || names[a.Name] || platforms[a.Platform] {
			return errors.New("invalid or duplicate update asset")
		}
		if a.Platform != "windows/amd64" && a.Platform != "darwin/amd64" && a.Platform != "darwin/arm64" {
			return errors.New("unsupported update platform")
		}
		if a.Size <= 0 || a.Size > MaxArchiveBytes {
			return errors.New("update archive exceeds size policy")
		}
		if _, err := hex.DecodeString(a.SHA256); err != nil || len(a.SHA256) != 64 {
			return errors.New("invalid release hash")
		}
		names[a.Name] = true
		platforms[a.Platform] = true
	}
	return nil
}
func Sign(m Manifest, key ed25519.PrivateKey) ([]byte, error) {
	if len(key) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid signing key")
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	signature := ed25519.Sign(key, payload)
	return json.Marshal(Envelope{Payload: payload, Signature: base64.StdEncoding.EncodeToString(signature)})
}
func Verify(data []byte, key ed25519.PublicKey) (Manifest, error) {
	var m Manifest
	if len(data) > MaxManifestBytes || len(key) != ed25519.PublicKeySize {
		return m, errors.New("invalid manifest/key size")
	}
	var e Envelope
	if err := decode(data, &e); err != nil {
		return m, err
	}
	signature, err := base64.StdEncoding.DecodeString(e.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize || !ed25519.Verify(key, e.Payload, signature) {
		return m, errors.New("publisher update signature verification failed")
	}
	if err := decode(e.Payload, &m); err != nil {
		return m, err
	}
	return m, m.Validate()
}
func (m Manifest) Asset(platform string) (Asset, error) {
	for _, a := range m.Assets {
		if a.Platform == platform {
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("no update asset for %s", platform)
}
