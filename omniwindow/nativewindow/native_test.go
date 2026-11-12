package nativewindow

import (
	"context"
	"errors"
	"image"
	"image/color"
	"os"
	"testing"
)

// Explicit opt-in: this test creates a real OS window and submits a frame. It
// does not certify IME, pixel correctness, DPI or compositor interoperability.
func TestNativeWindowLifecycle(t *testing.T) {
	if os.Getenv("OMNIGO_NATIVE_TEST") != "1" {
		t.Skip("set OMNIGO_NATIVE_TEST=1 on Windows or an X11 desktop")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frames := 0
	w, err := New(Options{Paint: func(width, height int) (*image.RGBA, error) {
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				img.SetRGBA(x, y, color.RGBA{R: 32, G: 64, B: 128, A: 255})
			}
		}
		frames++
		cancel()
		return img, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err = w.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if frames != 1 {
		t.Fatalf("submitted frames: %d", frames)
	}
}

func TestNativeOwnedChildLifecycle(t *testing.T) {
	if os.Getenv("OMNIGO_NATIVE_TEST") != "1" {
		t.Skip("explicit native owned-window test")
	}
	ctx := context.Background()
	childFrames := 0
	var child *Window
	parent, err := New(Options{Ready: func(p *Window) error {
		var err error
		child, err = p.NewChild(Options{Paint: func(width, height int) (*image.RGBA, error) {
			childFrames++
			if _, err := p.Close(ctx); err != nil {
				return nil, err
			}
			return image.NewRGBA(image.Rect(0, 0, width, height)), nil
		}})
		return err
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err = parent.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if childFrames == 0 || child == nil || !child.closed {
		t.Fatal("child lifetime not completed")
	}
}
