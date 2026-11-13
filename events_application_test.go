package omnigo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/herlanai/omnigo/omniweb"
	"github.com/herlanai/omnigo/omniwindow/host"
)

func TestApplicationShutdownCancelsEventStreams(t *testing.T) {
	events := omniweb.NewEvents()
	defer events.Close()
	if err := events.Register("progress", ""); err != nil {
		t.Fatal(err)
	}
	app, err := New(Options{Events: events, Assets: fstest.MapFS{"index.html": {Data: []byte("test")}}, Window: &WindowOptions{URL: "/"}})
	if err != nil {
		t.Fatal(err)
	}
	var response *http.Response
	var decoder *json.Decoder
	defer func() {
		if response != nil {
			response.Body.Close()
		}
	}()
	err = app.run(t.Context(), func(ctx context.Context, o host.Options) error {
		origin := strings.TrimSuffix(o.Window.URL, "/")
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, origin+omniweb.Prefix+"events", strings.NewReader(`{"v":1,"topics":["progress"]}`))
		if err != nil {
			return err
		}
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		response, err = (&http.Client{Timeout: 5 * time.Second}).Do(req)
		if err != nil {
			return err
		}
		if response.StatusCode != 200 {
			t.Fatalf("status %s", response.Status)
		}
		decoder = json.NewDecoder(response.Body)
		var frame map[string]any
		if err = decoder.Decode(&frame); err != nil {
			return err
		}
		if frame["kind"] != "ready" {
			t.Fatalf("frame %v", frame)
		}
		if _, err = events.Publish("progress", 42); err != nil {
			return err
		}
		if err = decoder.Decode(&frame); err != nil {
			return err
		}
		if frame["kind"] != "event" || frame["data"] != float64(42) {
			t.Fatalf("frame %v", frame)
		}
		// Return with an active stream; application shutdown must terminate it.
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if decoder == nil {
		t.Fatal("stream never started")
	}
	var frame any
	if err := decoder.Decode(&frame); err != io.EOF {
		t.Fatalf("stream did not finish cleanly: frame=%v err=%v", frame, err)
	}

	delivery, err := events.Publish("progress", 99)
	if err != nil {
		t.Fatalf("application closed caller-owned broker: %v", err)
	}
	if delivery.Delivered != 0 {
		t.Fatal("subscriber survived shutdown")
	}
}
