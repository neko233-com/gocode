package update

import (
	"crypto/ed25519"
	_ "embed"
	"encoding/base64"
	"errors"
	"strings"
)

//go:embed public-key.txt
var publisherKey string

func PublisherKey() (ed25519.PublicKey, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(publisherKey))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, errors.New("invalid embedded publisher key")
	}
	return ed25519.PublicKey(key), nil
}
