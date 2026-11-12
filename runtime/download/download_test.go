package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func digest(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
func TestRetryIntegrityAndAtomicPublication(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.WriteHeader(503)
			return
		}
		io.WriteString(w, "verified")
	}))
	defer server.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "artifact")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	waits := 0
	f := File{URL: server.URL, SHA256: digest("verified"), Size: 8, wait: func(context.Context, time.Duration) error { waits++; return nil }}
	if err := Get(context.Background(), f, path); err != nil {
		t.Fatal(err)
	}
	if waits != 1 || requests.Load() != 2 || !Check(path, digest("verified")) {
		t.Fatal("retry or bytes")
	}
	f.SHA256 = digest("wrong")
	if err := Get(context.Background(), f, path); err == nil {
		t.Fatal("hash accepted")
	}
	if !Check(path, digest("verified")) {
		t.Fatal("failed verification overwrote target")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("staging leaked", entries)
	}
}
func TestCacheConcurrentClientsAndCancellation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		close(entered)
		<-release
		io.WriteString(w, "cache")
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "cache")
	f := File{URL: server.URL, SHA256: digest("cache")}
	first := make(chan error, 1)
	go func() { first <- Cached(context.Background(), f, path) }()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Cached(ctx, f, path); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	waiting := make(chan struct{})
	firstDone := make(chan struct{})
	second := make(chan error, 1)
	follower := f
	follower.wait = func(ctx context.Context, d time.Duration) error { close(waiting); <-firstDone; return nil }
	go func() { second <- Cached(context.Background(), follower, path) }()
	<-waiting
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	close(firstDone)
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 {
		t.Fatal("duplicate cache download")
	}
}
func TestBudgetAndProgressCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "12345678") }))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "file")
	f := File{URL: server.URL, SHA256: digest("12345678"), MaxBytes: 4}
	if err := Get(context.Background(), f, path); err == nil {
		t.Fatal("size budget")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.MaxBytes = 32
	f.Progress = func(int64, int64) { cancel() }
	if err := Get(ctx, f, path); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("cancelled data published")
	}
}

func TestRedirectPolicyIsNotRetried(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.invalid/artifact", http.StatusFound)
	}))
	defer server.Close()
	waits := 0
	f := File{URL: server.URL, SHA256: digest("unused"), wait: func(context.Context, time.Duration) error { waits++; return nil }}
	if err := Get(context.Background(), f, filepath.Join(t.TempDir(), "target")); err == nil || waits != 0 {
		t.Fatal("policy error retried", err, waits)
	}
}

func TestStageValidatesBeforeFilesystemAccess(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	_, err := Stage(t.Context(), File{URL: "https://example.com/artifact", SHA256: "invalid"}, dir)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid metadata reached filesystem", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Stage(ctx, File{URL: "https://example.com/artifact", SHA256: digest("x")}, dir); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
