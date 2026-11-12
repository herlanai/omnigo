// Package updater verifies signed releases and stages bounded downloads.
// It never installs or relaunches an application without a host implementation.
package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

type Release struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
	Notes   string `json:"notes"`
}

// Verify checks the exact signed manifest bytes before parsing any download URL.
func Verify(manifest, signature []byte, key ed25519.PublicKey) (Release, error) {
	var r Release
	if len(manifest) > 1<<20 || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, manifest, signature) {
		return r, errors.New("updater: invalid manifest signature")
	}
	if e := json.Unmarshal(manifest, &r); e != nil {
		return r, e
	}
	digest, e := hex.DecodeString(r.SHA256)
	if e != nil || len(digest) != 32 || r.Size <= 0 || r.Size > 4<<30 || r.Version == "" {
		return Release{}, errors.New("updater: invalid release metadata")
	}
	if !safeURL(r.URL) {
		return Release{}, errors.New("updater: invalid download URL")
	}
	return r, nil
}
func safeURL(address string) bool {
	u, e := url.Parse(address)
	if e != nil || u.User != nil || u.Host == "" || u.Fragment != "" {
		return false
	}
	return u.Scheme == "https" || (u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))
}

// Download returns a new private staging file. Caller must remove it after use.
// Every failure deletes incomplete data. Progress runs on the calling goroutine.
func Download(ctx context.Context, r Release, directory string, client *http.Client, progress func(int64, int64)) (path string, err error) {
	digest, e := hex.DecodeString(r.SHA256)
	if e != nil || len(digest) != 32 || r.Size <= 0 || r.Size > 4<<30 || !safeURL(r.URL) {
		return "", errors.New("updater: invalid release")
	}
	c := http.Client{Timeout: 10 * time.Minute}
	if client != nil {
		c = *client
	}
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 || !safeURL(req.URL.String()) || req.URL.Scheme != via[0].URL.Scheme {
			return errors.New("updater: redirect denied")
		}
		return nil
	}
	req, e := http.NewRequestWithContext(ctx, "GET", r.URL, nil)
	if e != nil {
		return "", e
	}
	resp, e := c.Do(req)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("updater: HTTP status %d", resp.StatusCode)
	}
	f, e := os.CreateTemp(directory, "omnigo-update-*")
	if e != nil {
		return "", e
	}
	name := f.Name()
	success := false
	defer func() {
		f.Close()
		if !success {
			os.Remove(name)
		}
	}()
	hash := sha256.New()
	buffer := make([]byte, 64<<10)
	var count int64
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			count += int64(n)
			if count > r.Size {
				return "", errors.New("updater: artifact larger than manifest")
			}
			if _, e = f.Write(buffer[:n]); e != nil {
				return "", e
			}
			hash.Write(buffer[:n])
			if progress != nil {
				progress(count, r.Size)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", readErr
		}
		if e = ctx.Err(); e != nil {
			return "", e
		}
	}
	if count != r.Size || hex.EncodeToString(hash.Sum(nil)) != hex.EncodeToString(digest) {
		return "", errors.New("updater: artifact verification failed")
	}
	if e = f.Sync(); e != nil {
		return "", e
	}
	if e = f.Close(); e != nil {
		return "", e
	}
	success = true
	return name, nil
}
