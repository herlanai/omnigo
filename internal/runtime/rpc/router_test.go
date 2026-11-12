package rpc

import (
	"context"
	"errors"
	"testing"
)

type request struct {
	Value int `json:"value"`
}

func TestUninitializedRouter(t *testing.T) {
	var r Router
	if err := Bind(&r, "test", "", func(context.Context, request) (int, error) { return 0, nil }); err == nil {
		t.Fatal("uninitialized router accepted")
	}
	if _, err := r.Call(context.Background(), "test", nil, nil); err == nil {
		t.Fatal("uninitialized call accepted")
	}
	if _, err := New(1, 1).Call(nil, "test", nil, nil); err == nil {
		t.Fatal("nil context accepted")
	}
}

func TestValidationAndPermissions(t *testing.T) {
	r := New(1, 64)
	if err := Bind(r, "double", "compute", func(ctx context.Context, req request) (int, error) { return req.Value * 2, nil }); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"value":2,"extra":3}`, `{"value":2} {}`, `bad`} {
		if _, err := r.Call(context.Background(), "double", []byte(raw), []string{"compute"}); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if _, err := r.Call(context.Background(), "double", []byte(`{"value":2}`), nil); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	out, err := r.Call(context.Background(), "double", []byte(`{"value":2}`), []string{"compute"})
	if err != nil || string(out) != "4" {
		t.Fatalf("%s %v", out, err)
	}
	if _, err := r.Call(context.Background(), "double", make([]byte, 65), nil); !errors.Is(err, ErrTooLarge) {
		t.Fatal(err)
	}
}

func TestBoundedCallsAndPanicRecovery(t *testing.T) {
	r := New(1, 64)
	entered, release := make(chan struct{}), make(chan struct{})
	if err := Bind(r, "wait", "", func(ctx context.Context, req request) (int, error) {
		close(entered)
		<-release
		panic("private details")
	}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := r.Call(context.Background(), "wait", []byte(`{}`), nil); done <- err }()
	<-entered
	_, err := r.Call(context.Background(), "wait", []byte(`{}`), nil)
	close(release)
	if !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, ErrInternal) {
		t.Fatal(err)
	}
	if err := Bind(r, "ok", "", func(context.Context, request) (int, error) { return 1, nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Call(context.Background(), "ok", []byte(`{}`), nil); err != nil {
		t.Fatal("slot leaked:", err)
	}
}

func FuzzRequest(f *testing.F) {
	r := New(1, 4096)
	if err := Bind(r, "decode", "", func(context.Context, request) (bool, error) { return true, nil }); err != nil {
		f.Fatal(err)
	}
	f.Add([]byte(`{"value":2}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, raw []byte) { _, _ = r.Call(context.Background(), "decode", raw, nil) })
}
