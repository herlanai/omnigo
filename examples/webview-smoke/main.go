// Command webview-smoke requires a real desktop/WebView runtime. It verifies
// page JavaScript and a Unicode HTTP roundtrip, not merely window creation.
package main

import (
	"context"
	"fmt"
	"github.com/herlanai/omnigo/omniwindow"
	"github.com/herlanai/omnigo/omniwindow/host"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var complete atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, `<!doctype html><meta charset="utf-8"><title>WebView check</title><p>Native WebView roundtrip</p><script>(async()=>{const text='OmniGo 中文 🪟';const r=await fetch('/echo',{method:'POST',body:text});if(await r.text()!==text)throw Error('bad echo');await fetch('/done',{method:'POST',body:'verified'});})()</script>`)
	})
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method", 405)
			return
		}
		data, err := io.ReadAll(io.LimitReader(r.Body, 4096))
		if err != nil {
			http.Error(w, "read", 400)
			return
		}
		w.Write(data)
	})
	mux.HandleFunc("/done", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(io.LimitReader(r.Body, 32))
		if r.Method == "POST" && string(data) == "verified" {
			complete.Store(true)
			cancel()
		} else {
			http.Error(w, "invalid result", 400)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	err := host.Run(ctx, host.Options{Window: omniwindow.WindowOptions{Title: "WebView check", URL: server.URL, Width: 640, Height: 480}})
	if err != nil {
		return err
	}
	if !complete.Load() {
		return fmt.Errorf("native page did not complete roundtrip: %v", ctx.Err())
	}
	fmt.Println("PASS: native page JavaScript, Unicode HTTP roundtrip and host shutdown")
	return nil
}
