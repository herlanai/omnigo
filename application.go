// Package omnigo builds desktop applications from framework windows, Go content
// and typed services. Native resources and transports belong to the framework.
package omnigo

import (
	"errors"
	"io/fs"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/herlanai/omnigo/apps"
	"github.com/herlanai/omnigo/apps/host"
	"github.com/herlanai/omnigo/omniui"
	"github.com/herlanai/omnigo/runtime/lifecycle"
	"github.com/herlanai/omnigo/runtime/rpc"
)

type WindowOptions = apps.WindowOptions
type Point = apps.Point

const (
	TitleBarDefault     = apps.TitleBarDefault
	TitleBarHidden      = apps.TitleBarHidden
	TitleBarHiddenInset = apps.TitleBarHiddenInset
)

var ErrNotReady = errors.New("omnigo: native host is not ready")

type Options struct {
	// Window configures the primary Web window during construction.
	Window *WindowOptions
	// GoWindow creates the initial native content on the UI goroutine.
	GoWindow      func(WindowEnvironment) (ContentWindowOptions, error)
	Services      []Service
	Registrations []Registration
	Assets        fs.FS
	Icon          []byte
	CreateWindow  host.WindowFactory
	InitialWindow string
	Lifecycle     lifecycle.Options
}
type Application struct {
	mu        sync.Mutex
	used      bool
	options   Options
	window    *WindowOptions
	routes    *http.ServeMux
	router    *rpc.Router
	host      atomic.Pointer[host.Controller]
	lifecycle *lifecycle.Manager
}

func New(options Options) (*Application, error) {
	if options.GoWindow != nil {
		if options.InitialWindow != "" || options.CreateWindow != nil {
			return nil, errors.New("omnigo: GoWindow conflicts with window factory")
		}
		create := options.GoWindow
		options.InitialWindow = "main"
		options.CreateWindow = func(_ string, dark bool, services host.WindowServices) (host.ContentWindowOptions, error) {
			return create(WindowEnvironment{WindowServices: services, Dark: dark})
		}
	}
	// Snapshot the extension plan; application services never mutate caller registries.
	registry := &omniui.Registry{}
	if options.Lifecycle.Registry != nil {
		plan, err := options.Lifecycle.Registry.Resolve(options.Lifecycle.Extensions, options.Lifecycle.Permissions)
		if err != nil {
			return nil, err
		}
		for _, e := range plan {
			if err := registry.Register(e); err != nil {
				return nil, err
			}
		}
	}
	for _, service := range options.Services {
		if err := registry.Register(omniui.Extension{Descriptor: omniui.Descriptor{ID: service.ID, Version: "1", Kind: omniui.Service}, Start: service.Start, Stop: service.Stop}); err != nil {
			return nil, err
		}
		options.Lifecycle.Extensions = append(append([]string(nil), options.Lifecycle.Extensions...), service.ID)
	}
	options.Lifecycle.Registry = registry
	if options.Assets == nil && (options.InitialWindow == "" || options.CreateWindow == nil) {
		return nil, errors.New("omnigo: assets or an initial Go content window are required")
	}
	options.Icon = append([]byte(nil), options.Icon...)
	options.Lifecycle.Permissions = append([]string(nil), options.Lifecycle.Permissions...)
	app := &Application{options: options, routes: http.NewServeMux(), router: rpc.New(32, 64<<10), lifecycle: lifecycle.New(options.Lifecycle)}
	if options.Window != nil {
		if options.GoWindow != nil {
			return nil, errors.New("omnigo: choose Window or GoWindow for the primary window")
		}
		if err := app.ConfigureMainWindow(*options.Window); err != nil {
			return nil, err
		}
	}
	for _, module := range options.Registrations {
		if module == nil {
			return nil, errors.New("omnigo: nil registration")
		}
		if err := module(app); err != nil {
			return nil, err
		}
	}
	return app, nil
}
