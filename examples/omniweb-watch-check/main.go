// omniweb-watch-check validates the polling watch service in a real WebView.
package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing/fstest"
	"time"

	"github.com/herlanai/omnigo"
	"github.com/herlanai/omnigo/omniui/plugins/watch"
	"github.com/herlanai/omnigo/omniweb"
)

//go:embed probe.js
var probe []byte

type report struct {
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

func main() {
	sdk := flag.String("sdk", "", "JavaScript compiled from canonical omniweb/client/index.ts")
	flag.Parse()
	if err := run(*sdk); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("WKWebView watch: ready/create/write/rename/remove/ignore/deny/cancel/navigation/shutdown passed")
}
func run(sdkPath string) error {
	if sdkPath == "" {
		return errors.New("-sdk required")
	}
	sdk, err := os.ReadFile(sdkPath)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "omnigo-watch-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	stopped := map[string]chan struct{}{}
	for _, name := range []string{"normal", "navigation", "shutdown"} {
		if err = os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
			return err
		}
		stopped[name] = make(chan struct{})
	}
	single := filepath.Join(dir, "single.txt")
	if err = os.WriteFile(single, []byte("initial"), 0600); err != nil {
		return err
	}
	stopped["single"] = make(chan struct{})
	service, err := watch.NewService(watch.ServiceOptions{
		Roots: map[string]watch.Root{"single": {Path: single}, "shallow": {Path: dir}, "project": {Path: dir, Recursive: true, Ignore: func(path string, _ bool) bool { return strings.HasSuffix(path, ".tmp") }}},
		Allow: func(ctx context.Context, r watch.Request) bool {
			key := r.Path
			if r.Root == "single" {
				key = "single"
			}
			signal, ok := stopped[key]
			if !ok {
				return false
			}
			context.AfterFunc(ctx, func() { close(signal) })
			return true
		},
	})
	if err != nil {
		return err
	}
	defer service.Close()
	streams := omniweb.NewStreams(8)
	if err = service.Bind(streams, "watch.observe", "files.watch"); err != nil {
		return err
	}
	if err = service.Bind(streams, "watch.private", "private"); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	reports := make(chan report, 1)
	script := func(data []byte) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Write(data)
		})
	}
	app, err := omnigo.New(omnigo.Options{
		Streams:   streams,
		Lifecycle: omnigo.LifecycleOptions{Permissions: []string{"files.watch"}},
		Assets:    fstest.MapFS{"index.html": {Data: []byte(`<!doctype html><meta charset="utf-8"><title>OmniWeb watch check</title><p id="status">Checking file events…</p><script type="module" src="/probe.js"></script>`)}},
		Window:    &omnigo.WindowOptions{Title: "OmniWeb watch check", URL: "/", Width: 640, Height: 240},
		Registrations: []omnigo.Registration{
			omnigo.Route("/sdk.js", script(sdk)), omnigo.Route("/probe.js", script(probe)),
			omnigo.Method("stage", func(_ context.Context, s string) (bool, error) { fmt.Println("probe:", s); return true, nil }),
			omnigo.Method("mutate", func(_ context.Context, step string) (bool, error) {
				initial := filepath.Join(dir, "normal", "文件.txt")
				moved := filepath.Join(dir, "normal", "moved.txt")
				var err error
				switch step {
				case "file-write":
					err = os.WriteFile(single, []byte("updated file contents"), 0600)
				case "file-remove":
					err = os.Remove(single)
				case "create":
					if err = os.WriteFile(filepath.Join(dir, "normal", "ignored.tmp"), []byte("ignored"), 0600); err == nil {
						err = os.WriteFile(initial, []byte("hello"), 0600)
					}
				case "write":
					err = os.WriteFile(initial, []byte("updated content"), 0600)
				case "rename":
					err = os.Rename(initial, moved)
				case "remove":
					err = os.Remove(moved)
				default:
					return false, errors.New("invalid probe mutation")
				}
				return err == nil, err
			}),
			omnigo.Method("stopped", func(ctx context.Context, name string) (bool, error) {
				signal, ok := stopped[name]
				if !ok {
					return false, errors.New("unknown probe watch")
				}
				select {
				case <-signal:
					return true, nil
				case <-ctx.Done():
					return false, ctx.Err()
				}
			}),
			omnigo.Method("report", func(_ context.Context, r report) (bool, error) {
				select {
				case reports <- r:
					cancel()
					return true, nil
				default:
					return false, errors.New("duplicate report")
				}
			}),
		},
	})
	if err != nil {
		return err
	}
	if err = app.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	select {
	case r := <-reports:
		if !r.Passed {
			return fmt.Errorf("WebView probe: %s", r.Detail)
		}
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-stopped["shutdown"]:
			return nil
		case <-timer.C:
			return errors.New("shutdown did not cancel watch context")
		}
	default:
		return fmt.Errorf("WebView report missing: %v", ctx.Err())
	}
}
