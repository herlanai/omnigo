package watch

import (
	"context"
	"errors"
	"testing"
)

func TestCanceledResolverReleasesSourceButRetainsGlobalSlot(t *testing.T) {
	entered, leave := make(chan struct{}), make(chan struct{})
	dir := t.TempDir()
	s, err := NewService(ServiceOptions{MaxWatches: 2, MaxWatchesPerSource: 1,
		Resolve: func(ctx context.Context, name string) (Root, bool) {
			if name == "slow" {
				close(entered)
				<-leave
			}
			return Root{Path: dir}, true
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	slowDone := make(chan error, 1)
	go func() { slowDone <- s.observe(ctx, Request{Root: "slow"}, nil) }()
	<-entered
	cancel()
	// The resolver deliberately hasn't exited. A new call from the same source
	// must be admitted without waiting for the cancellation hook's scheduling.
	nextEntered, nextLeave := make(chan struct{}), make(chan struct{})
	nextDone := make(chan error, 1)
	stop := errors.New("complete")
	go func() {
		nextDone <- s.observe(t.Context(), Request{Root: "next"}, func(context.Context, Batch) error {
			close(nextEntered)
			<-nextLeave
			return stop
		})
	}()
	select {
	case <-nextEntered:
	case err := <-nextDone:
		close(leave)
		<-slowDone
		t.Fatalf("replacement was not admitted: %v", err)
	}
	s.mu.Lock()
	active, count := s.active, s.bySource[nil]
	s.mu.Unlock()
	if active != 2 || count != 1 {
		t.Fatal(active, count)
	}
	// The global limit still prevents canceled but uncooperative work accumulating.
	if err = s.observe(t.Context(), Request{Root: "third"}, nil); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	close(leave)
	if err = <-slowDone; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	s.mu.Lock()
	active, count = s.active, s.bySource[nil]
	s.mu.Unlock()
	if active != 1 || count != 1 {
		t.Fatal("late cleanup stole replacement quota", active, count)
	}
	close(nextLeave)
	if err = <-nextDone; !errors.Is(err, stop) {
		t.Fatal(err)
	}
	s.mu.Lock()
	active, count, reserved := s.active, len(s.bySource), len(s.reservations)
	s.mu.Unlock()
	if active != 0 || count != 0 || reserved != 0 {
		t.Fatal("quota retained", active, count, reserved)
	}
}
