package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestSignatureDownloadAndCleanup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "artifact") }))
	defer srv.Close()
	hash := sha256.Sum256([]byte("artifact"))
	release := Release{Version: "1.2.0", URL: srv.URL, SHA256: hex.EncodeToString(hash[:]), Size: 8}
	b, _ := json.Marshal(release)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := ed25519.Sign(priv, b)
	r, e := Verify(b, sig, pub)
	if e != nil {
		t.Fatal(e)
	}
	b[0] = 'x'
	if _, e = Verify(b, sig, pub); e == nil {
		t.Fatal("bad signature accepted")
	}
	dir := t.TempDir()
	path, e := Download(context.Background(), r, dir, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	os.Remove(path)
	r.Size = 7
	if _, e = Download(context.Background(), r, dir, nil, nil); e == nil {
		t.Fatal("oversize accepted")
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 0 {
		t.Fatal("partial artifact leaked")
	}
}

func TestChunkedProgressUsesSignedManifestSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.(http.Flusher).Flush(); io.WriteString(w, "artifact") }))
	defer server.Close()
	sum := sha256.Sum256([]byte("artifact"))
	release := Release{URL: server.URL, SHA256: hex.EncodeToString(sum[:]), Size: 8}
	var received, total int64
	path, err := Download(t.Context(), release, t.TempDir(), nil, func(n, size int64) { received, total = n, size })
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	if received != 8 || total != 8 {
		t.Fatal("progress lost manifest size", received, total)
	}
}

func TestReleaseValidationRetainsNarrowHTTPPolicy(t *testing.T) {
	for _, address := range []string{"http://127.0.0.2/file", "http://example.com/file", "https://user:password@example.com/file", "https://example.com/file#fragment"} {
		r := Release{URL: address, SHA256: hex.EncodeToString(make([]byte, 32)), Size: 1}
		if _, err := r.artifact(); err == nil {
			t.Fatal("release policy broadened", address)
		}
	}
	for _, address := range []string{"http://localhost/file", "http://127.0.0.1/file", "http://[::1]/file", "https://example.com/file"} {
		r := Release{URL: address, SHA256: hex.EncodeToString(make([]byte, 32)), Size: 1}
		if _, err := r.artifact(); err != nil {
			t.Fatal(address, err)
		}
	}
}
