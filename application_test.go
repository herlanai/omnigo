package omnigo

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/herlanai/omnigo/omniwindow/host"
	"github.com/herlanai/omnigo/omniweb"
	"github.com/herlanai/omnigo/runtime/lifecycle"
)

func TestApplicationOwnsResourcesBindingsAndShutdown(t *testing.T) {
	app, err := New(Options{Assets: fstest.MapFS{"index.html": {Data: []byte("framework resources")}}, Lifecycle: lifecycle.Options{Permissions: []string{"compute"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = app.ConfigureMainWindow(WindowOptions{Title: "Test app", URL: "/", Width: 800, Height: 600}); err != nil {
		t.Fatal(err)
	}
	if err = Bind(app, "double", "compute", func(_ context.Context, v int) (int, error) { return v * 2, nil }); err != nil {
		t.Fatal(err)
	}
	if err = app.Handle("/custom", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "application route") })); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: time.Second}
	var origin string
	err = app.run(t.Context(), func(ctx context.Context, options host.Options) error {
		origin = strings.TrimSuffix(options.Window.URL, "/")
		for path, want := range map[string]string{"/": "framework resources", "/custom": "application route"} {
			resp, err := client.Get(origin + path)
			if err != nil {
				return err
			}
			b, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return err
			}
			if string(b) != want {
				t.Fatal(string(b), want)
			}
		}
		req, err := http.NewRequestWithContext(ctx, "POST", origin+omniweb.Prefix+"call", strings.NewReader(`{"v":1,"id":"test","method":"double","params":21}`))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", origin)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if !strings.Contains(string(body), `"result":42`) {
			t.Fatal(string(body))
		}
		if err := app.ConfigureMainWindow(WindowOptions{URL: "/"}); err == nil {
			t.Fatal("late window configuration accepted")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.Get(origin); err == nil {
		t.Fatal("resource server survived application")
	}
	if err = app.OpenContentWindow(t.Context(), "test", false); !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
	if err = app.run(t.Context(), nil); err == nil {
		t.Fatal("application reused")
	}
}
func TestApplicationNativeOnlyAndConfigurationValidation(t *testing.T) {
	app, err := New(Options{InitialWindow: "counter", CreateWindow: func(string, bool, host.WindowServices) (host.ContentWindowOptions, error) {
		return host.ContentWindowOptions{}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err = app.run(t.Context(), func(ctx context.Context, o host.Options) error {
		if o.InitialWindow != "counter" || o.Window.URL != "" {
			t.Fatal("native app acquired Web UI")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"https://example.com/", "//example.com/", "relative"} {
		a, _ := New(Options{Assets: fstest.MapFS{}})
		if err := a.ConfigureMainWindow(WindowOptions{URL: url}); err == nil {
			t.Fatal(url)
		}
	}
}
