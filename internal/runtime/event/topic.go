// Package event provides typed, bounded event delivery without callbacks under locks.
package event

import "sync"

type Delivery struct{ Delivered, Dropped int }

// Topic uses per-subscriber bounded queues. Publish never waits for readers;
// a full queue drops the newest message and reports it to the producer.
// Values containing pointers must not be mutated after publication.
type Topic[T any] struct {
	mu     sync.Mutex
	next   uint64
	closed bool
	subs   map[uint64]chan T
}

func (t *Topic[T]) Subscribe(capacity int) (<-chan T, func()) {
	if capacity < 1 {
		capacity = 1
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	ch := make(chan T, capacity)
	if t.closed {
		close(ch)
		return ch, func() {}
	}
	if t.subs == nil {
		t.subs = make(map[uint64]chan T)
	}
	t.next++
	id := t.next
	t.subs[id] = ch
	return ch, func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		if current, ok := t.subs[id]; ok {
			delete(t.subs, id)
			close(current)
		}
	}
}

func (t *Topic[T]) Publish(value T) Delivery {
	t.mu.Lock()
	defer t.mu.Unlock()
	var result Delivery
	for _, ch := range t.subs {
		select {
		case ch <- value:
			result.Delivered++
		default:
			result.Dropped++
		}
	}
	return result
}

func (t *Topic[T]) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	t.closed = true
	for id, ch := range t.subs {
		close(ch)
		delete(t.subs, id)
	}
}
