// Command webviewcheck opens and closes one native window to verify an embedded
// WKWebView loads a document, remains a child view, and hides on unmount.
package main

import (
	"fmt"
	"github.com/herlanai/omnigo/omniwindow/host"
	"os"
)

func main() {
	if err := host.CheckEmbeddedWeb(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Native WebView: document loaded, child view attached, hidden on unmount")
}
