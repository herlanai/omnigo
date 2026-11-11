package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"omnigo.dev/omnigo/omniweb"
	"omnigo.dev/omnigo/runtime/rpc"
	"omnigo.dev/omnigo/studio"
	"omnigo.dev/omnigo/studio/internal/catalog"
	"omnigo.dev/omnigo/studio/internal/native"
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
	if !native.Available {
		return native.ErrUnsupported
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	address := listener.Addr().String()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	router := rpc.New(32, 64<<10)
	type empty struct{}
	type overview struct {
		Entries  []catalog.Entry `json:"entries"`
		Native   bool            `json:"native"`
		Platform string          `json:"platform"`
	}
	if err := rpc.Bind(router, "studio.catalog", "", func(context.Context, empty) (overview, error) {
		return overview{catalog.Entries(), native.Available, runtime.GOOS + "/" + runtime.GOARCH}, nil
	}); err != nil {
		return err
	}
	origins := []string{"http://" + address}
	bridge, err := omniweb.NewHTTP(router, omniweb.HTTPOptions{Host: address, Origins: origins})
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle(omniweb.Prefix, bridge)
	fsys := studio.Assets()
	if *assets != "" {
		fsys = os.DirFS(*assets)
	}
	files, err := omniweb.Assets(fsys)
	if err != nil {
		return errors.New("Studio assets missing; run pnpm build before starting the client")
	}
	mux.Handle("/", files)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	serverDone := make(chan error, 1)
	err = native.Run(ctx, "http://"+address, cancel, func(host *native.Host) {
		type request struct {
			ID   string `json:"id"`
			Dark bool   `json:"dark"`
		}
		type reply struct {
			Opened  bool   `json:"opened"`
			Message string `json:"message"`
		}
		if err := rpc.Bind(router, "studio.open", "", func(ctx context.Context, r request) (reply, error) {
			valid := false
			for _, e := range catalog.Entries() {
				if e.Demo == r.ID && r.ID != "" {
					valid = true
					break
				}
			}
			if !valid {
				return reply{Message: "此项尚无可执行演示。"}, nil
			}
			if !native.Available {
				return reply{Message: "Studio 客户端目前仅支持 macOS Apple Silicon。"}, nil
			}
			call, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err := host.Open(call, r.ID, r.Dark); err != nil {
				return reply{Message: err.Error()}, nil
			}
			return reply{Opened: true, Message: "已打开 Go 原生演示窗口。"}, nil
		}); err != nil {
			serverDone <- err
			cancel()
			return
		}
		go func() {
			log.Printf("Omni Studio: http://%s (native demos: %t)", address, native.Available)
			err := server.Serve(listener)
			serverDone <- err
			cancel()
		}()
		go func() {
			<-ctx.Done()
			shutdown, stop := context.WithTimeout(context.Background(), 3*time.Second)
			defer stop()
			if err := server.Shutdown(shutdown); err != nil {
				_ = server.Close()
			}
		}()

	})
	if err != nil {
		return err
	}
	select {
	case err := <-serverDone:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-time.After(4 * time.Second):
		return errors.New("Studio HTTP shutdown timed out")
	}
	return nil
}
