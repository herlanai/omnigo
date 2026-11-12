package nativewindow

import (
	"context"
	"errors"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/herlanai/omnigo/omniwindow"
	"github.com/herlanai/omnigo/runtime/dispatch"
)

type fakeBackend struct {
	events                [][]Event
	closed, wakes, frames int
	timeouts              []int
	beforeNext            func(int)
}

func (b *fakeBackend) next(timeout int) ([]Event, error) {
	b.timeouts = append(b.timeouts, timeout)
	if b.beforeNext != nil {
		b.beforeNext(timeout)
	}
	if len(b.events) == 0 {
		return []Event{{Kind: "close"}}, nil
	}
	e := b.events[0]
	b.events = b.events[1:]
	return e, nil
}
func (b *fakeBackend) present(*image.RGBA) error { b.frames++; return nil }
func (b *fakeBackend) wake()                     { b.wakes++ }
func (b *fakeBackend) close()                    { b.closed++ }
func TestLoopResizeDispatchAndResourceLifetime(t *testing.T) {
	b := &fakeBackend{events: [][]Event{{{Kind: "resize", Width: 400, Height: 200}}, {{Kind: "close"}}}}
	changes := 0
	var done <-chan error
	var sizes []image.Point
	w, _ := New(Options{Window: omniwindow.WindowOptions{Width: 200, Height: 100}, Ready: func(w *Window) error {
		var err error
		done, err = w.Post(context.Background(), func() error { changes++; return nil })
		return err
	}, Paint: func(w, h int) (*image.RGBA, error) {
		sizes = append(sizes, image.Pt(w, h))
		return image.NewRGBA(image.Rect(0, 0, w, h)), nil
	}})
	err := w.run(context.Background(), func(omniwindow.WindowOptions) (backend, error) { return b, nil })
	if err != nil || changes != 1 || <-done != nil || b.closed != 1 || b.frames != 2 || sizes[1] != image.Pt(400, 200) {
		t.Fatalf("err=%v changes=%d backend=%+v sizes=%v", err, changes, b, sizes)
	}
	if _, err = w.Post(context.Background(), func() error { return nil }); !errors.Is(err, dispatch.ErrClosed) {
		t.Fatal(err)
	}
	if err = w.Run(context.Background()); err == nil {
		t.Fatal("reused native lifecycle")
	}
}
func TestCancellationAndFailedPaintReleaseBackend(t *testing.T) {
	for _, cancelNow := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		b := &fakeBackend{}
		want := errors.New("paint failed")
		w, _ := New(Options{Ready: func(*Window) error {
			if cancelNow {
				cancel()
			}
			return nil
		}, Paint: func(int, int) (*image.RGBA, error) { return nil, want }})
		err := w.run(ctx, func(omniwindow.WindowOptions) (backend, error) { return b, nil })
		cancel()
		if cancelNow {
			want = context.Canceled
		}
		if !errors.Is(err, want) || b.closed != 1 {
			t.Fatalf("%v %+v", err, b)
		}
	}
}
func TestTimerAndIdleWait(t *testing.T) {
	b := &fakeBackend{events: [][]Event{nil, {{Kind: "close"}}}}
	now := time.Unix(0, 0)
	b.beforeNext = func(wait int) {
		if wait > 0 {
			now = now.Add(time.Duration(wait) * time.Millisecond)
		}
	}
	ticks := 0
	w, _ := New(Options{NextFrame: func() (time.Duration, bool) { return 7 * time.Millisecond, ticks == 0 }, Tick: func() error { ticks++; return nil }})
	w.now = func() time.Time { return now }
	if err := w.run(context.Background(), func(omniwindow.WindowOptions) (backend, error) { return b, nil }); err != nil {
		t.Fatal(err)
	}
	if ticks != 1 || b.timeouts[0] != 7 || b.timeouts[1] != -1 {
		t.Fatalf("ticks=%d waits=%v", ticks, b.timeouts)
	}
}
func TestNativeOptionsAndPixelBounds(t *testing.T) {
	for _, o := range []omniwindow.WindowOptions{{URL: "/"}, {TitleBarStyle: omniwindow.TitleBarHiddenInset}, {Width: 20000}, {TrafficLightPosition: &omniwindow.Point{X: 12, Y: 11}}} {
		if _, err := New(Options{Window: o}); err == nil {
			t.Fatal("unsupported options silently accepted", o)
		}
	}
	source := image.NewRGBA(image.Rect(2, 3, 4, 5))
	source.SetRGBA(3, 4, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	pixel := bgra(source.SubImage(image.Rect(3, 4, 4, 5)).(*image.RGBA))
	if string(pixel) != string([]byte{30, 20, 10, 255}) {
		t.Fatal(pixel)
	}
}
func TestUTF16Input(t *testing.T) {
	var d utf16Input
	if d.feed(0xd83d) != "" || d.feed(0xde00) != "😀" || d.feed('A') != "A" || d.feed(13) != "" || d.feed(0xdc00) != "�" {
		t.Fatal("invalid UTF-16 handling")
	}
}
