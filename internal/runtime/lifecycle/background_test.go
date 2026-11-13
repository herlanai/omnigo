package lifecycle

import (
	"context"
	"testing"
)

func TestBackgroundPanicAndReuse(t *testing.T) {
	start, stop := Background(func(context.Context) error { panic("boom") })
	ctx, cancel := context.WithCancel(t.Context())
	if err := start(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := stop(t.Context()); err == nil {
		t.Fatal("panic was lost")
	}
	if err := start(t.Context()); err == nil {
		t.Fatal("reused service")
	}
}
