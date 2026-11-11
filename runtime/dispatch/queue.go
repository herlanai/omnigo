// Package dispatch provides a bounded, explicitly drained main-thread work queue.
package dispatch

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrFull = errors.New("dispatch: queue full")
var ErrClosed = errors.New("dispatch: queue closed")

type task struct {
	ctx    context.Context
	fn     func() error
	result chan error
}

type Queue struct {
	mu         sync.Mutex
	items      []task
	head, size int
	closed     bool
	wake       chan struct{}
}

func New(capacity int) *Queue {
	if capacity < 1 {
		capacity = 256
	}
	return &Queue{items: make([]task, capacity), wake: make(chan struct{}, 1)}
}

// Wake is a coalesced notification, never a task count. A single platform main
// thread consumes it and calls Drain. No timer or polling goroutine is created.
func (q *Queue) Wake() <-chan struct{} { return q.wake }
func (q *Queue) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// Submit never blocks. The result channel receives exactly one error and closes.
// Waiting for it on the main thread would deadlock; use it from a worker only.
func (q *Queue) Submit(ctx context.Context, fn func() error) (<-chan error, error) {
	if ctx == nil || fn == nil {
		return nil, errors.New("dispatch: context and function required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil, ErrClosed
	}
	if q.size == len(q.items) {
		return nil, ErrFull
	}
	result := make(chan error, 1)
	q.items[(q.head+q.size)%len(q.items)] = task{ctx, fn, result}
	q.size++
	q.signal()
	return result, nil
}

func invoke(t task) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("dispatch: task panic: %v", p)
		}
	}()
	if err := t.ctx.Err(); err != nil {
		return err
	}
	return t.fn()
}

// Drain must be called by one consumer. Limit bounds work per native event-loop
// turn, even if callbacks submit additional work. Callbacks run without q.mu.
func (q *Queue) Drain(limit int) int {
	if limit <= 0 {
		return 0
	}
	select {
	case <-q.wake:
	default:
	}
	done := 0
	for done < limit {
		q.mu.Lock()
		if q.size == 0 {
			q.mu.Unlock()
			break
		}
		t := q.items[q.head]
		q.items[q.head] = task{}
		q.head = (q.head + 1) % len(q.items)
		q.size--
		q.mu.Unlock()
		t.result <- invoke(t)
		close(t.result)
		done++
	}
	q.mu.Lock()
	if q.size > 0 {
		q.signal()
	}
	q.mu.Unlock()
	return done
}

// Close rejects queued tasks. A task already executing finishes normally.
func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	for q.size > 0 {
		t := q.items[q.head]
		q.items[q.head] = task{}
		q.head = (q.head + 1) % len(q.items)
		q.size--
		t.result <- ErrClosed
		close(t.result)
	}
}
