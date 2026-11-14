package updater

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

// ParsePublicKey accepts a raw Ed25519 public key or the standard Base64 text
// produced by omni keygen. Raw bytes are never whitespace-trimmed.
func ParsePublicKey(data []byte) (ed25519.PublicKey, error) {
	key, err := decodeFixed(data, ed25519.PublicKeySize, "public key")
	return ed25519.PublicKey(key), err
}

// ParseSignature accepts a raw Ed25519 signature or standard Base64 text.
// Manifest bytes themselves must never be normalized before signature checking.
func ParseSignature(data []byte) ([]byte, error) {
	return decodeFixed(data, ed25519.SignatureSize, "signature")
}
func decodeFixed(data []byte, size int, name string) ([]byte, error) {
	if len(data) == size {
		return bytes.Clone(data), nil
	}
	text := bytes.TrimSpace(data)
	if len(text) != base64.StdEncoding.EncodedLen(size) {
		return nil, fmt.Errorf("updater: invalid %s length", name)
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(string(text))
	if err != nil || len(decoded) != size {
		return nil, fmt.Errorf("updater: invalid Base64 %s", name)
	}
	return decoded, nil
}
