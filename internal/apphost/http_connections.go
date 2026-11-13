package apphost

import (
	"net"
	"net/http"
	"sync"
)

// Browsers may preconnect without sending an HTTP request. net/http gives new
// connections five seconds before treating them as idle, longer than our three
// second shutdown budget. Close only unused connections when the native window
// ends; active handlers still get ordinary graceful shutdown.
type pendingConnections struct {
	mu          sync.Mutex
	connections map[net.Conn]struct{}
	stopping    bool
}

func (p *pendingConnections) state(conn net.Conn, state http.ConnState) {
	p.mu.Lock()
	closeNow := false
	if state == http.StateNew {
		if p.stopping {
			closeNow = true
		} else {
			if p.connections == nil {
				p.connections = make(map[net.Conn]struct{})
			}
			p.connections[conn] = struct{}{}
		}
	} else {
		delete(p.connections, conn)
	}
	p.mu.Unlock()
	if closeNow {
		_ = conn.Close()
	}
}

func (p *pendingConnections) stop() {
	p.mu.Lock()
	p.stopping = true
	connections := p.connections
	p.connections = nil
	p.mu.Unlock()
	for conn := range connections {
		_ = conn.Close()
	}
}
