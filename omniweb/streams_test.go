package omniweb

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func streamRequest(body string) *http.Request {
	r := httptest.NewRequest("POST", "http://app.test"+Prefix+"stream", strings.NewReader(body))
	r.Header.Set("Origin", "http://app.test")
	r.Header.Set("Content-Type", "application/json")
	return r
}
func TestTypedStreamCloseAndResultOrder(t *testing.T) {
	streams := NewStreams(1)
	err := BindStream(streams, "count", "read", func(ctx context.Context, n int, ch *Channel[int]) (string, error) {
		for i := 0; i < n; i++ {
			if err := ch.Send(i); err != nil {
				return "", err
			}
		}
		if err := ch.Close(); err != nil {
			return "", err
		}
		if err := ch.Close(); err != nil {
			return "", err
		}
		if err := ch.Send(99); !errors.Is(err, ErrChannelClosed) {
			t.Error(err)
		}
		return "finished", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	h, _ := NewHTTP(NewRouter(1, 1024), HTTPOptions{Streams: streams, Host: "app.test", Origins: []string{"http://app.test"}, Grants: []string{"read"}})
	// No ACKs: a completed producer must finish its bounded tail and retire.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, streamRequest(`{"v":1,"id":"one","method":"count","params":2}`))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(streams.sessions) != 0 || len(streams.slots) != 0 {
		t.Fatal("completed producer retained session")
	}
	decoder := json.NewDecoder(w.Body)
	for i, kind := range []string{"ready", "value", "value", "end", "result"} {
		var frame streamFrame
		if err := decoder.Decode(&frame); err != nil {
			t.Fatal(err)
		}
		if frame.Kind != kind || frame.ID != "one" || frame.Version != 1 {
			t.Fatalf("%d: %+v", i, frame)
		}
		if kind == "result" && string(frame.Result) != `"finished"` {
			t.Fatal(frame)
		}
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatal(err)
	}
}
func TestStreamAuthorizationValidationAndPanic(t *testing.T) {
	streams := NewStreams(1)
	calls := 0
	BindStream(streams, "panic", "read", func(context.Context, struct{}, *Channel[int]) (int, error) { calls++; panic("private details") })
	for _, test := range []struct {
		grant  bool
		params string
		status int
	}{{false, `{}`, 403}, {true, `{"unknown":1}`, 400}, {true, `{}`, 200}} {
		grants := []string{}
		if test.grant {
			grants = append(grants, "read")
		}
		h, _ := NewHTTP(NewRouter(1, 1024), HTTPOptions{Streams: streams, Host: "app.test", Origins: []string{"http://app.test"}, Grants: grants})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, streamRequest(`{"v":1,"id":"one","method":"panic","params":`+test.params+`}`))
		if w.Code != test.status {
			t.Fatal(w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "private details") {
			t.Fatal("panic leaked")
		}
		if test.status == 200 && !strings.Contains(w.Body.String(), `"code":"internal"`) {
			t.Fatal(w.Body.String())
		}
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
func TestStreamCancellationReachesHandler(t *testing.T) {
	streams := NewStreams(1)
	entered := make(chan struct{})
	stopped := make(chan struct{})
	BindStream(streams, "wait", "", func(ctx context.Context, _ struct{}, _ *Channel[int]) (bool, error) {
		close(entered)
		<-ctx.Done()
		close(stopped)
		return false, ctx.Err()
	})
	h, _ := NewHTTP(NewRouter(1, 1024), HTTPOptions{Streams: streams, Host: "app.test", Origins: []string{"http://app.test"}})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.ServeHTTP(httptest.NewRecorder(), streamRequest(`{"v":1,"id":"one","method":"wait","params":{}}`).WithContext(ctx))
	}()
	<-entered
	w := httptest.NewRecorder()
	h.ServeHTTP(w, streamRequest(`{"v":1,"id":"two","method":"wait","params":{}}`))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	cancel()
	<-stopped
	<-done
	if len(streams.slots) != 0 {
		t.Fatal("slot leaked")
	}
}
