package cdp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	browser "github.com/herlanai/omnigo/omniui/plugins/omnibrowser"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser/render"
)

func (c *connection) frame(t *target) map[string]any {
	s, _ := t.page.Snapshot(context.Background())
	return map[string]any{"id": t.id, "loaderId": "loader-" + fmt.Sprint(t.contextID), "url": s.URL, "securityOrigin": s.URL, "mimeType": "text/html", "domainAndRegistry": "", "secureContextType": "Secure", "crossOriginIsolatedContextType": "NotIsolated", "gatedAPIFeatures": []string{}}
}
func (c *connection) execution(t *target, session string) {
	c.event("Runtime.executionContextCreated", session, map[string]any{"context": map[string]any{"id": t.contextID, "uniqueId": fmt.Sprintf("%s-%d", t.id, t.contextID), "origin": "", "name": "", "auxData": map[string]any{"isDefault": true, "type": "default", "frameId": t.id}}})
}
func (c *connection) loaded(ctx context.Context, t *target, session string) error {
	t.owners = nil
	t.contextID++
	c.event("Page.frameNavigated", session, map[string]any{"frame": c.frame(t), "type": "Navigation"})
	c.event("Runtime.executionContextsCleared", session, map[string]any{})
	c.execution(t, session)
	for world, w := range t.worlds {
		_ = w.page.Close()
		delete(t.worlds, world)
		if _, err := c.world(ctx, t, session, world); err != nil {
			return err
		}
	}
	for _, name := range []string{"init", "DOMContentLoaded", "load"} {
		c.event("Page.lifecycleEvent", session, map[string]any{"frameId": t.id, "loaderId": "loader-" + fmt.Sprint(t.contextID), "name": name, "timestamp": 1})
	}
	c.event("Page.domContentEventFired", session, map[string]int{"timestamp": 1})
	c.event("Page.loadEventFired", session, map[string]int{"timestamp": 1})
	c.event("Page.frameStoppedLoading", session, map[string]string{"frameId": t.id})
	return nil
}
func (c *connection) pageCall(ctx context.Context, t *target, r request, p map[string]json.RawMessage) (any, error) {
	str := func(k string) string { var v string; _ = json.Unmarshal(p[k], &v); return v }
	flag := func(k string) bool { var v bool; _ = json.Unmarshal(p[k], &v); return v }
	empty := map[string]any{}
	active := t.page
	var executionID int
	_ = json.Unmarshal(p["executionContextId"], &executionID)
	if executionID == 0 {
		_ = json.Unmarshal(p["contextId"], &executionID)
	}
	for _, w := range t.worlds {
		if w.id == executionID {
			active = w.page
		}
	}
	if executionID != 0 && executionID != t.contextID && active == t.page {
		return nil, fmt.Errorf("unknown execution context")
	}
	if owner := t.owners[str("objectId")]; owner != nil {
		active = owner
	}

	remote := func(value browser.Remote, err error) (any, error) {
		if err != nil {
			return map[string]any{"result": map[string]string{"type": "undefined"}, "exceptionDetails": map[string]any{"exceptionId": 1, "text": err.Error(), "lineNumber": 0, "columnNumber": 0}}, nil
		}
		if value.ObjectID != "" {
			if t.owners == nil {
				t.owners = map[string]*browser.Page{}
			}
			t.owners[value.ObjectID] = active
		}
		return map[string]any{"result": value}, nil
	}
	switch r.Method {
	case "Target.getTargetInfo":
		return map[string]any{"targetInfo": c.info(t)}, nil
	case "Runtime.enable":
		c.execution(t, r.Session)
		return empty, nil
	case "Performance.enable", "Performance.disable", "Audits.enable", "Audits.disable", "Page.enable", "Runtime.disable", "Page.disable", "Page.setLifecycleEventsEnabled", "Runtime.runIfWaitingForDebugger", "Log.enable", "Log.disable", "Inspector.enable", "Inspector.disable", "Page.setInterceptFileChooserDialog":
		return empty, nil
	case "Network.enable":
		if t.network == nil {
			t.network = map[string]bool{}
		}
		t.network[r.Session] = true
		return empty, nil
	case "Network.disable":
		delete(t.network, r.Session)
		return empty, nil
	case "Network.getResponseBody":
		body, err := t.page.ResponseBody(t.responseIDs[str("requestId")])
		return map[string]any{"body": body, "base64Encoded": true}, err
	case "Page.getFrameTree":
		return map[string]any{"frameTree": map[string]any{"frame": c.frame(t)}}, nil
	case "Page.createIsolatedWorld":
		w, err := c.world(ctx, t, r.Session, str("worldName"))
		if err != nil {
			return nil, err
		}
		return map[string]any{"executionContextId": w.id}, nil
	case "Page.addScriptToEvaluateOnNewDocument":
		if len(t.scripts) >= 64 {
			return nil, fmt.Errorf("init script quota exceeded")
		}
		t.scripts = append(t.scripts, initScript{source: str("source"), world: str("worldName")})
		if name := str("worldName"); name != "" {
			if _, err := c.world(ctx, t, r.Session, name); err != nil {
				return nil, err
			}
		}
		if flag("runImmediately") {
			realmPage := t.page
			if w := t.worlds[str("worldName")]; w != nil {
				realmPage = w.page
			}
			_, err := realmPage.Eval(ctx, str("source"))
			if err != nil {
				return nil, err
			}
		}
		return map[string]string{"identifier": fmt.Sprint(len(t.scripts))}, nil
	case "Page.removeScriptToEvaluateOnNewDocument":
		return nil, fmt.Errorf("init script removal unsupported")
	case "Page.navigate":
		c.event("Page.frameStartedLoading", r.Session, map[string]string{"frameId": t.id})
		u := str("url")
		var err error
		if u == "about:blank" {
			err = t.page.SetContent(ctx, "")
		} else {
			err = t.page.Navigate(ctx, u)
		}
		if err != nil {
			return map[string]any{"frameId": t.id, "errorText": err.Error()}, nil
		}
		if err := c.loaded(ctx, t, r.Session); err != nil {
			return nil, err
		}
		for _, script := range t.scripts {
			realmPage := t.page
			if script.world != "" {
				w, err := c.world(ctx, t, r.Session, script.world)
				if err != nil {
					return nil, err
				}
				realmPage = w.page
			}
			if _, err := realmPage.Eval(ctx, script.source); err != nil {
				return nil, err
			}
		}
		return map[string]any{"frameId": t.id, "loaderId": "loader-" + fmt.Sprint(t.contextID)}, nil
	case "Page.setDocumentContent":
		if err := t.page.SetContent(ctx, str("html")); err != nil {
			return nil, err
		}
		if err := c.loaded(ctx, t, r.Session); err != nil {
			return nil, err
		}
		return empty, nil
	case "Runtime.evaluate":
		return remote(active.EvaluateRemote(ctx, str("expression"), flag("returnByValue"), flag("awaitPromise")))
	case "Runtime.callFunctionOn":
		var args []browser.Argument
		if raw := p["arguments"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
		}
		return remote(active.CallRemote(ctx, str("functionDeclaration"), str("objectId"), args, flag("returnByValue"), flag("awaitPromise")))
	case "Runtime.releaseObject":
		delete(t.owners, str("objectId"))
		return empty, active.Release(ctx, str("objectId"))
	case "Runtime.getProperties":
		props, err := active.Properties(ctx, str("objectId"))
		for _, prop := range props {
			if remote, ok := prop["value"].(browser.Remote); ok && remote.ObjectID != "" {
				if t.owners == nil {
					t.owners = map[string]*browser.Page{}
				}
				t.owners[remote.ObjectID] = active
			}
		}
		return map[string]any{"result": props, "internalProperties": []any{}}, err
	case "Page.captureScreenshot":
		if f := str("format"); f != "" && f != "png" {
			return nil, fmt.Errorf("only PNG supported")
		}
		s, err := t.page.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		img, err := render.Screenshot(s, 1024, 768, false)
		if err != nil {
			return nil, err
		}
		var b bytes.Buffer
		if err = png.Encode(&b, img); err != nil {
			return nil, err
		}
		return map[string]string{"data": base64.StdEncoding.EncodeToString(b.Bytes())}, nil
	case "Page.printToPDF":
		if mode := str("transferMode"); mode != "" && mode != "ReturnAsBase64" {
			return nil, fmt.Errorf("only ReturnAsBase64 supported")
		}
		s, err := t.page.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		data, err := render.PDF(s, 794, 1123)
		if err != nil {
			return nil, err
		}
		return map[string]string{"data": base64.StdEncoding.EncodeToString(data)}, nil
	case "Page.getLayoutMetrics":
		size := map[string]int{"x": 0, "y": 0, "width": 1024, "height": 768}
		viewport := map[string]any{"pageX": 0, "pageY": 0, "clientWidth": 1024, "clientHeight": 768, "scale": 1, "zoom": 1}
		return map[string]any{"contentSize": size, "cssContentSize": size, "layoutViewport": viewport, "visualViewport": viewport, "cssLayoutViewport": viewport, "cssVisualViewport": viewport}, nil
	default:
		return nil, fmt.Errorf("unsupported CDP method: %s", r.Method)
	}
}

func (c *connection) world(ctx context.Context, t *target, session, name string) (*realm, error) {
	if t.worlds == nil {
		t.worlds = map[string]*realm{}
	}
	w := t.worlds[name]
	if w == nil {
		if len(t.worlds) >= 64 {
			return nil, fmt.Errorf("isolated world limit exceeded")
		}
		page, err := t.page.NewRealm(ctx)
		if err != nil {
			return nil, err
		}
		c.next++
		w = &realm{page: page, id: 100000 + c.next}
		t.worlds[name] = w
	}
	c.event("Runtime.executionContextCreated", session, map[string]any{"context": map[string]any{"id": w.id, "uniqueId": fmt.Sprintf("utility-%s-%d", t.id, w.id), "origin": "", "name": name, "auxData": map[string]any{"isDefault": false, "type": "isolated", "frameId": t.id}}})
	return w, nil
}
