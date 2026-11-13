package omnigo

import (
	"context"

	"github.com/herlanai/omnigo/internal/runtime/lifecycle"
)

// Service starts before the native host and stops in reverse order after it.
// Start must return promptly and clean up partial work on error. Stop receives
// the independent, bounded shutdown context from lifecycle.Manager.
type Service struct {
	ID    string
	Start func(context.Context) error
	Stop  func(context.Context) error
}

// Background adapts cooperative background work to the application lifetime.
// Its result is reported when shutdown joins it. Work must return on cancellation;
// a shutdown timeout reports an error but cannot forcibly terminate a goroutine.
// Each returned service is single-use and belongs to one application.
func Background(id string, work func(context.Context) error) Service {
	start, stop := lifecycle.Background(work)
	return Service{ID: id, Start: start, Stop: stop}
}
