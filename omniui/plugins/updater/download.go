// Package updater verifies signed releases and stages bounded downloads.
// It never installs or relaunches an application without a host implementation.
package updater

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/herlanai/omnigo/runtime/download"
)

type Release struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
	Notes   string `json:"notes"`
}

// artifact applies release-specific constraints on top of shared artifact
// validation. Updater's original HTTP allowlist is intentionally narrower than
// the downloader's loopback policy; refactoring must not broaden that policy.
func (r Release) artifact() (download.File, error) {
	file := download.File{URL: r.URL, SHA256: r.SHA256, Size: r.Size, MaxBytes: 4 << 30}
	if r.Size <= 0 {
		return file, errors.New("updater: release size must be positive")
	}
	if err := file.Validate(); err != nil {
		return file, err
	}
	u, _ := url.Parse(r.URL) // Already validated by the download layer.
	if u.Scheme == "http" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1" {
		return file, errors.New("updater: invalid download URL")
	}
	return file, nil
}

// Verify authenticates the exact bytes before parsing URLs or release metadata.
func Verify(manifest, signature []byte, key ed25519.PublicKey) (Release, error) {
	var r Release
	if len(manifest) > 1<<20 || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, manifest, signature) {
		return r, errors.New("updater: invalid manifest signature")
	}
	if err := json.Unmarshal(manifest, &r); err != nil {
		return r, err
	}
	if r.Version == "" {
		return Release{}, errors.New("updater: missing version")
	}
	if _, err := r.artifact(); err != nil {
		return Release{}, err
	}
	return r, nil
}

// Download stages a verified artifact. Caller owns and removes the successful file.
// Progress retains signed manifest totals even with chunked HTTP responses.
func Download(ctx context.Context, r Release, directory string, client *http.Client, progress func(int64, int64)) (string, error) {
	file, err := r.artifact()
	if err != nil {
		return "", err
	}
	file.Client = client
	if progress != nil {
		file.Progress = func(received, _ int64) { progress(received, r.Size) }
	}
	return download.Stage(ctx, file, directory)
}
