// Command omnibrowser runs the pure-Go document engine, without a native browser.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser/cdp"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser/mcp"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser/render"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: omnibrowser fetch [flags] URL | html [flags] FILE | scrape [flags] URL... | serve [flags] | mcp [flags]")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	concurrency := flags.Int("concurrency", 4, "parallel scrape sessions (maximum 32)")
	eval := flags.String("eval", "", "JavaScript expression (JSON output)")
	selector := flags.String("selector", "", "CSS selector (JSON output)")
	dump := flags.String("dump", "text", "text, html or links")
	pdf := flags.String("pdf", "", "write a raster viewport PDF")
	animationTime := flags.Duration("animation-time", 0, "CSS transform animation sample time; does not run JavaScript timers")
	shot := flags.String("screenshot", "", "write supported CSS viewport PNG")
	timeout := flags.Duration("timeout", 5*time.Second, "navigation/script deadline")
	allow := flags.Bool("allow-private-network", false, "allow local HTTP servers")
	addr := flags.String("addr", "127.0.0.1:9222", "loopback CDP address")
	token := flags.String("token", "", "CDP bearer token (random if omitted)")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if args[0] == "mcp" {
		return mcp.Serve(ctx, os.Stdin, os.Stdout, omnibrowser.Options{Timeout: *timeout, AllowPrivateNetwork: *allow})
	}
	page := omnibrowser.New(omnibrowser.Options{Timeout: *timeout, AllowPrivateNetwork: *allow})
	defer page.Close()
	if args[0] == "serve" {
		host, _, err := net.SplitHostPort(*addr)
		if err != nil {
			return err
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("CDP address must be a loopback IP")
		}
		if *token == "" {
			var b [24]byte
			if _, err = rand.Read(b[:]); err != nil {
				return err
			}
			*token = hex.EncodeToString(b[:])
		}
		if err = page.SetContent(ctx, "<title>OmniBrowser</title><h1>Ready</h1>"); err != nil {
			return err
		}
		listener, err := net.Listen("tcp", *addr)
		if err != nil {
			return err
		}
		server := &http.Server{Handler: cdp.Handler(page, *token), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
		fmt.Fprintf(os.Stderr, "OmniBrowser CDP subset at %s\nBearer token: %s\n", listener.Addr(), *token)
		go func() { <-ctx.Done(); _ = page.Close(); _ = server.Close() }()
		err = server.Serve(listener)
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
	if args[0] == "scrape" {
		if flags.NArg() == 0 {
			return fmt.Errorf("scrape requires URLs")
		}
		return json.NewEncoder(os.Stdout).Encode(omnibrowser.Scrape(ctx, flags.Args(), *concurrency, omnibrowser.Options{Timeout: *timeout, AllowPrivateNetwork: *allow}, *eval))
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("one URL or HTML file required; put flags before the URL/file")
	}
	switch args[0] {
	case "fetch":
		if err := page.Navigate(ctx, flags.Arg(0)); err != nil {
			return err
		}
	case "html":
		data, err := os.ReadFile(flags.Arg(0))
		if err != nil {
			return err
		}
		if err = page.SetContent(ctx, string(data)); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	snapshot, err := page.Snapshot(ctx)
	if err != nil {
		return err
	}
	for _, message := range snapshot.ScriptErrors {
		fmt.Fprintln(os.Stderr, "Page script error:", message)
	}
	if *eval != "" {
		value, err := page.Eval(ctx, *eval)
		if err != nil {
			return err
		}
		fmt.Println(string(value))
		snapshot, err = page.Snapshot(ctx)
		if err != nil {
			return err
		}
	} else if *selector != "" {
		matches, err := page.Query(ctx, *selector)
		if err != nil {
			return err
		}
		if err = json.NewEncoder(os.Stdout).Encode(matches); err != nil {
			return err
		}
	} else {
		switch *dump {
		case "text":
			fmt.Println(snapshot.Text)
		case "html":
			fmt.Println(snapshot.HTML)
		case "links":
			matches, err := page.Query(ctx, "a[href]")
			if err != nil {
				return err
			}
			_ = json.NewEncoder(os.Stdout).Encode(matches)
		default:
			return fmt.Errorf("unsupported dump %s", *dump)
		}
	}
	if *pdf != "" {
		data, err := render.PDF(snapshot, 794, 1123)
		if err != nil {
			return err
		}
		if err = os.WriteFile(*pdf, data, 0600); err != nil {
			return err
		}
	}
	if strings.TrimSpace(*shot) != "" {
		img, err := render.ScreenshotAt(snapshot, 1024, 768, false, *animationTime)
		if err != nil {
			return err
		}
		file, err := os.Create(*shot)
		if err != nil {
			return err
		}
		err = png.Encode(file, img)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
	return nil
}
