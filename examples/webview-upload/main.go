// Command webview-upload exercises HTML file inputs in both native WebView hosts.
package main

import (
	"embed"
	"flag"
	"log"

	"github.com/herlanai/omnigo"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/omniwindow/host"
)

//go:embed index.html
var assets embed.FS

func main() {
	embedded := flag.Bool("embedded", false, "use a WebView inside a Go layout")
	flag.Parse()
	options := omnigo.Options{Assets: assets, Window: &omnigo.WindowOptions{Title: "WebView Upload Check", Width: 800, Height: 650}}
	if *embedded {
		options.Window = nil
		options.GoWindow = func(omnigo.WindowEnvironment) (omnigo.ContentWindowOptions, error) {
			var state *ui.WebViewState
			html, _ := assets.ReadFile("index.html")
			return omnigo.ContentWindowOptions{Title: "Embedded Upload Check", Width: 800, Height: 650,
				WebView: &host.WebViewOptions{HTML: string(html), OnStateReady: func(value *ui.WebViewState) { state = value }},
				View: func() *ui.Element {
					if state == nil {
						return ui.Text("loading", "Loading WebView…")
					}
					return ui.Column("root", ui.WebView("upload", state).Grow(1)).Padding(16)
				},
			}, nil
		}
	}
	if err := omnigo.Run(options); err != nil {
		log.Fatal(err)
	}
}
