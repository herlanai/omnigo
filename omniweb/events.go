package omniweb

import (
	"encoding/json"
	"errors"
	"sync"
)

// Events is a bounded, concurrent event broker. Register topics before exposing
// the HTTP endpoint. Payloads are serialized at publication, never retained by reference.
type Events struct {
	mu     sync.Mutex
	topics map[string]string
	subs   map[*eventSubscription]bool
	closed bool
}
type eventSubscription struct {
	topics map[string]bool
	queue  chan eventFrame
	end    chan string
}
type eventFrame struct {
	Version int             `json:"v"`
	Kind    string          `json:"kind"`
	Topic   string          `json:"topic,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	Code    string          `json:"code,omitempty"`
}
type EventDelivery struct{ Delivered, Disconnected int }

func NewEvents() *Events {
	return &Events{topics: map[string]string{}, subs: map[*eventSubscription]bool{}}
}
func (e *Events) Register(name, permission string) error {
	if !methodName.MatchString(name) {
		return ErrInvalidRequest
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return errors.New("omniweb: events closed")
	}
	if _, ok := e.topics[name]; ok {
		return errors.New("omniweb: duplicate event")
	}
	if e.topics == nil {
		e.topics = map[string]string{}
	}
	e.topics[name] = permission
	return nil
}
func (e *Events) Publish(name string, value any) (EventDelivery, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return EventDelivery{}, err
	}
	if len(raw) > 256<<10 {
		return EventDelivery{}, ErrTooLarge
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return EventDelivery{}, errors.New("omniweb: events closed")
	}
	if _, ok := e.topics[name]; !ok {
		return EventDelivery{}, ErrNotFound
	}
	result := EventDelivery{}
	for sub := range e.subs {
		if sub.topics[name] {
			select {
			case sub.queue <- eventFrame{Version: Version, Kind: "event", Topic: name, Data: raw}:
				result.Delivered++
			default:
				sub.end <- "overflow"
				delete(e.subs, sub)
				result.Disconnected++
			}
		}
	}
	return result, nil
}
func (e *Events) subscribe(names, grants []string) (*eventSubscription, func(), error) {
	if len(names) == 0 || len(names) > 32 {
		return nil, nil, ErrInvalidRequest
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, nil, errors.New("omniweb: events closed")
	}
	if len(e.subs) >= 128 {
		return nil, nil, ErrBusy
	}
	allowed := map[string]bool{}
	for _, g := range grants {
		allowed[g] = true
	}
	topics := map[string]bool{}
	for _, name := range names {
		permission, exists := e.topics[name]
		if !exists {
			return nil, nil, ErrNotFound
		}
		if permission != "" && !allowed[permission] {
			return nil, nil, ErrForbidden
		}
		if topics[name] {
			return nil, nil, ErrInvalidRequest
		}
		topics[name] = true
	}
	sub := &eventSubscription{topics: topics, queue: make(chan eventFrame, 64), end: make(chan string, 1)}
	if e.subs == nil {
		e.subs = map[*eventSubscription]bool{}
	}
	e.subs[sub] = true
	return sub, func() { e.mu.Lock(); delete(e.subs, sub); e.mu.Unlock() }, nil
}
func (e *Events) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	e.closed = true
	for s := range e.subs {
		s.end <- "closed"
		delete(e.subs, s)
	}
}
