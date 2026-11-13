package omniweb

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"
)

func TestTypeScriptEventsAgainstGoHTTP(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js 24+ required for SDK integration")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	events := NewEvents()
	defer events.Close()
	if err := events.Register("progress", "read"); err != nil {
		t.Fatal(err)
	}
	if err := events.Register("private", "secret"); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(4, 4096)
	if err := Bind(router, "emit", "read", func(context.Context, struct{}) (int, error) {
		for _, value := range []string{"中文😀", "line\nbreak", "last"} {
			if _, err := events.Publish("progress", value); err != nil {
				return 0, err
			}
		}
		return 3, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := Bind(router, "stop", "read", func(context.Context, struct{}) (bool, error) { events.Close(); return true, nil }); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(nil)
	origin := "http://" + server.Listener.Addr().String()
	handler, err := NewHTTP(router, HTTPOptions{Events: events, Host: server.Listener.Addr().String(), Origins: []string{origin}, Grants: []string{"read"}})
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	server.Config.Handler = handler
	server.Config.BaseContext = func(net.Listener) context.Context { return ctx }
	server.Start()
	defer server.Close()
	command := exec.CommandContext(ctx, node, "testdata/events_e2e.ts", origin)
	output, err := command.CombinedOutput()
	if err != nil {
		cancel()
		t.Fatalf("SDK process: %v\n%s", err, output)
	}
	t.Log(string(output))
	server.Close()
	events.mu.Lock()
	remaining := len(events.subs)
	events.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("%d subscriptions survived child process", remaining)
	}
}

func TestEventRequestCancellationUnsubscribes(t *testing.T) {
	events := NewEvents()
	defer events.Close()
	events.Register("progress", "")
	h, _ := NewHTTP(NewRouter(1, 1024), HTTPOptions{Events: events, Host: "test", Origins: []string{"http://test"}})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	r := httptest.NewRequest(http.MethodPost, "http://test"+Prefix+"events", nil).WithContext(ctx)
	// Enter the transport after the common HTTP authorization/body checks.
	w := &readyEventWriter{ResponseRecorder: httptest.NewRecorder(), ready: make(chan struct{})}
	done := make(chan struct{})
	go func() { defer close(done); h.events(w, r, []byte(`{"v":1,"topics":["progress"]}`)) }()
	<-w.ready
	cancel()
	<-done
	events.mu.Lock()
	remaining := len(events.subs)
	events.mu.Unlock()
	if remaining != 0 {
		t.Fatal("canceled subscriber retained")
	}
}

type readyEventWriter struct {
	*httptest.ResponseRecorder
	ready chan struct{}
}

func (w *readyEventWriter) Flush() {
	w.ResponseRecorder.Flush()
	select {
	case <-w.ready:
	default:
		close(w.ready)
	}
}
