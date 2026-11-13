package host

import (
	"context"
	"errors"
	"github.com/herlanai/omnigo/omniwindow"
	"slices"
	"sync"
)

// SystemSubscription is owned by both its context and the application. Close
// is nonblocking, idempotent and safe in UI callbacks. Events closes on cleanup.
// The 16-event buffer drops the oldest notification when full.
type SystemSubscription struct {
	Events  <-chan omniwindow.SystemEvent
	events  chan omniwindow.SystemEvent
	done    chan struct{}
	host    *Controller
	once    sync.Once
	dropped uint64 // UI-owned
}

func (s *SystemSubscription) Close() {
	s.once.Do(func() { s.host.control(func() { s.host.removeSubscription(s) }) })
}
func (h *Controller) removeSubscription(s *SystemSubscription) {
	if _, ok := h.subscriptions[s]; !ok {
		return
	}
	delete(h.subscriptions, s)
	close(s.events)
	close(s.done)
	if len(h.subscriptions) == 0 && h.stopSystemWatch != nil {
		h.stopSystemWatch()
		h.stopSystemWatch = nil
	}
}
func (h *Controller) WatchSystemEvents(ctx context.Context) (*SystemSubscription, error) {
	if ctx == nil {
		return nil, errors.New("host: nil context")
	}
	s := &SystemSubscription{host: h, events: make(chan omniwindow.SystemEvent, 16), done: make(chan struct{})}
	s.Events = s.events
	err := h.createUI(ctx, func() error {
		if h.watchSystem == nil {
			return ErrUnsupported
		}
		if len(h.subscriptions) == 0 {
			stop, err := h.watchSystem(h.queueSystemEvent)
			if err != nil {
				return err
			}
			h.stopSystemWatch = stop
		}
		if h.subscriptions == nil {
			h.subscriptions = make(map[*SystemSubscription]struct{})
		}
		h.subscriptions[s] = struct{}{}
		return nil
	}, func() { h.removeSubscription(s) })
	if err != nil {
		return nil, err
	}
	go func() {
		select {
		case <-ctx.Done():
			s.Close()
		case <-s.done:
		}
	}()
	return s, nil
}
func (h *Controller) queueSystemEvent(kind omniwindow.SystemEventKind) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	state := kind == omniwindow.SystemDisplaysChanged || kind == omniwindow.SystemAppearanceChanged
	if state {
		for _, pending := range h.systemSignals {
			if pending == kind {
				return
			}
		}
	}
	if len(h.systemSignals) == 64 {
		h.systemSignals = h.systemSignals[1:]
		h.systemMissed++
	}
	h.systemSignals = append(h.systemSignals, kind)
	if h.systemScheduled {
		return
	}
	h.systemScheduled = true
	h.controls = append(h.controls, func() {
		h.mu.Lock()
		signals, missed := h.systemSignals, h.systemMissed
		h.systemSignals = nil
		h.systemMissed = 0
		h.systemScheduled = false
		h.mu.Unlock()
		for sub := range h.subscriptions {
			sub.dropped += missed
		}
		for _, kind := range signals {
			h.publishSystemEvent(kind)
		}
	})
	if h.wake != nil {
		h.wake()
	}
}
func (h *Controller) publishSystemEvent(kind omniwindow.SystemEventKind) {
	if len(h.subscriptions) == 0 {
		return
	}
	h.systemSequence++
	event := omniwindow.SystemEvent{Kind: kind, Sequence: h.systemSequence}
	switch kind {
	case omniwindow.SystemDisplaysChanged:
		if h.screen == nil {
			event.Err = ErrUnsupported
		} else {
			state, err := h.screen()
			event.Screen = &state
			event.Err = err
		}
	case omniwindow.SystemAppearanceChanged:
		if h.appearance == nil {
			event.Err = ErrUnsupported
		} else {
			state, err := h.appearance("")
			event.Appearance = &state
			event.Err = err
		}
	}
	for sub := range h.subscriptions {
		copy := event
		if event.Screen != nil {
			state := *event.Screen
			state.Displays = slices.Clone(state.Displays)
			copy.Screen = &state
		}
		if event.Appearance != nil {
			state := *event.Appearance
			copy.Appearance = &state
		}
		// The UI is the sole producer/closer. A consumer may drain between checks.
		if len(sub.events) == cap(sub.events) {
			select {
			case <-sub.events:
				sub.dropped++
			default:
			}
		}
		copy.Dropped = sub.dropped
		sub.events <- copy
	}
}
