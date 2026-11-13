package omnigo

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/herlanai/omnigo/omniweb"
	"github.com/herlanai/omnigo/omniwindow/host"
)

func (a *Application) Handle(pattern string, handler http.Handler) (err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.used {
		return errors.New("omnigo: configure routes before Run")
	}
	if pattern == "/" || strings.HasPrefix(pattern, omniweb.Prefix) {
		return errors.New("omnigo: reserved framework route")
	}
	defer func() {
		if recover() != nil {
			err = errors.New("omnigo: invalid or conflicting HTTP route")
		}
	}()
	a.routes.Handle(pattern, handler)
	return nil
}

// Route adds an application HTTP handler, retaining framework route validation.
func Route(pattern string, handler http.Handler) Registration {
	return func(app *Application) error { return app.Handle(pattern, handler) }
}

func (a *Application) serveWindow(ctx context.Context, native func(context.Context, host.Options) error, options host.Options) error {
	files, err := omniweb.Assets(a.options.Assets)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	origin := "http://" + listener.Addr().String()
	bridge, err := omniweb.NewHTTP(a.router, omniweb.HTTPOptions{Streams: a.options.Streams, Events: a.options.Events, Host: listener.Addr().String(), Origins: []string{origin}, Grants: a.options.Lifecycle.Permissions})
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle(omniweb.Prefix, bridge)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler, pattern := a.routes.Handler(r)
		if pattern != "" {
			handler.ServeHTTP(w, r)
		} else {
			files.ServeHTTP(w, r)
		}
	}))
	lifetime, cancel := context.WithCancel(ctx)
	defer cancel()
	server := &http.Server{BaseContext: func(net.Listener) context.Context { return lifetime }, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	defer server.Close()
	done := make(chan error, 1)
	go func() { err := server.Serve(listener); done <- err; cancel() }()
	options.Window = *a.window
	options.Window.URL = origin + a.window.URL
	nativeErr := native(lifetime, options)
	cancel()
	shutdown, stop := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	shutdownErr := server.Shutdown(shutdown)
	stop()
	if shutdownErr != nil {
		_ = server.Close()
	}
	serveErr := <-done
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	return errors.Join(nativeErr, shutdownErr, serveErr)
}
