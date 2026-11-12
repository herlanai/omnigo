package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/herlanai/omnigo"
	"github.com/herlanai/omnigo/apps/host"
	"github.com/herlanai/omnigo/apps/metrics"
	"github.com/herlanai/omnigo/studio"
	"github.com/herlanai/omnigo/studio/internal/browser"
	"github.com/herlanai/omnigo/studio/internal/demos"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	assets := flag.String("assets", "", "optional directory overriding the embedded Studio assets")
	flag.Parse()
	if !host.Available {
		return host.ErrUnsupported
	}
	fsys := studio.Assets()
	if *assets != "" {
		fsys = os.DirFS(*assets)
	}
	browsers := browser.New(context.Background())
	monitor := metrics.NewMonitor()
	return omnigo.Run(omnigo.Options{
		Window: &omnigo.WindowOptions{Title: "Omni Studio", Width: 1280, Height: 820, MinWidth: 860, MinHeight: 640},
		Assets: fsys, Icon: studio.ApplicationIcon(), CreateWindow: demos.Window,
		Services: []omnigo.Service{
			{ID: "studio.browser", Stop: func(context.Context) error { browsers.Close(); return nil }},
			omnigo.Background("studio.metrics", monitor.Run),
		},
		Registrations: []omnigo.Registration{studioRegistration(browsers, monitor)},
	})
}
