// omniweb-streams-check opens a real framework WebView and tests the canonical
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
	"strings"
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
	fmt.Println("WKWebView OmniWeb streams: multi-window Unicode/order/result/permissions/cancel/navigation/shutdown passed")
}
func run(path string) error {
	if path == "" {
		return errors.New("-sdk compiled canonical SDK path required")
	}
	sdk, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	streams := omniweb.NewStreams(4)
	count := func(ctx context.Context, n int, ch *omniweb.Channel[string]) (int, error) {
		for i := 0; i < n; i++ {

			if err := ch.Send(strings.Repeat("中文😀", 8192)); err != nil {
				return i, err
			}
		}

		return n, nil
	}
	if err := omniweb.BindStream(streams, "count", "read", count); err != nil {
		return err
	}
	if err := omniweb.BindStream(streams, "private", "secret", count); err != nil {
		return err
	}
	stopped := make(chan struct{})
	entered := make(chan struct{})
	if err := omniweb.BindStream(streams, "hold", "read", func(ctx context.Context, _ struct{}, ch *omniweb.Channel[string]) (bool, error) {
		close(entered)
		<-ctx.Done()
		close(stopped)
		return false, ctx.Err()
	}); err != nil {
		return err
	}
	navigationEntered, navigationStopped := make(chan struct{}), make(chan struct{})
	if err := omniweb.BindStream(streams, "navigate", "read", func(ctx context.Context, _ struct{}, _ *omniweb.Channel[string]) (bool, error) {
		close(navigationEntered)
		<-ctx.Done()
		close(navigationStopped)
		return false, ctx.Err()
	}); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	reports := make(chan result, 1)
	app, err := omnigo.New(omnigo.Options{
		Streams:   streams,
		Lifecycle: omnigo.LifecycleOptions{Permissions: []string{"read"}},
		Assets: fstest.MapFS{
			"index.html": {Data: []byte(`<!doctype html><meta charset="utf-8"><title>OmniWeb stream check</title><p id="status">Checking live Go streams…</p><script type="module" src="/probe.js"></script>`)},
			"probe.js":   {Data: probe}, "sdk.js": {Data: sdk},
		},
		Window: &omnigo.WindowOptions{Title: "OmniWeb stream check", URL: "/", Width: 600, Height: 240},
		Registrations: []omnigo.Registration{
			omnigo.Method("navigationEntered", func(ctx context.Context, _ struct{}) (bool, error) {
				select {
				case <-navigationEntered:
					return true, nil
				case <-ctx.Done():
					return false, ctx.Err()
				}
			}),
			omnigo.Method("navigationStopped", func(ctx context.Context, _ struct{}) (bool, error) {
				select {
				case <-navigationStopped:
					return true, nil
				case <-ctx.Done():
					return false, ctx.Err()
				}
			}),
			omnigo.Method("stage", func(_ context.Context, s string) (bool, error) { fmt.Println("probe:", s); return true, nil }),
			omnigo.Method("report", func(_ context.Context, r result) (bool, error) {
				select {
				case reports <- r:
					if r.Passed {
						select {
						case <-entered:
						case <-ctx.Done():
							return false, ctx.Err()
						}
					}
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
		select {
		case <-stopped:
			return nil
		default:
			return errors.New("active stream handler survived application shutdown")
		}
	default:
		return fmt.Errorf("WebView report missing: %v", ctx.Err())
	}
}
