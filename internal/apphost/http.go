package apphost

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"time"

	"github.com/herlanai/omnigo/internal/runtime/rpc"
	"github.com/herlanai/omnigo/omniweb"
	"github.com/herlanai/omnigo/omniwindow"
	"github.com/herlanai/omnigo/omniwindow/host"
)

// WebOptions supplies the already-configured application resources. This layer
// owns the loopback server lifetime, not public application configuration.
type WebOptions struct {
	Assets      fs.FS
	Router      *rpc.Router
	Routes      *http.ServeMux
	Streams     *omniweb.Streams
	Events      *omniweb.Events
	Permissions []string
	Window      omniwindow.WindowOptions
}

// ServeWindow keeps application transport resources alive until native exit,
// then cancels requests and performs bounded HTTP cleanup.
func ServeWindow(ctx context.Context, native func(context.Context, host.Options) error, options host.Options, web WebOptions) error {
	files, err := omniweb.Assets(web.Assets)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	origin := "http://" + listener.Addr().String()
	bridge, err := omniweb.NewHTTP(web.Router, omniweb.HTTPOptions{Streams: web.Streams, Events: web.Events, Host: listener.Addr().String(), Origins: []string{origin}, Grants: web.Permissions})
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle(omniweb.Prefix, bridge)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler, pattern := web.Routes.Handler(r)
		if pattern != "" {
			handler.ServeHTTP(w, r)
		} else {
			files.ServeHTTP(w, r)
		}
	}))
	lifetime, cancel := context.WithCancel(ctx)
	defer cancel()
	var pending pendingConnections
	server := &http.Server{BaseContext: func(net.Listener) context.Context { return lifetime }, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	server.ConnState = pending.state
	defer server.Close()
	done := make(chan error, 1)
	go func() { err := server.Serve(listener); done <- err; cancel() }()
	options.Window = web.Window
	options.Window.URL = origin + web.Window.URL
	nativeErr := native(lifetime, options)
	cancel()
	pending.stop()
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
