// Command chromium runs the optional standalone Omni Browser CEF host.
package main

import (
	"context"
	"flag"
	"github.com/herlanai/omnigo/omniui/plugins/chromium"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func main() {
	url := flag.String("url", "", "initial HTTP(S) URL; empty opens the offline home page")
	style := flag.String("style", "alloy", "alloy or chrome")
	host := flag.String("host", "", "CEF host executable")
	profile := flag.String("profile", "", "persistent profile directory")
	extensions := flag.String("extensions", "", "comma-separated unpacked extension paths (Chrome style)")
	app := flag.Bool("app", false, "content-only application window")
	ephemeral := flag.Bool("ephemeral", false, "disposable development profile; never use for credentials")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *host == "" && os.Getenv("OMNIGO_CHROMIUM_HOST") == "" {
		candidate := "build/chromium-runtime/Omni Browser.app/Contents/MacOS/Omni Browser"
		if runtime.GOOS == "windows" {
			candidate = "build/chromium-runtime/omnigo-chromium.exe"
		}
		if _, err := os.Stat(candidate); err == nil {
			*host, _ = filepath.Abs(candidate)
		}
	}
	opts := chromium.Options{HostPath: *host, URL: *url, Style: chromium.BrowserStyle(*style), ProfileDir: *profile, AppMode: *app, EphemeralDemo: *ephemeral, AppName: "Omni Browser"}
	if *extensions != "" {
		opts.Extensions = strings.Split(*extensions, ",")
	}
	b, err := chromium.Launch(ctx, opts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		stopCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = b.Close(stopCtx)
	}()
	for {
		select {
		case s, ok := <-b.Events():
			if !ok {
				if err := b.Wait(); err != nil {
					log.Print(err)
				}
				return
			}
			log.Printf("%s · %s · %d tabs", s.Title, s.URL, len(s.Tabs))
		case <-ctx.Done():
			return
		}
	}
}
