package host

import (
	"context"
	"github.com/herlanai/omnigo/internal/runtime/dispatch"
	"github.com/herlanai/omnigo/omniwindow"
	"testing"
)

func TestSystemSubscriptionsLifetimeAndOverflow(t *testing.T) {
	q := dispatch.New(8)
	h := &Controller{queue: q, wake: func() {}}
	starts, stops := 0, 0
	h.watchSystem = func(func(omniwindow.SystemEventKind)) (func(), error) { starts++; return func() { stops++ }, nil }
	h.screen = func() (omniwindow.ScreenState, error) {
		return omniwindow.ScreenState{Displays: []omniwindow.Display{{ID: 7}}}, nil
	}
	subscribe := func() *SystemSubscription {
		out := make(chan *SystemSubscription, 1)
		go func() {
			s, err := h.WatchSystemEvents(t.Context())
			if err != nil {
				t.Error(err)
			}
			out <- s
		}()
		<-q.Wake()
		h.drain()
		return <-out
	}
	a, b := subscribe(), subscribe()
	if starts != 1 {
		t.Fatal(starts)
	}
	h.queueSystemEvent(omniwindow.SystemDisplaysChanged)
	h.queueSystemEvent(omniwindow.SystemDisplaysChanged)
	h.drain()
	first, second := <-a.Events, <-b.Events
	first.Screen.Displays[0].ID = 9
	if second.Screen.Displays[0].ID != 7 || len(a.events) != 0 {
		t.Fatal("snapshot alias or failed coalescing")
	}
	for range 30 {
		h.publishSystemEvent(omniwindow.SystemDidWake)
	}
	event := <-a.Events
	if event.Sequence <= 2 {
		t.Fatal("oldest event was not discarded")
	}
	var last omniwindow.SystemEvent
	for len(a.events) > 0 {
		last = <-a.Events
	}
	if last.Dropped != 14 {
		t.Fatal(last.Dropped)
	}
	a.Close()
	a.Close()
	h.drain()
	if _, ok := <-a.Events; ok {
		t.Fatal("subscription not closed")
	}
	if stops != 0 {
		t.Fatal("observer removed with remaining subscriber")
	}
	h.close()
	for range b.Events {
	}
	if stops != 1 {
		t.Fatal(stops)
	}
	b.Close()
}
func TestSystemSignalBoundsAndOrder(t *testing.T) {
	h := &Controller{queue: dispatch.New(1), wake: func() {}}
	defer h.close()
	s := &SystemSubscription{host: h, events: make(chan omniwindow.SystemEvent, 128), done: make(chan struct{})}
	h.subscriptions = map[*SystemSubscription]struct{}{s: {}}
	for i := range 100 {
		kind := omniwindow.SystemWillSleep
		if i%2 != 0 {
			kind = omniwindow.SystemDidWake
		}
		h.queueSystemEvent(kind)
	}
	if len(h.controls) != 1 || len(h.systemSignals) != 64 {
		t.Fatal("unbounded native notification queue")
	}
	h.drain()
	for i := range 64 {
		event := <-s.events
		want := omniwindow.SystemWillSleep
		if i%2 != 0 {
			want = omniwindow.SystemDidWake
		}
		if event.Kind != want || event.Dropped != 36 {
			t.Fatal(event)
		}
	}
}
func TestSystemSubscriptionCanceledBeforeDispatch(t *testing.T) {
	h := &Controller{queue: dispatch.New(1), wake: func() {}}
	defer h.close()
	h.watchSystem = func(func(omniwindow.SystemEventKind)) (func(), error) {
		t.Error("started canceled watcher")
		return func() {}, nil
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := h.WatchSystemEvents(ctx); err == nil {
		t.Fatal("canceled subscription accepted")
	}
	h.drain()
}

func TestCanceledSubscriptionCreationReleasesObserver(t *testing.T) {
	q := dispatch.New(1)
	entered, release := make(chan struct{}), make(chan struct{})
	stops := 0
	h := &Controller{queue: q, wake: func() {}, watchSystem: func(func(omniwindow.SystemEventKind)) (func(), error) {
		close(entered)
		<-release
		return func() { stops++ }, nil
	}}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := h.WatchSystemEvents(ctx); result <- err }()
	<-q.Wake()
	drained := make(chan struct{})
	go func() { h.drain(); close(drained) }()
	<-entered
	cancel()
	if err := <-result; err != context.Canceled {
		t.Fatal(err)
	}
	close(release)
	<-drained
	h.drain()
	h.close()
	if stops != 1 || len(h.subscriptions) != 0 {
		t.Fatal("late observer leaked", stops)
	}
}
