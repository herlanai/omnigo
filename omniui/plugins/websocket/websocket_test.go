package websocket

import (
	"context"
	ws "github.com/coder/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoopbackFramesAndCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := ws.Accept(w, r, &ws.AcceptOptions{Subprotocols: []string{"v1"}})
		if e != nil {
			return
		}
		defer c.CloseNow()
		for {
			kind, b, e := c.Read(r.Context())
			if e != nil {
				return
			}
			if e = c.Write(r.Context(), kind, b); e != nil {
				return
			}
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, e := Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), Options{Protocols: []string{"v1"}, MaxMessageBytes: 8})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Abort()
	if c.Protocol() != "v1" {
		t.Fatal("protocol missing")
	}
	for _, kind := range []MessageType{Text, Binary} {
		if e = c.Send(ctx, kind, []byte("hi")); e != nil {
			t.Fatal(e)
		}
		got, b, e := c.Receive(ctx)
		if e != nil || got != kind || string(b) != "hi" {
			t.Fatal(got, string(b), e)
		}
	}
	if e = c.Send(ctx, Text, make([]byte, 9)); e == nil {
		t.Fatal("limit ignored")
	}
	cancel()
	if _, _, e = c.Receive(ctx); e == nil {
		t.Fatal("cancel ignored")
	}
}
