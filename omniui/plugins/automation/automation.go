// Package automation exposes an explicitly started, local JSON-lines endpoint.
// Every handler runs through the host's UI dispatch queue. Import has no effects.
package automation

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/herlanai/omnigo/internal/runtime/dispatch"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"image/png"
	"net"
	"os"
	"path/filepath"
	"sync"
)

type Request struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}
type response struct {
	ID     json.RawMessage `json:"id"`
	Result any             `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}
type Handler func(Request) (any, error)
type Server struct {
	listener    net.Listener
	queue       *dispatch.Queue
	handler     Handler
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	connections map[net.Conn]bool
	closed      bool
	wg          sync.WaitGroup
}

// Listen requires an application-owned private directory (0700 on Unix). It
// never deletes an existing socket or modifies a caller's directory permissions.
func Listen(ctx context.Context, path string, queue *DispatchQueue, handler Handler) (*Server, error) {
	if queue == nil || handler == nil || ctx == nil {
		return nil, errors.New("automation: context, queue and handler required")
	}
	info, e := os.Stat(filepath.Dir(path))
	if e != nil {
		return nil, e
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("automation: socket directory must be private")
	}
	l, e := net.Listen("unix", path)
	if e != nil {
		return nil, e
	}
	if e = os.Chmod(path, 0600); e != nil {
		l.Close()
		return nil, e
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Server{listener: l, queue: queue, handler: handler, ctx: ctx, cancel: cancel, connections: make(map[net.Conn]bool)}
	s.wg.Add(1)
	go s.accept()
	go func() { <-ctx.Done(); s.Close() }()
	return s, nil
}
func (s *Server) accept() {
	defer s.wg.Done()
	for {
		c, e := s.listener.Accept()
		if e != nil {
			return
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			c.Close()
			return
		}
		s.connections[c] = true
		s.wg.Add(1)
		s.mu.Unlock()
		go s.serve(c)
	}
}
func (s *Server) serve(c net.Conn) {
	defer s.wg.Done()
	defer func() { c.Close(); s.mu.Lock(); delete(s.connections, c); s.mu.Unlock() }()
	scanner := bufio.NewScanner(c)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	encoder := json.NewEncoder(c)
	for scanner.Scan() {
		var req Request
		if e := json.Unmarshal(scanner.Bytes(), &req); e != nil {
			if encoder.Encode(response{Error: "invalid request"}) != nil {
				return
			}
			continue
		}
		reply := response{ID: req.ID}
		done, e := s.queue.Submit(s.ctx, func() error { var err error; reply.Result, err = s.handler(req); return err })
		if e == nil {
			select {
			case e = <-done:
			case <-s.ctx.Done():
				return
			}
		}
		if e != nil {
			reply.Error = e.Error()
			reply.Result = nil
		}
		if encoder.Encode(reply) != nil {
			return
		}
	}
}
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.cancel()
	e := s.listener.Close()
	for c := range s.connections {
		c.Close()
	}
	s.mu.Unlock()
	return e
}

// Wait is called by the worker/owner after Close, never from a UI handler.
func (s *Server) Wait() { s.wg.Wait() }

// SurfaceHandler handles native controls only. Rebuild is called after input;
// the caller retains ownership of the surface and all declarative state.
func SurfaceHandler(surface *ui.Surface, rebuild func() error) Handler {
	return func(r Request) (any, error) {
		decode := func(v any) error { return json.Unmarshal(r.Params, v) }
		var err error
		switch r.Method {
		case "snapshot":
			return surface.Accessibility(), nil
		case "capture":
			img, e := surface.Render()
			if e != nil {
				return nil, e
			}
			var b bytes.Buffer
			if e = png.Encode(&b, img); e != nil {
				return nil, e
			}
			return base64.StdEncoding.EncodeToString(b.Bytes()), nil
		case "key":
			var v ui.KeyEvent
			if err = decode(&v); err == nil {
				err = surface.Key(v)
			}
		case "pointer":
			var v ui.PointerEvent
			if err = decode(&v); err == nil {
				err = surface.Pointer(v)
			}
		case "wheel":
			var v ui.WheelEvent
			if err = decode(&v); err == nil {
				err = surface.Wheel(v)
			}
		case "text":
			var v string
			if err = decode(&v); err == nil {
				err = surface.TextInput(v)
			}
		default:
			return nil, errors.New("automation: unknown method")
		}
		if err == nil && rebuild != nil {
			err = rebuild()
		}
		return err == nil, err
	}
}

// DispatchQueue is a bounded UI work queue. The application must consume Wake
// and call Drain on its UI goroutine; workers may submit but must not drain it.
type DispatchQueue = dispatch.Queue

// NewDispatchQueue creates an application-owned queue for automation handlers.
func NewDispatchQueue(capacity int) *DispatchQueue { return dispatch.New(capacity) }
