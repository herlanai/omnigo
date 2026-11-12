package cdp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser"
)

func TestAuthenticatedCDPEvaluationAndUnsupportedMethods(t *testing.T) {
	page := omnibrowser.New(omnibrowser.Options{})
	defer page.Close()
	ctx := context.Background()
	if err := page.SetContent(ctx, "<title>CDP</title><h1>Go</h1>"); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(Handler(page, "test-token"))
	defer server.Close()
	response, err := http.Get(server.URL + "/json/version")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatal(response.Status)
	}
	header := http.Header{"Authorization": []string{"Bearer test-token"}}
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/devtools/page/1", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	for _, test := range []struct {
		request string
		want    string
	}{{`{"id":1,"method":"Runtime.evaluate","params":{"expression":"document.title","returnByValue":true}}`, `"value":"CDP"`}, {`{"id":2,"method":"Page.setDocumentContent","params":{"html":"<title>Changed</title>"}}`, `"result":{}`}, {`{"id":3,"method":"Runtime.evaluate","params":{"expression":"document.title"}}`, `"value":"Changed"`}, {`{"id":4,"method":"Network.nonexistent"}`, `unsupported CDP method`}, {`{"id":5,"method":"Runtime.evaluate","params":{"expression":"throw new Error('failure')"}}`, `exceptionDetails`}} {
		if err := conn.Write(ctx, websocket.MessageText, []byte(test.request)); err != nil {
			t.Fatal(err)
		}
		var raw []byte
		for {
			_, raw, err = conn.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var envelope map[string]json.RawMessage
			_ = json.Unmarshal(raw, &envelope)
			if _, ok := envelope["id"]; ok {
				break
			}
		}
		if !json.Valid(raw) || !strings.Contains(string(raw), test.want) {
			t.Fatal(string(raw))
		}
	}
}
