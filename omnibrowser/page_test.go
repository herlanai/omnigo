package omnibrowser

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testHTML = `<!doctype html><title>Test</title><h1 id="title">Hello</h1><input id="name" value="Ada"><button id="go">Run</button><ul id="list"><li class="item">One</li></ul><script>
document.querySelector('#go').addEventListener('click', function(){document.getElementById('title').textContent='Hello '+document.querySelector('#name').value;});
console.log('ready');</script>`

func TestDOMJavaScriptInteractionAndSnapshots(t *testing.T) {
	ctx := context.Background()
	p := New(Options{})
	defer p.Close()
	if err := p.SetContent(ctx, testHTML); err != nil {
		t.Fatal(err)
	}
	if err := p.Fill(ctx, "#name", "世界"); err != nil {
		t.Fatal(err)
	}
	if err := p.Click(ctx, "#go"); err != nil {
		t.Fatal(err)
	}
	value, err := p.Eval(ctx, `document.querySelector('#title').textContent`)
	if err != nil || string(value) != `"Hello 世界"` {
		t.Fatal(string(value), err)
	}
	value, err = p.Eval(ctx, `var li=document.createElement('li');li.className='item';li.textContent='Two';document.querySelector('#list').appendChild(li);Array.from(document.querySelectorAll('li.item')).map(x=>x.textContent)`)
	if err != nil || string(value) != `["One","Two"]` {
		t.Fatal(string(value), err)
	}
	if _, err = p.Eval(ctx, `li.appendChild(document.body)`); err == nil {
		t.Fatal("cycle accepted")
	}
	if _, err = p.Eval(ctx, `document.title='Updated';document.querySelector('#list').innerHTML='<li>Three</li>'`); err != nil {
		t.Fatal(err)
	}
	snapshot, err := p.Snapshot(ctx)
	if err != nil || snapshot.Title != "Updated" || !strings.Contains(snapshot.Text, "Hello 世界") || strings.Contains(snapshot.Text, "console.log") {
		t.Fatal(snapshot, err)
	}
	matches, err := p.Query(ctx, "ul > li")
	if err != nil || len(matches) != 1 || matches[0].Text != "Three" {
		t.Fatal(matches, err)
	}
	snapshot.Logs[0] = "changed"
	again, _ := p.Snapshot(ctx)
	if again.Logs[0] != "ready" {
		t.Fatal("snapshot alias")
	}
	if _, err = p.Query(ctx, "["); err == nil {
		t.Fatal("invalid selector")
	}
	p.Close()
	if _, err = p.Eval(ctx, "1"); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}
