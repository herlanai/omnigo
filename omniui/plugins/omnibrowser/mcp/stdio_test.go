package mcp

import (
	"context"
	"encoding/json"
	"io"
	browser "github.com/herlanai/omnigo/omniui/plugins/omnibrowser"
	"testing"
	"time"
)

func TestStdioLifecycleToolsAndCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	in, writer := io.Pipe()
	out, reader := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, in, reader, browser.Options{}); reader.Close() }()
	defer out.Close()
	defer writer.Close()
	enc, dec := json.NewEncoder(writer), json.NewDecoder(out)
	send := func(id any, method string, params any) {
		t.Helper()
		m := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
		if id != nil {
			m["id"] = id
		}
		if err := enc.Encode(m); err != nil {
			t.Fatal(err)
		}
	}
	receive := func() map[string]any {
		t.Helper()
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	send(1, "tools/list", nil)
	if receive()["error"] == nil {
		t.Fatal("accepted before initialize")
	}
	send(2, "initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "test", "version": "1"}})
	if receive()["result"] == nil {
		t.Fatal("initialize failed")
	}
	send(nil, "notifications/initialized", nil)
	send(3, "tools/list", nil)
	list := receive()["result"].(map[string]any)["tools"].([]any)
	if len(list) != 8 {
		t.Fatal(len(list))
	}
	for _, entry := range list {
		if _, ok := entry.(map[string]any)["inputSchema"].(map[string]any)["required"].([]any); !ok {
			t.Fatal("required must be array")
		}
	}
	send(4, "tools/call", map[string]any{"name": "set_content", "arguments": map[string]string{"html": "<p id='x'>Ready</p>"}})
	if receive()["result"].(map[string]any)["isError"] == true {
		t.Fatal("content failed")
	}
	send(5, "tools/call", map[string]any{"name": "evaluate", "arguments": map[string]string{"expression": "document.querySelector('#x').textContent"}})
	value := receive()["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"]
	if value != `"Ready"` {
		t.Fatal(value)
	}
	send(6, "tools/call", map[string]any{"name": "evaluate", "arguments": map[string]string{"expression": "new Promise(()=>{})"}})
	send(nil, "notifications/cancelled", map[string]any{"requestId": 6})
	if receive()["result"].(map[string]any)["isError"] != true {
		t.Fatal("cancel not reported")
	}
	send(7, "tools/call", map[string]any{"name": "query", "arguments": map[string]any{"selector": 42}})
	if receive()["result"].(map[string]any)["isError"] != true {
		t.Fatal("invalid arguments accepted")
	}
	writer.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
