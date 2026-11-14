// Command native-rpc-smoke requires a real WKWebView or WebView2 runtime.
// It verifies native message/reply RPC and uses HTTP only for assets and completion.
package main

import (
	"context"
	"fmt"
	"github.com/herlanai/omnigo/omniweb"
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
		io.WriteString(w, `<!doctype html><meta charset="utf-8"><title>WebView check</title><p>Native WebView roundtrip</p><script>(async()=>{const post=window.webkit ? x=>window.webkit.messageHandlers.omnigoNative.postMessage(x) : x=>new Promise((resolve,reject)=>{const cb=e=>{if(e.data.token!=='smoke')return;chrome.webview.removeEventListener('message',cb);e.data.error?reject(Error(e.data.error)):resolve(e.data.result)};chrome.webview.addEventListener('message',cb);chrome.webview.postMessage(x)});const text='OmniGo 中文 🪟';const r=JSON.parse(await post(JSON.stringify({kind:'call',token:'smoke',request:{v:1,id:'1',method:'echo',params:text}})));if(r.result!==text)throw Error('bad echo');await fetch('/done',{method:'POST',body:'verified'});})()</script>`)
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
	router := omniweb.NewRouter(4, 1024)
	if err := omniweb.Bind(router, "echo", "", func(ctx context.Context, value string) (string, error) { return value, nil }); err != nil {
		return err
	}
	server := httptest.NewServer(mux)
	defer server.Close()
	err := host.Run(ctx, host.Options{NativeExchange: omniweb.NativeExchange(router, nil), Window: omniwindow.WindowOptions{Title: "WebView check", URL: server.URL, Width: 640, Height: 480}})
	if err != nil {
		return err
	}
	if !complete.Load() {
		return fmt.Errorf("native page did not complete roundtrip: %v", ctx.Err())
	}
	fmt.Println("PASS: native message/reply RPC, Unicode and host shutdown")
	return nil
}
