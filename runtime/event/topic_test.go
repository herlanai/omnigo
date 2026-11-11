package event

import (
	"sync"
	"testing"
)

func TestBackpressureAndClose(t *testing.T) {
	var topic Topic[int]
	ch, cancel := topic.Subscribe(1)
	if got := topic.Publish(1); got != (Delivery{Delivered: 1}) {
		t.Fatal(got)
	}
	if got := topic.Publish(2); got != (Delivery{Dropped: 1}) {
		t.Fatal(got)
	}
	if <-ch != 1 {
		t.Fatal("lost first event")
	}
	cancel()
	cancel()
	topic.Close()
	topic.Close()
	if _, ok := <-ch; ok {
		t.Fatal("subscription still open")
	}
	closed, _ := topic.Subscribe(1)
	if _, ok := <-closed; ok {
		t.Fatal("subscribe after close")
	}
}

func TestConcurrentLifecycle(t *testing.T) {
	var topic Topic[int]
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 100; j++ {
				_, cancel := topic.Subscribe(1)
				topic.Publish(j)
				cancel()
			}
		}()
	}
	workers.Add(1)
	go func() { defer workers.Done(); topic.Close() }()
	workers.Wait()
}

func BenchmarkPublish(b *testing.B) {
	var topic Topic[int]
	for i := 0; i < 16; i++ {
		topic.Subscribe(1)
	}
	b.ReportAllocs()
	for b.Loop() {
		topic.Publish(1)
	}
}
