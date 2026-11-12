package permissionflow

import (
	"context"
	"errors"
	"testing"
)

func TestUnknownPermissionAndCanceledRequestNeverPrompt(t *testing.T) {
	if _, e := Check("unknown"); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Request(ctx, ScreenRecording); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
