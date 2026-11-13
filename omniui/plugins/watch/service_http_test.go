package watch

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/herlanai/omnigo/omniweb"
)

type serviceRecorder struct {
	*httptest.ResponseRecorder
	frames chan json.RawMessage
}

func (w *serviceRecorder) Write(p []byte) (int, error) {
	w.frames <- append(json.RawMessage(nil), p...)
	return w.ResponseRecorder.Write(p)
}
func TestNamedWatchThroughOmniWebStream(t *testing.T) {
	dir := t.TempDir()
	s, err := NewService(ServiceOptions{Roots: map[string]Root{"project": {Path: dir}}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ticks := make(chan struct{})
	s.next = func(ctx context.Context, w *Watcher) ([]Event, error) {
		select {
		case <-ticks:
			return w.Poll(ctx)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	streams := omniweb.NewStreams(1)
	if err = s.Bind(streams, "watch.observe", "files.watch"); err != nil {
		t.Fatal(err)
	}
	h, err := omniweb.NewHTTP(omniweb.NewRouter(1, 1024), omniweb.HTTPOptions{Streams: streams, Host: "app.test", Origins: []string{"http://app.test"}, Grants: []string{"files.watch"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	r := httptest.NewRequest("POST", "http://app.test"+omniweb.Prefix+"stream", strings.NewReader(`{"v":1,"id":"one","method":"watch.observe","params":{"root":"project","path":"."}}`)).WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://app.test")
	w := &serviceRecorder{ResponseRecorder: httptest.NewRecorder(), frames: make(chan json.RawMessage, 8)}
	done := make(chan struct{})
	go func() { defer close(done); h.ServeHTTP(w, r) }()
	read := func() struct {
		Kind string
		Data Batch
	} {
		t.Helper()
		var f struct {
			Kind string
			Data Batch
		}
		if err := json.Unmarshal(<-w.frames, &f); err != nil {
			t.Fatal(err)
		}
		return f
	}
	if f := read(); f.Kind != "ready" {
		t.Fatal(f)
	}
	if f := read(); f.Kind != "value" || !f.Data.Ready {
		t.Fatal(f)
	}
	if err = os.WriteFile(filepath.Join(dir, "file"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	ticks <- struct{}{}
	if f := read(); f.Kind != "value" || len(f.Data.Events) != 1 || f.Data.Events[0].Path != "file" {
		t.Fatal(f)
	}
	cancel()
	<-done
	if s.active != 0 {
		t.Fatal("watch retained after page cancellation")
	}
}

func TestWatchStreamRejectsInvalidPathWithoutSendingReplacement(t *testing.T) {
	s, err := NewService(ServiceOptions{Roots: map[string]Root{"project": {Path: t.TempDir()}}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.next = func(context.Context, *Watcher) ([]Event, error) {
		return []Event{{Op: "create", Path: "safe"}, {Op: "rename", Path: "new", OldPath: "private\xff"}}, nil
	}
	streams := omniweb.NewStreams(1)
	if err := s.Bind(streams, "watch.observe", "files.watch"); err != nil {
		t.Fatal(err)
	}
	h, err := omniweb.NewHTTP(omniweb.NewRouter(1, 1024), omniweb.HTTPOptions{Streams: streams, Host: "app.test", Origins: []string{"http://app.test"}, Grants: []string{"files.watch"}})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "http://app.test"+omniweb.Prefix+"stream", strings.NewReader(`{"v":1,"id":"encoding","method":"watch.observe","params":{"root":"project"}}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://app.test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var kinds []string
	for _, line := range strings.Split(strings.TrimSpace(w.Body.String()), "\n") {
		var frame struct {
			Kind   string
			Data   Batch
			Error  *struct{ Code string }
			Result Outcome
		}
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Fatal(err, line)
		}
		kinds = append(kinds, frame.Kind)
		if frame.Kind == "value" && (!frame.Data.Ready || len(frame.Data.Events) != 0) {
			t.Fatal("invalid batch reached transport", line)
		}
		if frame.Kind == "result" && (frame.Error != nil || frame.Result.Code != "path-encoding") {
			t.Fatal(line)
		}
	}
	if strings.Join(kinds, ",") != "ready,value,end,result" {
		t.Fatal(kinds, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "private") || s.active != 0 {
		t.Fatal("path disclosed or watch retained", w.Body.String())
	}
}
