package omniweb

import (
	"context"
	"github.com/herlanai/omnigo/internal/runtime/rpc"
)

// Router dispatches typed calls with request limits and permission checks.
// Applications use this public entry point without importing framework internals.
type Router = rpc.Router

// NewRouter creates a router. Nonpositive limits select the framework defaults.
func NewRouter(maxConcurrent, maxBytes int) *Router { return rpc.New(maxConcurrent, maxBytes) }

// Bind registers one statically typed handler on the Web application's router.
func Bind[Request, Response any](router *Router, name, permission string, handler func(context.Context, Request) (Response, error)) error {
	return rpc.Bind(router, name, permission, handler)
}

var (
	ErrBusy           = rpc.ErrBusy
	ErrNotFound       = rpc.ErrNotFound
	ErrTooLarge       = rpc.ErrTooLarge
	ErrForbidden      = rpc.ErrForbidden
	ErrInternal       = rpc.ErrInternal
	ErrInvalidRequest = rpc.ErrInvalidRequest
)
