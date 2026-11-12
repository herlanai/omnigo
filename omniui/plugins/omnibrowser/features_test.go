package omnibrowser

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStylesPromisesAndRealms(t *testing.T) {
	ctx := context.Background()
	p := New(Options{})
	defer p.Close()
	if err := p.SetContent(ctx, `<style>p {color: red} #x {color: blue !important}</style><p id="x" style="color:green">one</p>`); err != nil {
		t.Fatal(err)
	}
	check := func(p *Page, js, want string) {
		t.Helper()
		got, err := p.Eval(ctx, js)
		if err != nil || string(got) != want {
			t.Fatalf("%s: %s %v; want %s", js, got, err, want)
		}
	}
	check(p, `getComputedStyle(document.querySelector('#x')).color`, `"blue"`)
	check(p, `document.querySelector('#x').style.setProperty('color','yellow','important');getComputedStyle(document.querySelector('#x')).color`, `"yellow"`)
	check(p, `new Promise(resolve => setTimeout(()=>resolve(42),0))`, `42`)
	check(p, `globalThis.secret=7;localStorage.setItem('key','value');true`, `true`)
	realm, err := p.NewRealm(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer realm.Close()
	check(realm, `typeof secret`, `"undefined"`)
	check(realm, `document.querySelector('#x').textContent='two';localStorage.getItem('key')`, `"value"`)
	check(p, `document.querySelector('#x').textContent`, `"two"`)
	handle, err := p.EvaluateRemote(ctx, `({answer:42})`, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.SetContent(ctx, `<p>next</p>`); err != nil {
		t.Fatal(err)
	}
	if _, err = realm.Eval(ctx, `1`); !errors.Is(err, ErrClosed) {
		t.Fatalf("old realm survived navigation: %v", err)
	}
	if _, err = p.Properties(ctx, handle.ObjectID); err == nil {
		t.Fatal("stale handle survived navigation")
	}
	deadline, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	if _, err = p.Eval(deadline, `new Promise(()=>{})`); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	check(p, `21*2`, `42`)
}

func TestFetchCookiesBodyAndOrigin(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "yes", Path: "/"})
			io.WriteString(w, "<title>fetch</title>")
			return
		}
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "https://example.com", http.StatusFound)
			return
		}
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value != "yes" {
			http.Error(w, "no cookie", 401)
			return
		}
		data, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write(data)
	}))
	defer site.Close()
	p := New(Options{AllowPrivateNetwork: true})
	defer p.Close()
	ctx := context.Background()
	if err := p.Navigate(ctx, site.URL); err != nil {
		t.Fatal(err)
	}
	got, err := p.Eval(ctx, `fetch('/echo',{method:'POST',body:'{"ok":true}'}).then(async r=>[r.status,(await r.json()).ok,r.bodyUsed])`)
	if err != nil || string(got) != `[201,true,true]` {
		t.Fatal(string(got), err)
	}
	for _, script := range []string{`fetch('https://example.com')`, `fetch('/redirect')`, `fetch('/echo',{headers:{Cookie:'injected'}})`} {
		if _, err = p.Eval(ctx, script); err == nil {
			t.Fatalf("accepted %s", script)
		}
	}
}

func TestRemoteSpecialNumbers(t *testing.T) {
	p := New(Options{})
	defer p.Close()
	ctx := context.Background()
	if err := p.SetContent(ctx, ""); err != nil {
		t.Fatal(err)
	}
	for _, expression := range []string{"NaN", "Infinity", "-Infinity", "-0", "42n"} {
		r, err := p.EvaluateRemote(ctx, expression, true, false)
		if err != nil || r.UnserializableValue != expression {
			t.Fatal(expression, r, err)
		}
	}
}
