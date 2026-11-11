package omniweb

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"omnigo.dev/omnigo/runtime/rpc"
)

func host(t testing.TB, counter *int) *HTTP {
	t.Helper()
	r := rpc.New(4, 1024)
	type input struct {
		Value int `json:"value"`
	}
	if err := rpc.Bind(r, "math.double", "compute", func(ctx context.Context, in input) (int, error) { *counter++; return in.Value * 2, nil }); err != nil {
		t.Fatal(err)
	}
	if err := rpc.Bind(r, "fail", "", func(context.Context, input) (int, error) { return 0, errors.New("PRIVATE_SECRET") }); err != nil {
		t.Fatal(err)
	}
	h, err := NewHTTP(r, HTTPOptions{Host: "127.0.0.1:7331", Origins: []string{"http://127.0.0.1:7331"}, Grants: []string{"compute"}, MaxRequestBytes: 1024, MaxBatch: 4})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func request(path, body string) *http.Request {
	r := httptest.NewRequest("POST", "http://127.0.0.1:7331"+Prefix+path, strings.NewReader(body))
	r.Header.Set("Origin", "http://127.0.0.1:7331")
	r.Header.Set("Content-Type", "application/json")
	return r
}

const valid = `{"v":1,"id":"one","method":"math.double","params":{"value":21}}`

func TestHTTPCallAndRedaction(t *testing.T) {
	count := 0
	h := host(t, &count)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request("call", valid))
	var response Response
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || response.ID != "one" || string(response.Result) != "42" || count != 1 {
		t.Fatal(w.Code, w.Body.String(), count)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, request("call", strings.Replace(valid, "math.double", "fail", 1)))
	if strings.Contains(w.Body.String(), "PRIVATE_SECRET") || !strings.Contains(w.Body.String(), `"internal"`) {
		t.Fatal(w.Body.String())
	}
}

func TestHTTPBoundary(t *testing.T) {
	cases := []struct {
		name   string
		modify func(*http.Request)
		body   string
		status int
	}{
		{"origin", func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }, valid, 403},
		{"missing origin", func(r *http.Request) { r.Header.Del("Origin") }, valid, 403},
		{"rebinding", func(r *http.Request) { r.Host = "evil.example:7331" }, valid, 403},
		{"media", func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, valid, 415},
		{"method", func(r *http.Request) { r.Method = "GET" }, valid, 405},
		{"large", nil, strings.Repeat(" ", 1025), 413},
		{"version", nil, strings.Replace(valid, `"v":1`, `"v":2`, 1), 400},
		{"unknown", nil, strings.Replace(valid, `"v":1`, `"admin":true,"v":1`, 1), 400},
		{"trailing", nil, valid + `{}`, 400},
		{"null", nil, `null`, 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			count := 0
			h := host(t, &count)
			r := request("call", tc.body)
			if tc.modify != nil {
				tc.modify(r)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || count != 0 {
				t.Fatal(w.Code, w.Body.String(), count)
			}
			if w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("unexpected CORS")
			}
		})
	}
}

func TestBatchValidationBeforeSideEffects(t *testing.T) {
	count := 0
	h := host(t, &count)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request("batch", "["+valid+","+valid+"]"))
	if w.Code != 400 || count != 0 {
		t.Fatal(w.Code, count)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, request("batch", "["+valid+","+strings.Replace(valid, `"one"`, `"two"`, 1)+"]"))
	var responses []Response
	if err := json.Unmarshal(w.Body.Bytes(), &responses); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || count != 2 || len(responses) != 2 || responses[1].ID != "two" {
		t.Fatal(w.Code, count, w.Body.String())
	}
}

func TestCancellationAndParams(t *testing.T) {
	count := 0
	h := host(t, &count)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request("call", valid).WithContext(ctx))
	if count != 0 || !strings.Contains(w.Body.String(), `"canceled"`) {
		t.Fatal(w.Body.String(), count)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, request("call", strings.Replace(valid, `"value":21`, `"other":21`, 1)))
	if count != 0 || !strings.Contains(w.Body.String(), `"invalid_params"`) {
		t.Fatal(w.Body.String(), count)
	}
}

func TestAssetsShellAndBoundaries(t *testing.T) {
	h, err := Assets(fstest.MapFS{"index.html": {Data: []byte("shell")}, "assets/app.js": {Data: []byte("script")}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, accept string
		status       int
		body         string
	}{
		{"/", "text/html", 200, "shell"}, {"/settings/profile", "text/html", 200, "shell"},
		{"/assets/app.js", "*/*", 200, "script"}, {"/missing.js", "text/html", 404, ""},
		{"/omniweb/v1/missing", "text/html", 404, ""}, {"/../go.mod", "text/html", 404, ""},
		{"/.env", "text/html", 404, ""}, {"/api/unknown", "application/json", 404, ""},
	} {
		t.Run(tc.path, func(t *testing.T) {
			r := httptest.NewRequest("GET", tc.path, nil)
			r.Header.Set("Accept", tc.accept)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || (tc.body != "" && w.Body.String() != tc.body) {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
}

func FuzzEnvelope(f *testing.F) {
	f.Add(valid)
	f.Add(`null`)
	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 2048 {
			return
		}
		count := 0
		h := host(t, &count)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, request("call", body))
		if w.Code != 200 && count != 0 {
			t.Fatal("invalid frame executed")
		}
	})
}

func BenchmarkHTTPCall(b *testing.B) {
	count := 0
	h := host(b, &count)
	b.ReportAllocs()
	for b.Loop() {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, request("call", valid))
		if w.Code != 200 {
			b.Fatal(w.Code)
		}
	}
}
