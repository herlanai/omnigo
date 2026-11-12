// desktop-smoke verifies native startup, Go rendering and teardown using only
// public framework APIs. It briefly opens a real window and uses no sleeps.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/herlanai/omnigo"
	"github.com/herlanai/omnigo/omniwindow/host"
	ui "github.com/herlanai/omnigo/omniui/baseui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Public OmniGo application: Go frame rendered and content closed")
}
func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rendered, closed := false, false
	app, err := omnigo.New(omnigo.Options{InitialWindow: "main", CreateWindow: func(string, bool, host.WindowServices) (host.ContentWindowOptions, error) {
		return host.ContentWindowOptions{Title: "OmniGo API smoke", Width: 360, Height: 180, View: func() *ui.Element {
			rendered = true
			cancel()
			return ui.Column("root", ui.Heading("title", "Public OmniGo API", 2), ui.Text("body", "Native framework window")).Padding(20).Gap(12)
		}, Close: func() { closed = true }}, nil
	}})
	if err != nil {
		return err
	}
	if err = app.Run(ctx); err != nil {
		return err
	}
	if !rendered || !closed {
		return fmt.Errorf("incomplete lifecycle: rendered=%t closed=%t", rendered, closed)
	}
	return nil
}
