package dshgo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStreamToolFragmentsAndUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing key")
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"function\":{\"name\":\"search\",\"arguments\":\"{}\"}}]}}]}\n\ndata: {\"usage\":{\"total_tokens\":12}}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	c, e := New(Options{BaseURL: srv.URL, APIKey: "test-key"})
	if e != nil {
		t.Fatal(e)
	}
	var deltas []Delta
	u, e := c.Chat(context.Background(), Request{Model: "test", Messages: []Message{{Role: "user", Content: "hello"}}}, func(d Delta) error { deltas = append(deltas, d); return nil })
	if e != nil || u.TotalTokens != 12 || len(deltas) != 2 || deltas[1].ToolCalls[0].Function.Name != "search" {
		t.Fatal(deltas, u, e)
	}
}
