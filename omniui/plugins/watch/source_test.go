package watch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/herlanai/omnigo/omniweb"
)

func TestWatchSourceQuotaCannotBeChangedByRequestHeaders(t *testing.T) {
	s, err := NewService(ServiceOptions{Roots: map[string]Root{"project": {Path: t.TempDir()}}, MaxWatches: 2, MaxWatchesPerSource: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.next = func(ctx context.Context, _ *Watcher) ([]Event, error) { <-ctx.Done(); return nil, ctx.Err() }
	streams := omniweb.NewStreams(8)
	if err = s.Bind(streams, "observe", ""); err != nil {
		t.Fatal(err)
	}
	newHost := func() *omniweb.HTTP {
		h, err := omniweb.NewHTTP(omniweb.NewRouter(1, 1024), omniweb.HTTPOptions{Streams: streams, Host: "app.test", Origins: []string{"http://app.test"}})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	request := func(ctx context.Context) *http.Request {
		r := httptest.NewRequest("POST", "http://app.test"+omniweb.Prefix+"stream", strings.NewReader(`{"v":1,"id":"call","method":"observe","params":{"root":"project"}}`)).WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://app.test")
		return r
	}
	start := func(h *omniweb.HTTP) func() {
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		w := &serviceRecorder{ResponseRecorder: httptest.NewRecorder(), frames: make(chan json.RawMessage, 8)}
		go func() { defer close(done); h.ServeHTTP(w, request(ctx)) }()
		<-w.frames // transport ready
		frame := <-w.frames
		if !strings.Contains(string(frame), `"ready":true`) {
			cancel()
			<-done
			t.Fatalf("watch not started: %s", frame)
		}
		return func() { cancel(); <-done }
	}
	denied := func(h *omniweb.HTTP) {
		r := request(t.Context())
		r.Header.Set("X-Page-ID", "new-page")
		r.Header.Set("X-Source", "other")
		r.Header.Set("Cookie", "page=another")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if !strings.Contains(w.Body.String(), `"code":"limit"`) || strings.Contains(w.Body.String(), `"ready":true`) {
			t.Fatal(w.Body)
		}
	}
	first, second, third := newHost(), newHost(), newHost()
	stopFirst := start(first)
	defer stopFirst()
	denied(first)
	stopSecond := start(second)
	defer stopSecond()
	denied(third) // total quota is independent of per-source quota
	stopFirst()
	// Once a canceled request exits, its own source can subscribe again.
	stopAgain := start(first)
	stopAgain()
	stopSecond()
	s.mu.Lock()
	active, sources := s.active, len(s.bySource)
	s.mu.Unlock()
	if active != 0 || sources != 0 {
		t.Fatal("quota identities leaked", active, sources)
	}
}
