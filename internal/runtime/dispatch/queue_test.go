package dispatch

import (
	"context"
	"errors"
	"testing"
)

func TestQueueBoundariesAndReentrancy(t *testing.T) {
	q := New(1)
	var second <-chan error
	first, err := q.Submit(context.Background(), func() error {
		var err error
		second, err = q.Submit(context.Background(), func() error { return nil })
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.Submit(context.Background(), func() error { return nil }); !errors.Is(err, ErrFull) {
		t.Fatal(err)
	}
	<-q.Wake()
	if q.Drain(1) != 1 {
		t.Fatal("wrong work count")
	}
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	<-q.Wake()
	q.Close()
	if err := <-second; !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	if _, err := q.Submit(context.Background(), func() error { return nil }); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}

func TestCancelAndPanic(t *testing.T) {
	q := New(2)
	ctx, cancel := context.WithCancel(context.Background())
	a, err := q.Submit(ctx, func() error { t.Fatal("canceled work ran"); return nil })
	if err != nil {
		t.Fatal(err)
	}
	b, err := q.Submit(context.Background(), func() error { panic("bad") })
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	q.Drain(2)
	if err := <-a; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := <-b; err == nil {
		t.Fatal("panic was lost")
	}
}
