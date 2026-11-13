// Package rpc implements transport-independent typed request handlers.
package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"sync"
)

var (
	ErrBusy           = errors.New("rpc: concurrency limit reached")
	ErrNotFound       = errors.New("rpc: method not found")
	ErrTooLarge       = errors.New("rpc: request too large")
	ErrForbidden      = errors.New("rpc: permission denied")
	ErrInternal       = errors.New("rpc: handler failed")
	ErrInvalidRequest = errors.New("rpc: invalid request")
)

type handler func(context.Context, []byte) ([]byte, error)
type route struct {
	request, response reflect.Type
	permission        string
	call              handler
}
type Router struct {
	mu       sync.RWMutex
	routes   map[string]route
	slots    chan struct{}
	maxBytes int
}

func New(maxConcurrent, maxBytes int) *Router {
	if maxConcurrent <= 0 {
		maxConcurrent = 64
	}
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	return &Router{routes: make(map[string]route), slots: make(chan struct{}, maxConcurrent), maxBytes: maxBytes}
}

func Bind[Request, Response any](r *Router, name, permission string, fn func(context.Context, Request) (Response, error)) error {
	if r == nil || fn == nil || name == "" {
		return errors.New("rpc: router, name and handler required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.routes == nil || r.slots == nil {
		return errors.New("rpc: initialize router with New")
	}
	if _, ok := r.routes[name]; ok {
		return fmt.Errorf("rpc: duplicate method %q", name)
	}
	r.routes[name] = route{request: reflect.TypeFor[Request](), response: reflect.TypeFor[Response](), permission: permission, call: func(ctx context.Context, raw []byte) ([]byte, error) {
		var req Request
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return nil, fmt.Errorf("%w: trailing request data", ErrInvalidRequest)
		}
		response, err := fn(ctx, req)
		if err != nil {
			return nil, err
		}
		return json.Marshal(response)
	}}
	return nil
}

// Call must receive grants from a trusted host policy, never from the request
// body. Cancellation is cooperative; a handler retains its slot until it exits.
func (r *Router) Call(ctx context.Context, name string, raw []byte, grants []string) (out []byte, err error) {
	if r == nil || ctx == nil || r.slots == nil {
		return nil, errors.New("rpc: initialized router and context required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(raw) > r.maxBytes {
		return nil, ErrTooLarge
	}
	r.mu.RLock()
	entry, ok := r.routes[name]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	allowed := entry.permission == ""
	for _, grant := range grants {
		allowed = allowed || grant == entry.permission
	}
	if !allowed {
		return nil, ErrForbidden
	}
	select {
	case r.slots <- struct{}{}:
	default:
		return nil, ErrBusy
	}
	defer func() {
		<-r.slots
		if recover() != nil {
			out = nil
			err = ErrInternal
		}
	}()
	return entry.call(ctx, raw)
}

// MethodType is a snapshot of a registered JSON method's Go types. It grants no
// authority and is used only by offline contract generation.
type MethodType struct {
	Name              string
	Request, Response reflect.Type
}

func (r *Router) MethodTypes() []MethodType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]MethodType, 0, len(r.routes))
	for name, route := range r.routes {
		out = append(out, MethodType{Name: name, Request: route.request, Response: route.response})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