func TestCancellationLimitsAndRecovery(t *testing.T) {
	p := New(Options{Timeout: 30 * time.Millisecond, MaxBytes: 1024, MaxNodes: 30})
	defer p.Close()
	ctx := context.Background()
	if err := p.SetContent(ctx, "<p>Test</p>"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Eval(ctx, "while(true){}"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	result, err := p.Eval(ctx, "6*7")
	if err != nil || string(result) != "42" {
		t.Fatal(string(result), err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = p.Eval(canceled, "1"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err = p.SetContent(ctx, strings.Repeat("x", 1025)); err == nil {
		t.Fatal("byte limit")
	}
	if err = p.SetContent(ctx, strings.Repeat("<p>x</p>", 40)); err == nil {
		t.Fatal("node limit")
	}
	if _, err = p.Eval(ctx, `for(var i=0;i<50;i++)document.createElement('div')`); err == nil {
		t.Fatal("mutation node limit")
	}
}
func TestNetworkRedirectCookiesBoundsAndPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "ok", Path: "/"})
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value != "ok" {
				http.Error(w, "missing cookie", 400)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<title>Session</title><p>cookie retained</p>"))
		case "/large":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(strings.Repeat("x", 2049)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	ctx := context.Background()
	blocked := New(Options{})
	defer blocked.Close()
	if err := blocked.Navigate(ctx, server.URL); err == nil {
		t.Fatal("private network accepted")
	}
	p := New(Options{AllowPrivateNetwork: true, MaxBytes: 2048})
	defer p.Close()
	if err := p.Navigate(ctx, server.URL+"/start"); err != nil {
		t.Fatal(err)
	}
	snap, _ := p.Snapshot(ctx)
	if snap.Title != "Session" || snap.URL != server.URL+"/final" {
		t.Fatal(snap)
	}
	for _, address := range []string{server.URL + "/large", server.URL + "/missing", "file:///etc/passwd"} {
		if err := p.Navigate(ctx, address); err == nil {
			t.Fatal("accepted", address)
		}
	}
}

func TestExternalScriptsAndParallelScrape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/script.js" {
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write([]byte(`document.title='Script executed'`))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<title>Initial</title><script src="/script.js"></script><p>Go</p>`))
	}))
	defer server.Close()
	results := Scrape(context.Background(), []string{server.URL, server.URL, "file:///bad"}, 2, Options{AllowPrivateNetwork: true}, "document.title")
	if len(results) != 3 || results[0].Title != "Script executed" || string(results[1].Value) != `"Script executed"` || results[2].Error == "" {
		t.Fatal(results)
	}
}

func TestInsertAdjacentHTMLPositions(t *testing.T) {
	page := New(Options{})
	defer page.Close()
	ctx := context.Background()
	if err := page.SetContent(ctx, `<div><span id="anchor">C</span></div>`); err != nil {
		t.Fatal(err)
	}
	value, err := page.Eval(ctx, `var a=document.getElementById('anchor');a.insertAdjacentHTML('beforebegin','<b>A</b>');a.insertAdjacentHTML('afterbegin','B');a.insertAdjacentHTML('beforeend','D');a.insertAdjacentHTML('afterend','<b>E</b>');document.body.textContent`)
	if err != nil || string(value) != `"ABCDE"` {
		t.Fatal(string(value), err)
	}
	if _, err = page.Eval(ctx, `a.insertAdjacentHTML('invalid','x')`); err == nil {
		t.Fatal("invalid position")
	}
}

func TestPageScriptErrorsRemainObservable(t *testing.T) {
	ctx := context.Background()
	source := `<title>Loaded</title><script>missingAPI()</script><script>document.title='Continued'</script>`
	p := New(Options{})
	defer p.Close()
	if err := p.SetContent(ctx, source); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := p.Snapshot(ctx)
	if snapshot.Title != "Continued" || len(snapshot.ScriptErrors) != 1 {
		t.Fatal(snapshot)
	}
	strict := New(Options{StrictScripts: true})
	defer strict.Close()
	if err := strict.SetContent(ctx, source); err == nil {
		t.Fatal("strict exception ignored")
	}
}

func TestCloseInterruptsActiveScript(t *testing.T) {
	page := New(Options{Timeout: time.Minute})
	ctx := context.Background()
	if err := page.SetContent(ctx, "<p>cancel</p>"); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	if err := page.vm.Set("started", func() { close(started) }); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { _, err := page.Eval(ctx, "started(); while(true){}"); finished <- err }()
	<-started
	if err := page.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-finished; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestReadOnlyInputAndDisabledClick(t *testing.T) {
	page := New(Options{})
	defer page.Close()
	ctx := context.Background()
	if err := page.SetContent(ctx, `<input id="locked" readonly value="keep"><button id="disabled" disabled onclick="document.title='wrong'">No</button>`); err != nil {
		t.Fatal(err)
	}
	if err := page.Fill(ctx, "#locked", "changed"); err == nil {
		t.Fatal("readonly input changed")
	}
	if err := page.Click(ctx, "#disabled"); err == nil {
		t.Fatal("disabled action fired")
	}
	value, err := page.Eval(ctx, `document.querySelector('#locked').value`)
	if err != nil || string(value) != `"keep"` {
		t.Fatal(string(value), err)
	}
}
