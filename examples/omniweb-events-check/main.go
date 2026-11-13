// omniweb-events-check opens a real framework WebView and tests the canonical
// SDK against Go services. Supply JavaScript compiled from omniweb/client/index.ts.
package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"testing/fstest"
	"time"

	"github.com/herlanai/omnigo"
	"github.com/herlanai/omnigo/omniweb"
)

//go:embed probe.js
var probe []byte

type result struct {
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

func main() {
	sdk := flag.String("sdk", "", "JavaScript compiled from canonical OmniWeb SDK")
	flag.Parse()
	if err := run(*sdk); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("WKWebView OmniWeb events: Unicode/order/once/permissions/cancel/close passed")
}
func run(path string) error {
	if path == "" {
		return errors.New("-sdk compiled canonical SDK path required")
	}
	sdk, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	events := omniweb.NewEvents()
	defer events.Close()
	if err := events.Register("progress", "read"); err != nil {
		return err
	}
	if err := events.Register("private", "secret"); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reports := make(chan result, 1)
	app, err := omnigo.New(omnigo.Options{
		Events:    events,
		Lifecycle: omnigo.LifecycleOptions{Permissions: []string{"read"}},
		Assets: fstest.MapFS{
			"index.html": {Data: []byte(`<!doctype html><meta charset="utf-8"><title>OmniWeb event check</title><p id="status">Checking live Go events…</p><script type="module" src="/probe.js"></script>`)},
			"probe.js":   {Data: probe}, "sdk.js": {Data: sdk},
		},
		Window: &omnigo.WindowOptions{Title: "OmniWeb event check", URL: "/", Width: 600, Height: 240},
		Registrations: []omnigo.Registration{
			omnigo.Method("emit", func(context.Context, struct{}) (int, error) {
				for _, s := range []string{"中文😀", "line\nbreak", "last"} {
					if _, err := events.Publish("progress", s); err != nil {
						return 0, err
					}
				}
				return 3, nil
			}, "read"),
			omnigo.Query("stop", func() bool { events.Close(); return true }, "read"),
			omnigo.Method("report", func(_ context.Context, r result) (bool, error) {
				select {
				case reports <- r:
					cancel()
					return true, nil
				default:
					return false, errors.New("duplicate report")
				}
			}),
			// Do not depend on platform MIME databases to serve JavaScript modules.
			omnigo.Route("/sdk.js", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
				w.Write(sdk)
			})),
			omnigo.Route("/probe.js", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
				w.Write(probe)
			})),
		},
	})
	if err != nil {
		return err
	}
	err = app.Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	select {
	case report := <-reports:
		if !report.Passed {
			return fmt.Errorf("WebView probe: %s", report.Detail)
		}
		return nil
	default:
		return fmt.Errorf("WebView report missing: %v", ctx.Err())
	}
}
