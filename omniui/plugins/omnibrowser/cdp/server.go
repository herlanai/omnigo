// Package cdp implements the documented browser/session automation subset.
package cdp

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"net/http"
	browser "github.com/herlanai/omnigo/omniui/plugins/omnibrowser"
	"strings"
)

type request struct {
	ID      int64           `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Session string          `json:"sessionId"`
}
type realm struct {
	page *browser.Page
	id   int
}
type initScript struct{ source, world string }
type target struct {
	responseIDs map[string]string
	network     map[string]bool
	owners      map[string]*browser.Page
	worlds      map[string]*realm
	page        *browser.Page
	id, context string
	owned       bool
	contextID   int
	scripts     []initScript
}
type connection struct {
	targets        map[string]*target
	sessions       map[string]string
	contexts       map[string]bool
	next           int
	auto, discover bool
	events         []map[string]any
	seed           *browser.Page
}

func Handler(page *browser.Page, token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/json/version", func(w http.ResponseWriter, r *http.Request) {
		scheme := "ws"
		if r.TLS != nil {
			scheme = "wss"
		}
		writeJSON(w, map[string]any{"Browser": "OmniBrowser/0.2", "Protocol-Version": "1.3", "webSocketDebuggerUrl": scheme + "://" + r.Host + "/devtools/browser/omni"})
	})
	list := func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []any{map[string]string{"id": "1", "type": "page", "title": "OmniBrowser", "url": "about:blank", "webSocketDebuggerUrl": "ws://" + r.Host + "/devtools/page/1"}})
	}
	mux.HandleFunc("/json/version/", func(w http.ResponseWriter, r *http.Request) { r.URL.Path = "/json/version"; mux.ServeHTTP(w, r) })
	mux.HandleFunc("/json/list", list)
	mux.HandleFunc("/json", list)
	handle := func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		conn.SetReadLimit(2 << 20)
		c := &connection{targets: map[string]*target{"1": {page: page, id: "1", contextID: 1}}, sessions: map[string]string{}, contexts: map[string]bool{}, next: 1, seed: page}
		defer func() {
			for _, t := range c.targets {
				for _, w := range t.worlds {
					_ = w.page.Close()
				}
				if t.owned {
					_ = t.page.Close()
				}
			}
		}()
		send := func(v any) error {
			data, err := json.Marshal(v)
			if err != nil {
				return err
			}
			return conn.Write(r.Context(), websocket.MessageText, data)
		}
		for {
			kind, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			if kind != websocket.MessageText {
				continue
			}
			var req request
			if json.Unmarshal(data, &req) != nil {
				return
			}
			c.events = nil
			result, err := c.call(r.Context(), req)
			lifecycle := c.events
			c.events = nil
			c.networkEvents()
			c.events = append(c.events, lifecycle...)
			reply := map[string]any{"id": req.ID}
			if req.Session != "" {
				reply["sessionId"] = req.Session
			}
			if err != nil {
				reply["error"] = map[string]any{"code": -32000, "message": err.Error()}
			} else {
				reply["result"] = result
			}
			for _, event := range c.events {
				if send(event) != nil {
					return
				}
			}
			if send(reply) != nil {
				return
			}
		}
	}
	mux.HandleFunc("/devtools/browser/omni", handle)
	mux.HandleFunc("/devtools/page/1", handle)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		provided := strings.TrimPrefix(auth, "Bearer ")
		if token == "" || !strings.HasPrefix(auth, "Bearer ") || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			http.Error(w, "bearer token required", 401)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func (c *connection) event(method, session string, params any) {
	e := map[string]any{"method": method, "params": params}
	if session != "" {
		e["sessionId"] = session
	}
	c.events = append(c.events, e)
}
func (c *connection) info(t *target) map[string]any {
	s, _ := t.page.Snapshot(context.Background())
	u := s.URL
	if u == "" {
		u = "about:blank"
	}
	contextID := t.context
	if contextID == "" {
		contextID = "default"
	}
	return map[string]any{"targetId": t.id, "type": "page", "title": s.Title, "url": u, "attached": true, "browserContextId": contextID, "canAccessOpener": false}
}
func (c *connection) attach(t *target, parent string) string {
	c.next++
	id := fmt.Sprintf("session-%d", c.next)
	c.sessions[id] = t.id
	c.event("Target.attachedToTarget", parent, map[string]any{"sessionId": id, "targetInfo": c.info(t), "waitingForDebugger": false})
	return id
}
func (c *connection) call(ctx context.Context, r request) (any, error) {
	p := map[string]json.RawMessage{}
	if len(r.Params) > 0 {
		if err := json.Unmarshal(r.Params, &p); err != nil {
			return nil, err
		}
	}
	str := func(k string) string { var v string; _ = json.Unmarshal(p[k], &v); return v }
	boolean := func(k string) bool { var v bool; _ = json.Unmarshal(p[k], &v); return v }
	empty := map[string]any{}
	switch r.Method {
	case "Browser.getVersion":
		return map[string]string{"protocolVersion": "1.3", "product": "OmniBrowser/0.2.0", "revision": "OmniBrowser-Go-subset", "userAgent": "OmniBrowser/0.2 Headless", "jsVersion": "goja"}, nil
	case "Browser.getBrowserCommandLine":
		return map[string]any{"arguments": []string{}}, nil
	case "Target.getBrowserContexts":
		ids := []string{}
		for id := range c.contexts {
			ids = append(ids, id)
		}
		return map[string]any{"browserContextIds": ids}, nil
	case "Target.createBrowserContext":
		if len(c.contexts) >= 32 {
			return nil, fmt.Errorf("browser context limit exceeded")
		}
		c.next++
		id := fmt.Sprintf("context-%d", c.next)
		c.contexts[id] = true
		return map[string]string{"browserContextId": id}, nil
	case "Target.disposeBrowserContext":
		id := str("browserContextId")
		for key, t := range c.targets {
			if t.context == id {
				for _, w := range t.worlds {
					_ = w.page.Close()
				}
				if t.owned {
					_ = t.page.Close()
				}
				delete(c.targets, key)
				for session, targetID := range c.sessions {
					if targetID == key {
						delete(c.sessions, session)
						c.event("Target.detachedFromTarget", "", map[string]string{"sessionId": session, "targetId": key})
					}
				}
				c.event("Target.targetDestroyed", "", map[string]string{"targetId": key})
			}
		}
		delete(c.contexts, id)
		return empty, nil
	case "Target.getTargets":
		infos := []any{}
		for _, t := range c.targets {
			infos = append(infos, c.info(t))
		}
		return map[string]any{"targetInfos": infos}, nil
	case "Target.setDiscoverTargets":
		c.discover = boolean("discover")
		if c.discover {
			for _, t := range c.targets {
				c.event("Target.targetCreated", "", map[string]any{"targetInfo": c.info(t)})
			}
		}
		return empty, nil
	case "Target.setAutoAttach":
		if r.Session != "" {
			return empty, nil
		}
		c.auto = boolean("autoAttach")
		if c.auto {
			for _, t := range c.targets {
				already := false
				for _, id := range c.sessions {
					if id == t.id {
						already = true
					}
				}
				if !already {
					c.attach(t, "")
				}
			}
		}
		return empty, nil
	case "Target.attachToBrowserTarget":
		if len(c.sessions) >= 128 {
			return nil, fmt.Errorf("session limit exceeded")
		}
		c.next++
		id := fmt.Sprintf("browser-session-%d", c.next)
		c.sessions[id] = "browser"
		return map[string]string{"sessionId": id}, nil
	case "Target.attachToTarget":
		if len(c.sessions) >= 128 {
			return nil, fmt.Errorf("session limit exceeded")
		}
		t := c.targets[str("targetId")]
		if t == nil {
			return nil, fmt.Errorf("unknown target")
		}
		return map[string]string{"sessionId": c.attach(t, r.Session)}, nil
	case "Target.detachFromTarget":
		delete(c.sessions, str("sessionId"))
		return empty, nil
	case "Target.createTarget":
		if len(c.targets) >= 32 {
			return nil, fmt.Errorf("target limit exceeded")
		}
		contextID := str("browserContextId")
		if contextID != "" && !c.contexts[contextID] {
			return nil, fmt.Errorf("unknown context")
		}
		c.next++
		id := fmt.Sprint(c.next)
		page := c.seed.NewSibling()
		if err := page.SetContent(ctx, "<title></title>"); err != nil {
			_ = page.Close()
			return nil, err
		}
		if u := str("url"); u != "" && u != "about:blank" {
			if err := page.Navigate(ctx, u); err != nil {
				_ = page.Close()
				return nil, err
			}
		}
		t := &target{page: page, id: id, context: contextID, owned: true, contextID: 1}
		c.targets[id] = t
		if c.discover {
			c.event("Target.targetCreated", "", map[string]any{"targetInfo": c.info(t)})
		}
		if c.auto {
			c.attach(t, "")
		}
		return map[string]string{"targetId": id}, nil
	case "Target.closeTarget":
		id := str("targetId")
		if t := c.targets[id]; t != nil {
			for _, w := range t.worlds {
				_ = w.page.Close()
			}
			if t.owned {
				_ = t.page.Close()
			}
			delete(c.targets, id)
			for session, target := range c.sessions {
				if target == id {
					c.event("Target.detachedFromTarget", "", map[string]string{"sessionId": session, "targetId": id})
					delete(c.sessions, session)
				}
			}
			c.event("Target.targetDestroyed", "", map[string]string{"targetId": id})
		}
		return map[string]bool{"success": true}, nil
	}
	id := "1"
	if r.Session != "" {
		var ok bool
		id, ok = c.sessions[r.Session]
		if !ok {
			return nil, fmt.Errorf("unknown session")
		}
	}
	t := c.targets[id]
	if t == nil {
		return nil, fmt.Errorf("target closed")
	}
	return c.pageCall(ctx, t, r, p)
}
