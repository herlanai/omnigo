// desktop-smoke verifies native startup, Go rendering and teardown using only
// public framework APIs. It briefly opens a real window and uses no sleeps.
package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/herlanai/omnigo"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/omniwindow/host"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Public OmniGo application: Go frame rendered, activation request completed and content closed")
}
func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rendered, closed := false, false
	painted := make(chan struct{})
	var paintOnce sync.Once
	app, err := omnigo.New(omnigo.Options{InitialWindow: "main", CreateWindow: func(string, bool, host.WindowServices) (host.ContentWindowOptions, error) {
		return host.ContentWindowOptions{Title: "OmniGo API smoke", Width: 360, Height: 180, View: func() *ui.Element {
			rendered = true
			paintOnce.Do(func() { close(painted) })
			return ui.Column("root", ui.Heading("title", "Public OmniGo API", 2), ui.Text("body", "Native framework window")).Padding(20).Gap(12)
		}, Close: func() { closed = true }}, nil
	}})
	if err != nil {
		return err
	}
	activation := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			activation <- ctx.Err()
			return
		case <-painted:
		}
		done, err := app.Activate(ctx)
		if err == nil {
			select {
			case err = <-done:
			case <-ctx.Done():
				err = ctx.Err()
			}
		}
		activation <- err
		cancel()
	}()
	if err = app.Run(ctx); err != nil {
		return err
	}
	if err := <-activation; err != nil {
		return err
	}
	if !rendered || !closed {
		return fmt.Errorf("incomplete lifecycle: rendered=%t closed=%t", rendered, closed)
	}
	return nil
}
