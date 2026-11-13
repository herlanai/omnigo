package omniweb

import (
	"context"
	"net"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestTypeScriptStreamsAgainstGoHTTP(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js 24+ required")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	s := NewStreams(2)
	handler := func(ctx context.Context, n int, ch *Channel[string]) (int, error) {
		for i := 0; i < n; i++ {
			if err := ch.Send(strings.Repeat("中文😀", 8192)); err != nil {
				return i, err
			}
		}
		return n, nil
	}
	if err := BindStream(s, "count", "read", handler); err != nil {
		t.Fatal(err)
	}
	if err := BindStream(s, "private", "secret", handler); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(nil)
	origin := "http://" + server.Listener.Addr().String()
	h, err := NewHTTP(NewRouter(1, 1024), HTTPOptions{Streams: s, Host: server.Listener.Addr().String(), Origins: []string{origin}, Grants: []string{"read"}})
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	server.Config.Handler = h
	server.Config.BaseContext = func(net.Listener) context.Context { return ctx }
	server.Start()
	defer server.Close()
	out, err := exec.CommandContext(ctx, node, "testdata/streams_e2e.ts", origin).CombinedOutput()
	if err != nil {
		cancel()
		t.Fatalf("%v\n%s", err, out)
	}
	t.Log(string(out))
	server.Close()
	s.mu.RLock()
	remaining := len(s.sessions)
	s.mu.RUnlock()
	if remaining != 0 || len(s.slots) != 0 {
		t.Fatal("stream session/slot leaked", remaining, len(s.slots))
	}
}
