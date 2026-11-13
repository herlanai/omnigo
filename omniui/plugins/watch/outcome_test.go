package watch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/herlanai/omnigo/omniweb"
)

func TestBoundWatchOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		nextErr    error
		request    string
		entries    int
	}{
		{name: "completed", nextErr: io.EOF},
		{name: "unknown-root", code: "denied", request: `{"root":"unknown"}`},
		{name: "debounce-limit", code: "limit", request: `{"root":"project","debounceMs":60001}`},
		{name: "missing-file", code: "not-found", request: `{"root":"project","path":"missing"}`},
		{name: "snapshot-limit", code: "limit", entries: 2},
		{name: "unsupported", code: "unsupported", nextErr: &fs.PathError{Op: "watch", Path: "PRIVATE", Err: errors.ErrUnsupported}},
		{name: "encoding", code: "path-encoding", nextErr: ErrEncoding},
		{name: "io", code: "io", nextErr: &fs.PathError{Op: "read", Path: "PRIVATE", Err: fs.ErrPermission}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for n := 0; n < tc.entries; n++ {
				if err := os.WriteFile(filepath.Join(dir, string(rune('a'+n))), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			s, err := NewService(ServiceOptions{Roots: map[string]Root{"project": {Path: dir}}, Scan: Options{MaxEntries: 1}})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			s.next = func(context.Context, *Watcher) ([]Event, error) {
				if tc.nextErr == nil {
					t.Fatal("unexpected watcher start")
				}
				return nil, tc.nextErr
			}
			streams := omniweb.NewStreams(1)
			if err = s.Bind(streams, "watch", ""); err != nil {
				t.Fatal(err)
			}
			h, err := omniweb.NewHTTP(omniweb.NewRouter(1, 1024), omniweb.HTTPOptions{Streams: streams, Host: "app.test", Origins: []string{"http://app.test"}})
			if err != nil {
				t.Fatal(err)
			}
			params := tc.request
			if params == "" {
				params = `{"root":"project"}`
			}
			req := httptest.NewRequest("POST", "http://app.test"+omniweb.Prefix+"stream", strings.NewReader(`{"v":1,"id":"test","method":"watch","params":`+params+`}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", "http://app.test")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
			var final struct {
				Kind   string
				Result *Outcome
				Error  json.RawMessage
			}
			if err = json.Unmarshal([]byte(lines[len(lines)-1]), &final); err != nil {
				t.Fatal(err, rec.Body.String())
			}
			if final.Kind != "result" || final.Result == nil || final.Result.Code != tc.code || len(final.Error) != 0 {
				t.Fatal(rec.Body.String())
			}
			if tc.code != "" && final.Result.Message == "" {
				t.Fatal("missing error description")
			}
			if strings.Contains(rec.Body.String(), "PRIVATE") || strings.Contains(rec.Body.String(), dir) {
				t.Fatal("private path leaked")
			}
			if s.active != 0 || len(s.reservations) != 0 || len(s.bySource) != 0 {
				t.Fatal("subscription leaked")
			}
		})
	}
}
