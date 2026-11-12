package fetch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestStreamingLimitsRedirectAndCookies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/body", 302)
			return
		}
		w.Header().Set("Set-Cookie", "secret=1")
		w.(http.Flusher).Flush()
		io.WriteString(w, "abcdef")
	}))
	defer server.Close()
	c := New(Options{MaxBodyBytes: 3})
	defer c.Close()
	r, e := c.Do(context.Background(), Request{URL: server.URL})
	if e != nil {
		t.Fatal(e)
	}
	if r.Header.Get("Set-Cookie") != "" {
		t.Fatal("cookie exposed")
	}
	data, e := io.ReadAll(r.Body)
	if string(data) != "abc" || !errors.Is(e, ErrBodyLimit) {
		t.Fatalf("%q %v", data, e)
	}
	manual := New(Options{})
	defer manual.Close()
	r, e = manual.Do(context.Background(), Request{URL: server.URL + "/redirect", Redirect: Manual})
	if e != nil || r.StatusCode != 302 {
		t.Fatal(e)
	}
	r.Body.Close()
	denied := New(Options{Allow: func(u *url.URL) bool { return !strings.HasSuffix(u.Path, "/body") }})
	defer denied.Close()
	if _, e = denied.Do(context.Background(), Request{URL: server.URL + "/redirect"}); e == nil {
		t.Fatal("redirect bypassed policy")
	}
	c.Close()
	if _, e = c.Do(context.Background(), Request{URL: server.URL}); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
}
func TestCloseCancelsStreamingBody(t *testing.T) {
	ready := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		close(ready)
		<-r.Context().Done()
	}))
	defer server.Close()
	c := New(Options{})
	r, e := c.Do(context.Background(), Request{URL: server.URL})
	if e != nil {
		t.Fatal(e)
	}
	<-ready
	c.Close()
	defer r.Body.Close()
	if _, e = io.ReadAll(r.Body); e == nil {
		t.Fatal("body survived client close")
	}
}
