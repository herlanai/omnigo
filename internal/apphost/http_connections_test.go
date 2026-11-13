package apphost

import (
	"net"
	"net/http"
	"sync"
	"testing"
)

type trackedConn struct {
	net.Conn
	mu     sync.Mutex
	closed bool
}

func (c *trackedConn) Close() error   { c.mu.Lock(); defer c.mu.Unlock(); c.closed = true; return nil }
func (c *trackedConn) isClosed() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.closed }

func TestPendingConnectionsShutdown(t *testing.T) {
	var pending pendingConnections
	unused, active, idle, hijacked := &trackedConn{}, &trackedConn{}, &trackedConn{}, &trackedConn{}
	for _, c := range []*trackedConn{unused, active, idle, hijacked} {
		pending.state(c, http.StateNew)
	}
	pending.state(active, http.StateActive)
	pending.state(idle, http.StateIdle)
	pending.state(hijacked, http.StateHijacked)
	pending.stop()
	if !unused.isClosed() {
		t.Fatal("preconnection leaked")
	}
	for _, c := range []*trackedConn{active, idle, hijacked} {
		if c.isClosed() {
			t.Fatal("shutdown closed a used connection")
		}
	}
	late := &trackedConn{}
	pending.state(late, http.StateNew)
	if !late.isClosed() {
		t.Fatal("connection accepted during shutdown leaked")
	}
	pending.state(unused, http.StateClosed)
	pending.stop()
}

func TestPendingConnectionsConcurrentShutdown(t *testing.T) {
	var pending pendingConnections
	var wg sync.WaitGroup
	connections := make([]*trackedConn, 128)
	for i := range connections {
		c := &trackedConn{}
		connections[i] = c
		wg.Go(func() { pending.state(c, http.StateNew) })
	}
	wg.Go(pending.stop)
	wg.Wait()
	for _, c := range connections {
		if !c.isClosed() {
			t.Fatal("racing preconnection leaked")
		}
	}
}
