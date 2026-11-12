package nativewindow

import (
	"context"
	"errors"
	"image"
	"testing"
	"time"

	"github.com/herlanai/omnigo/omniwindow"
	"github.com/herlanai/omnigo/runtime/dispatch"
)

type childFake struct {
	fakeBackend
	name    string
	order   *[]string
	created []*childFake
	isChild bool
}

func (b *childFake) child(omniwindow.WindowOptions) (backend, error) {
	c := &childFake{name: b.name + "/child", order: b.order, isChild: true}
	b.created = append(b.created, c)
	return c, nil
}
func (b *childFake) close() { *b.order = append(*b.order, b.name); b.fakeBackend.close() }
func (b *childFake) next(timeout int) ([]Event, error) {
	if b.isChild && len(b.events) == 0 {
		return nil, nil
	}
	return b.fakeBackend.next(timeout)
}
func TestOwnedChildLifetimeAndDispatch(t *testing.T) {
	var order []string
	b := &childFake{name: "root", order: &order}
	var child *Window
	w, _ := New(Options{Ready: func(parent *Window) error {
		var err error
		child, err = parent.NewChild(Options{Paint: func(w, h int) (*image.RGBA, error) { return image.NewRGBA(image.Rect(0, 0, w, h)), nil }, Ready: func(c *Window) error { _, err := c.NewChild(Options{}); return err }})
		return err
	}})
	if err := w.run(context.Background(), func(omniwindow.WindowOptions) (backend, error) { return b, nil }); err != nil {
		t.Fatal(err)
	}
	if len(order) != 3 || order[0] != "root/child/child" || order[1] != "root/child" || order[2] != "root" {
		t.Fatal("incorrect destruction order", order)
	}
	if b.created[0].frames != 1 {
		t.Fatal("child did not present frame")
	}
	if _, err := child.Post(context.Background(), func() error { return nil }); !errors.Is(err, dispatch.ErrClosed) {
		t.Fatal(err)
	}
}
func TestFailedChildReadyReleasesDescendants(t *testing.T) {
	var order []string
	b := &childFake{name: "root", order: &order}
	want := errors.New("ready failed")
	w, _ := New(Options{Ready: func(parent *Window) error {
		_, err := parent.NewChild(Options{Ready: func(child *Window) error {
			if _, err := child.NewChild(Options{}); err != nil {
				return err
			}
			return want
		}})
		if !errors.Is(err, want) {
			t.Fatal(err)
		}
		if len(parent.children) != 0 {
			t.Fatal("failed subtree retained")
		}
		return nil
	}})
	if err := w.run(context.Background(), func(omniwindow.WindowOptions) (backend, error) { return b, nil }); err != nil {
		t.Fatal(err)
	}
	if len(order) != 3 {
		t.Fatal(order)
	}
}
func TestChildClockUsesExplicitDeadline(t *testing.T) {
	now := time.Unix(0, 0)
	root, _ := New(Options{})
	root.now = func() time.Time { return now }
	var order []string
	ticks := 0
	child, _ := New(Options{NextFrame: func() (time.Duration, bool) { return 10 * time.Millisecond, true }, Tick: func() error { ticks++; return nil }})
	child.driver = &childFake{isChild: true, order: &order}
	root.children = []*Window{child}
	if root.childTimeout(-1) != 10 {
		t.Fatal("wrong child wakeup")
	}
	now = now.Add(9 * time.Millisecond)
	if err := root.pumpChildren(); err != nil {
		t.Fatal(err)
	}
	if ticks != 0 {
		t.Fatal("early tick")
	}
	now = now.Add(time.Millisecond)
	if err := root.pumpChildren(); err != nil {
		t.Fatal(err)
	}
	if ticks != 1 {
		t.Fatal("missing tick")
	}
}

func TestChildDeadlinesDoNotAdvanceParentAnimation(t *testing.T) {
	now := time.Unix(0, 0)
	var order []string
	b := &childFake{name: "root", order: &order}
	b.events = [][]Event{nil, {{Kind: "move"}}, nil, {{Kind: "close"}}}
	b.beforeNext = func(wait int) {
		if wait > 0 {
			now = now.Add(time.Duration(wait) * time.Millisecond)
		}
	}
	parentTicks, childTicks := 0, 0
	w, _ := New(Options{
		NextFrame: func() (time.Duration, bool) { return 30 * time.Millisecond, true },
		Tick:      func() error { parentTicks++; return nil },
		Ready: func(parent *Window) error {
			_, err := parent.NewChild(Options{
				NextFrame: func() (time.Duration, bool) { return 10 * time.Millisecond, true },
				Tick:      func() error { childTicks++; return nil },
			})
			return err
		},
	})
	w.now = func() time.Time { return now }
	if err := w.run(context.Background(), func(omniwindow.WindowOptions) (backend, error) { return b, nil }); err != nil {
		t.Fatal(err)
	}
	if parentTicks != 1 || childTicks != 3 {
		t.Fatalf("parent=%d child=%d waits=%v", parentTicks, childTicks, b.timeouts)
	}
	for _, wait := range b.timeouts {
		if wait != 10 {
			t.Fatalf("deadline postponed by events: %v", b.timeouts)
		}
	}
}

func TestRootAndChildRejectInvalidFramesAndReleaseResources(t *testing.T) {
	for _, owned := range []bool{false, true} {
		t.Run(map[bool]string{false: "root", true: "child"}[owned], func(t *testing.T) {
			var order []string
			b := &childFake{name: "root", order: &order}
			paint := func(int, int) (*image.RGBA, error) { return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil }
			options := Options{Paint: paint}
			if owned {
				options = Options{Ready: func(parent *Window) error { _, err := parent.NewChild(Options{Paint: paint}); return err }}
			}
			w, err := New(options)
			if err != nil {
				t.Fatal(err)
			}
			if err = w.run(t.Context(), func(omniwindow.WindowOptions) (backend, error) { return b, nil }); err == nil {
				t.Fatal("invalid framebuffer accepted")
			}
			if b.closed != 1 || b.frames != 0 {
				t.Fatal("root resource or presentation", b)
			}
			if owned && (len(b.created) != 1 || b.created[0].closed != 1 || b.created[0].frames != 0) {
				t.Fatal("child resource or presentation")
			}
		})
	}
}
