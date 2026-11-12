// Package cdp exposes an explicit, small CDP subset for one document target.
// It is not a Puppeteer/Playwright compatibility claim.
package cdp

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"omnigo.dev/omnigo/omnibrowser"
	"omnigo.dev/omnigo/omnibrowser/render"
)

// Handler owns no page: the caller must close it after shutting down the server.
// A nonempty bearer token is required. The CLI binds only to loopback.
func Handler(page *omnibrowser.Page, token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/json/version", func(w http.ResponseWriter, r *http.Request) {
		scheme := "ws"
		if r.TLS != nil {
			scheme = "wss"
		}
		writeJSON(w, map[string]any{"Browser": "OmniBrowser/0.1", "Protocol-Version": "1.3", "webSocketDebuggerUrl": scheme + "://" + r.Host + "/devtools/page/1"})
	})
	mux.HandleFunc("/devtools/page/1", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		conn.SetReadLimit(2 << 20)
		for {
			kind, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			if kind != websocket.MessageText {
				continue
			}
			var request struct {
				ID     int64           `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if err = json.Unmarshal(data, &request); err != nil {
				return
			}
			result, err := call(r.Context(), page, request.Method, request.Params)
			response := map[string]any{"id": request.ID}
			if err != nil {
				response["error"] = map[string]any{"code": -32000, "message": err.Error()}
			} else {
				response["result"] = result
			}
			encoded, err := json.Marshal(response)
			if err != nil {
				return
			}
			if err = conn.Write(r.Context(), websocket.MessageText, encoded); err != nil {
				return
			}
		}
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			http.Error(w, "bearer token required", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
func call(ctx context.Context, page *omnibrowser.Page, method string, params json.RawMessage) (any, error) {
	var p struct {
		URL           string `json:"url"`
		HTML          string `json:"html"`
		Expression    string `json:"expression"`
		ReturnByValue bool   `json:"returnByValue"`
		Format        string `json:"format"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
	}
	switch method {
	case "Browser.getVersion":
		return map[string]string{"protocolVersion": "1.3", "product": "OmniBrowser/0.1", "revision": "independent-go-subset", "userAgent": "OmniBrowser/0.1", "jsVersion": "goja"}, nil
	case "Page.setDocumentContent":
		if err := page.SetContent(ctx, p.HTML); err != nil {
			return nil, err
		}
		return map[string]any{}, nil
	case "Page.navigate":
		if err := page.Navigate(ctx, p.URL); err != nil {
			return nil, err
		}
		return map[string]any{"frameId": "main"}, nil
	case "Runtime.evaluate":
		result, err := page.Eval(ctx, p.Expression)
		if err != nil {
			return map[string]any{"result": map[string]any{"type": "undefined"}, "exceptionDetails": map[string]any{"exceptionId": 1, "text": err.Error(), "lineNumber": 0, "columnNumber": 0}}, nil
		}
		var value any
		if err = json.Unmarshal(result, &value); err != nil {
			return nil, err
		}
		typ := "object"
		switch value.(type) {
		case string:
			typ = "string"
		case bool:
			typ = "boolean"
		case float64:
			typ = "number"
		}
		if typ == "object" && value != nil && !p.ReturnByValue {
			return nil, fmt.Errorf("object handles unsupported: request returnByValue")
		}
		remote := map[string]any{"type": typ, "value": value}
		if value == nil {
			remote["subtype"] = "null"
		}
		return map[string]any{"result": remote}, nil
	case "Page.captureScreenshot":
		if p.Format != "" && p.Format != "png" {
			return nil, fmt.Errorf("only PNG semantic previews supported")
		}
		snapshot, err := page.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		img, err := render.Screenshot(snapshot, 1024, 768, false)
		if err != nil {
			return nil, err
		}
		var buffer bytes.Buffer
		if err = png.Encode(&buffer, img); err != nil {
			return nil, err
		}
		return map[string]string{"data": base64.StdEncoding.EncodeToString(buffer.Bytes())}, nil
	default:
		return nil, fmt.Errorf("unsupported CDP method: %s", method)
	}
}
