package omniweb

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStreamConsumptionWindow(t *testing.T) {
	f := newStreamFlow("")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if seq, err := f.reserve(ctx, streamWindow); err != nil || seq != 1 {
		t.Fatal(seq, err)
	}
	if err := f.confirm(2); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	if f.bytes != streamWindow {
		t.Fatal("future ACK changed window")
	}
	done := make(chan error, 1)
	go func() { _, err := f.reserve(ctx, 1); done <- err }()
	// A full window cannot admit another reservation before confirmation.
	select {
	case err := <-done:
		t.Fatalf("reservation escaped full window: %v", err)
	default:
	}
	if err := f.confirm(1); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := f.confirm(1); err != nil {
		t.Fatal(err)
	}
	if f.bytes != 1 {
		t.Fatal("duplicate ACK released bytes twice")
	}
	cancel()
	if _, err := f.reserve(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := f.confirm(2); err != nil {
		t.Fatal(err)
	}
	if len(f.pending) != 0 || f.bytes != 0 {
		t.Fatal("retained reservations")
	}
	for _, size := range []int{0, -1, streamWindow + 1} {
		if _, err := f.reserve(t.Context(), size); !errors.Is(err, ErrTooLarge) {
			t.Fatal(size, err)
		}
	}
}
func TestStreamBlockedReservationCancellation(t *testing.T) {
	f := newStreamFlow("")
	f.reserve(t.Context(), streamWindow)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { _, err := f.reserve(ctx, 1); done <- err }()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if f.bytes != streamWindow || f.sent != 1 {
		t.Fatal("canceled reservation consumed capacity")
	}
}
func TestStreamAckAuthorizationAndSession(t *testing.T) {
	s := NewStreams(1)
	token := strings.Repeat("a", 64)
	f := newStreamFlow("read")
	f.reserve(t.Context(), 1)
	s.sessions[token] = f
	for _, tc := range []struct {
		grant, origin, token string
		seq, status          int
	}{
		{"", "http://app.test", token, 1, 403},
		{"read", "http://evil.test", token, 1, 403},
		{"read", "http://app.test", strings.Repeat("b", 64), 1, 410},
		{"read", "http://app.test", token, 2, 400},
		{"read", "http://app.test", token, 1, 204},
		{"read", "http://app.test", token, 1, 204},
	} {
		h, _ := NewHTTP(NewRouter(1, 1024), HTTPOptions{Streams: s, Host: "app.test", Origins: []string{"http://app.test"}, Grants: []string{tc.grant}})
		seq := "1"
		if tc.seq == 2 {
			seq = "2"
		}
		r := streamRequest(`{"v":1,"token":"` + tc.token + `","seq":` + seq + `}`)
		r.URL.Path = Prefix + "stream/ack"
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%+v: %d %s", tc, w.Code, w.Body)
		}
	}
	if f.bytes != 0 {
		t.Fatal("ACK did not release capacity")
	}
}
