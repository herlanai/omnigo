// Package mcp provides the MCP 2025-06-18 stdio tool transport.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	browser "github.com/herlanai/omnigo/omniui/plugins/omnibrowser"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser/render"
	"sync"
)

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// Serve owns input and its isolated page. EOF/cancellation closes both. Stdout
// contains JSON-RPC only. Outstanding tools are bounded and cancellable.
func Serve(ctx context.Context, input io.ReadCloser, output io.Writer, options browser.Options) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer input.Close()
	page := browser.New(options)
	defer page.Close()
	_ = page.SetContent(ctx, "<title>OmniBrowser</title>")
	stop := context.AfterFunc(ctx, func() { _ = input.Close() })
	defer stop()
	var writeMu, mu sync.Mutex
	var wg sync.WaitGroup
	active := map[string]context.CancelFunc{}
	slots := make(chan struct{}, 8)
	send := func(id json.RawMessage, result any, code int, err string) {
		writeMu.Lock()
		defer writeMu.Unlock()
		r := map[string]any{"jsonrpc": "2.0", "id": id}
		if code != 0 {
			r["error"] = map[string]any{"code": code, "message": err}
		} else {
			r["result"] = result
		}
		_ = json.NewEncoder(output).Encode(r)
	}
	defer func() { cancel(); _ = page.Close(); wg.Wait() }()
	initialized, ready := false, false
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 2<<20)
	for scanner.Scan() {
		var req message
		raw := append([]byte(nil), scanner.Bytes()...)
		if err := json.Unmarshal(raw, &req); err != nil {
			send(json.RawMessage("null"), nil, -32700, "parse error")
			continue
		}
		if req.JSONRPC != "2.0" || req.Method == "" {
			if len(req.ID) > 0 {
				send(req.ID, nil, -32600, "invalid request")
			}
			continue
		}
		if len(req.ID) == 0 {
			switch req.Method {
			case "notifications/initialized":
				if initialized {
					ready = true
				}
			case "notifications/cancelled":
				var v struct {
					RequestID json.RawMessage `json:"requestId"`
				}
				if json.Unmarshal(req.Params, &v) == nil {
					mu.Lock()
					if c := active[string(v.RequestID)]; c != nil {
						c()
					}
					mu.Unlock()
				}
			}
			continue
		}
		switch req.Method {
		case "initialize":
			if initialized {
				send(req.ID, nil, -32600, "already initialized")
				continue
			}
			initialized = true
			send(req.ID, map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{"listChanged": false}}, "serverInfo": map[string]string{"name": "OmniBrowser", "version": "0.2.0"}}, 0, "")
		case "ping":
			send(req.ID, map[string]any{}, 0, "")
		case "tools/list":
			if !ready {
				send(req.ID, nil, -32002, "initialize first")
				continue
			}
			send(req.ID, map[string]any{"tools": tools()}, 0, "")
		case "tools/call":
			if !ready {
				send(req.ID, nil, -32002, "initialize first")
				continue
			}
			mu.Lock()
			_, duplicate := active[string(req.ID)]
			mu.Unlock()
			if duplicate {
				send(req.ID, nil, -32600, "duplicate active request ID")
				continue
			}
			select {
			case slots <- struct{}{}:
			default:
				send(req.ID, nil, -32000, "too many active tools")
				continue
			}
			callCtx, stop := context.WithCancel(ctx)
			mu.Lock()
			active[string(req.ID)] = stop
			mu.Unlock()
			wg.Add(1)
			go func(req message) {
				defer wg.Done()
				defer func() { stop(); <-slots; mu.Lock(); delete(active, string(req.ID)); mu.Unlock() }()
				result, err := call(callCtx, page, req.Params)
				if err != nil {
					result = map[string]any{"isError": true, "content": []any{map[string]string{"type": "text", "text": err.Error()}}}
				}
				send(req.ID, result, 0, "")
			}(req)
		default:
			send(req.ID, nil, -32601, "method not found")
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return scanner.Err()
}
func tools() []any {
	var list []any
	for _, entry := range []struct {
		name, description string
		required          []string
	}{{"navigate", "Load an HTTP(S) page", []string{"url"}}, {"set_content", "Load inline HTML", []string{"html"}}, {"evaluate", "Evaluate JavaScript and await a promise", []string{"expression"}}, {"query", "Query DOM elements", []string{"selector"}}, {"click", "Dispatch a DOM click", []string{"selector"}}, {"fill", "Fill a DOM form control", []string{"selector", "value"}}, {"screenshot", "Capture the supported CSS viewport", []string{}}, {"pdf", "Export a raster viewport PDF", []string{}}} {
		properties := map[string]any{}
		for _, k := range entry.required {
			properties[k] = map[string]string{"type": "string"}
		}
		list = append(list, map[string]any{"name": entry.name, "description": entry.description, "inputSchema": map[string]any{"type": "object", "properties": properties, "required": entry.required, "additionalProperties": false}})
	}
	return list
}
func call(ctx context.Context, page *browser.Page, raw json.RawMessage) (any, error) {
	var req struct {
		Name      string                     `json:"name"`
		Arguments map[string]json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	known := false
	var required []string
	for _, tool := range tools() {
		m := tool.(map[string]any)
		if m["name"] == req.Name {
			known = true
			required = m["inputSchema"].(map[string]any)["required"].([]string)
		}
	}
	if !known {
		return nil, fmt.Errorf("unknown tool %s", req.Name)
	}
	values := map[string]string{}
	for _, key := range required {
		v, ok := req.Arguments[key]
		if !ok {
			return nil, fmt.Errorf("missing %s", key)
		}
		var value string
		if err := json.Unmarshal(v, &value); err != nil {
			return nil, err
		}
		values[key] = value
	}
	if len(req.Arguments) != len(required) {
		return nil, fmt.Errorf("unexpected arguments")
	}
	var result any
	var err error
	switch req.Name {
	case "navigate":
		err = page.Navigate(ctx, values["url"])
	case "set_content":
		err = page.SetContent(ctx, values["html"])
	case "evaluate":
		result, err = page.Eval(ctx, values["expression"])
	case "query":
		result, err = page.Query(ctx, values["selector"])
	case "click":
		err = page.Click(ctx, values["selector"])
	case "fill":
		err = page.Fill(ctx, values["selector"], values["value"])
	case "screenshot", "pdf":
		snapshot, e := page.Snapshot(ctx)
		if e != nil {
			return nil, e
		}
		if req.Name == "pdf" {
			data, e := render.PDF(snapshot, 794, 1123)
			if e != nil {
				return nil, e
			}
			return map[string]any{"content": []any{map[string]any{"type": "resource", "resource": map[string]string{"uri": "omnibrowser://page/preview.pdf", "mimeType": "application/pdf", "blob": base64.StdEncoding.EncodeToString(data)}}}}, nil
		}
		img, e := render.Screenshot(snapshot, 1024, 768, false)
		if e != nil {
			return nil, e
		}
		var b bytes.Buffer
		if e = png.Encode(&b, img); e != nil {
			return nil, e
		}
		return map[string]any{"content": []any{map[string]string{"type": "image", "mimeType": "image/png", "data": base64.StdEncoding.EncodeToString(b.Bytes())}}}, nil
	}
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = map[string]bool{"ok": true}
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return map[string]any{"content": []any{map[string]string{"type": "text", "text": string(data)}}}, nil
}
