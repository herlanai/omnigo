package omniweb

import (
	"context"
	"encoding/json"
	"testing"
)

func TestNativePermissionsAndBatchValidation(t *testing.T) {
	r := NewRouter(4, 1024)
	calls := 0
	if err := Bind(r, "echo", "trusted", func(ctx context.Context, v string) (string, error) {
		calls++
		if Source(ctx) == nil {
			t.Fatal("missing source")
		}
		return v, nil
	}); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"v":1,"id":"a","method":"echo","params":"中文"}`)
	out, err := NativeExchange(r, nil)(t.Context(), raw)
	if err != nil {
		t.Fatal(err)
	}
	var response Response
	json.Unmarshal(out, &response)
	if response.Error == nil || response.Error.Code != "forbidden" || calls != 0 {
		t.Fatal(string(out))
	}
	exchange := NativeExchange(r, []string{"trusted"})
	if _, err = exchange(t.Context(), raw); err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
	duplicate := append(append(append([]byte{'['}, raw...), ','), raw...)
	duplicate = append(duplicate, ']')
	if _, err = exchange(t.Context(), duplicate); err == nil || calls != 1 {
		t.Fatal("batch executed before validation")
	}
}

func TestNativeExchangeSharesHTTPHostIdentity(t *testing.T) {
	router := NewRouter(4, 1024)
	var observed *RequestSource
	if err := Bind(router, "source", "", func(ctx context.Context, _ struct{}) (bool, error) { observed = Source(ctx); return true, nil }); err != nil {
		t.Fatal(err)
	}
	h, err := NewHTTP(router, HTTPOptions{Host: "127.0.0.1:7331", Origins: []string{"http://127.0.0.1:7331"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.NativeExchange()(t.Context(), []byte(`{"v":1,"id":"1","method":"source","params":{}}`)); err != nil {
		t.Fatal(err)
	}
	if observed != h.source {
		t.Fatal("native and HTTP requests have different host identities")
	}
}
