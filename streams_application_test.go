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

func TestApplicationShutdownCancelsBlockedStreamProducer(t *testing.T) {
	streams := omniweb.NewStreams(1)
	entered, stopped := make(chan struct{}), make(chan struct{})
	err := omniweb.BindStream(streams, "produce", "", func(ctx context.Context, _ struct{}, ch *omniweb.Channel[string]) (int, error) {
		defer close(stopped)
		close(entered)
		for n := 0; ; n++ {
			if err := ch.Send(strings.Repeat("x", 128<<10)); err != nil {
				return n, err
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	app, err := New(Options{Streams: streams, Assets: fstest.MapFS{"index.html": {Data: []byte("test")}}, Window: &WindowOptions{URL: "/"}})
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
		req, err := http.NewRequestWithContext(t.Context(), "POST", origin+omniweb.Prefix+"stream", strings.NewReader(`{"v":1,"id":"one","method":"produce","params":{}}`))
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
			t.Fatalf("unexpected frame %v", frame)
		}
		<-entered
		// Never acknowledge any values. Closing the host must release even a producer
		// blocked by the consumption window, then wait for its handler to return.
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("producer survived shutdown")
	}
	for {
		var frame map[string]any
		err = decoder.Decode(&frame)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if frame["kind"] != "value" {
			t.Fatalf("unexpected shutdown frame %v", frame)
		}
	}
}
