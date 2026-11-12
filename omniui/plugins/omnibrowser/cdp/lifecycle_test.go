package cdp

import (
	"context"
	"encoding/json"
	browser "github.com/herlanai/omnigo/omniui/plugins/omnibrowser"
	"testing"
)

func TestTargetsRealmsAndHandleLifetime(t *testing.T) {
	ctx := context.Background()
	seed := browser.New(browser.Options{})
	defer seed.Close()
	if err := seed.SetContent(ctx, "<p>seed</p>"); err != nil {
		t.Fatal(err)
	}
	c := &connection{targets: map[string]*target{}, sessions: map[string]string{}, contexts: map[string]bool{}, seed: seed}
	call := func(method, session string, params any) any {
		t.Helper()
		raw, _ := json.Marshal(params)
		v, e := c.call(ctx, request{Method: method, Session: session, Params: raw})
		if e != nil {
			t.Fatal(method, e)
		}
		return v
	}
	contextID := call("Target.createBrowserContext", "", nil).(map[string]string)["browserContextId"]
	id := call("Target.createTarget", "", map[string]any{"browserContextId": contextID}).(map[string]string)["targetId"]
	session := call("Target.attachToTarget", "", map[string]string{"targetId": id}).(map[string]string)["sessionId"]
	worldID := call("Page.createIsolatedWorld", session, map[string]string{"worldName": "test"}).(map[string]any)["executionContextId"]
	w := c.targets[id].worlds["test"].page
	r := call("Runtime.evaluate", session, map[string]any{"expression": "({a:1})", "contextId": worldID}).(map[string]any)["result"].(browser.Remote)
	if c.targets[id].owners[r.ObjectID] != w {
		t.Fatal("wrong handle owner")
	}
	call("Runtime.releaseObject", session, map[string]string{"objectId": r.ObjectID})
	if len(c.targets[id].owners) != 0 {
		t.Fatal("released owner retained")
	}
	call("Page.navigate", session, map[string]string{"url": "about:blank"})
	if _, e := w.Eval(ctx, "1"); e == nil {
		t.Fatal("old world alive")
	}
	raw, _ := json.Marshal(map[string]any{"expression": "1", "contextId": worldID})
	if _, e := c.call(ctx, request{Method: "Runtime.evaluate", Session: session, Params: raw}); e == nil {
		t.Fatal("stale context accepted")
	}
	w = c.targets[id].worlds["test"].page
	call("Target.disposeBrowserContext", "", map[string]string{"browserContextId": contextID})
	if len(c.targets) != 0 || len(c.sessions) != 0 || len(c.contexts) != 0 {
		t.Fatal("context resources retained")
	}
	if _, e := w.Eval(ctx, "1"); e == nil {
		t.Fatal("world survived context disposal")
	}
}
