package omniweb

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEventPermissionsSnapshotAndOverflow(t *testing.T) {
	e := NewEvents()
	defer e.Close()
	if err := e.Register("progress", "read"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.subscribe([]string{"progress"}, nil); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	sub, off, err := e.subscribe([]string{"progress"}, []string{"read"})
	if err != nil {
		t.Fatal(err)
	}
	defer off()
	value := map[string]int{"n": 1}
	e.Publish("progress", value)
	value["n"] = 2
	if string((<-sub.queue).Data) != `{"n":1}` {
		t.Fatal("payload aliased")
	}
	for i := 0; i < 64; i++ {
		if d, err := e.Publish("progress", i); err != nil || d.Delivered != 1 {
			t.Fatal(d, err)
		}
	}
	if d, err := e.Publish("progress", 65); err != nil || d.Disconnected != 1 {
		t.Fatal(d, err)
	}
	if reason := <-sub.end; reason != "overflow" {
		t.Fatal(reason)
	}
	if d, _ := e.Publish("progress", 66); d.Delivered != 0 {
		t.Fatal(d)
	}
}
func TestHTTPEventsReadyDeliveryAndClose(t *testing.T) {
	e := NewEvents()
	e.Register("progress", "read")
	h, err := NewHTTP(NewRouter(1, 1024), HTTPOptions{Events: e, Host: "app.test", Origins: []string{"http://app.test"}, Grants: []string{"read"}})
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { defer close(finished); h.ServeHTTP(w, r) }))
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", server.URL+Prefix+"events", strings.NewReader(`{"v":1,"topics":["progress"]}`))
	req.Host = "app.test"
	req.Header.Set("Origin", "http://app.test")
	req.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || response.Header.Get("Content-Type") != "application/x-ndjson" {
		t.Fatal(response.Status)
	}
	decoder := json.NewDecoder(response.Body)
	var frame eventFrame
	if err := decoder.Decode(&frame); err != nil || frame.Kind != "ready" {
		t.Fatal(frame, err)
	}
	e.Publish("progress", 42)
	if err := decoder.Decode(&frame); err != nil || frame.Topic != "progress" || string(frame.Data) != "42" {
		t.Fatal(frame, err)
	}
	e.Close()
	if err := decoder.Decode(&frame); err != nil || frame.Code != "closed" {
		t.Fatal(frame, err)
	}
	<-finished
}
func TestEventHTTPRejectsUntrustedSubscriptions(t *testing.T) {
	e := NewEvents()
	e.Register("private", "read")
	h, _ := NewHTTP(NewRouter(1, 1024), HTTPOptions{Events: e, Host: "app.test", Origins: []string{"http://app.test"}})
	for _, origin := range []string{"http://evil.test", "http://app.test"} {
		r := httptest.NewRequest("POST", "http://app.test"+Prefix+"events", strings.NewReader(`{"v":1,"topics":["private"]}`))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
