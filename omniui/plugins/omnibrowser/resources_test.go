package omnibrowser

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStylesheetLoadingAndNetworkEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/style.css" {
			w.Header().Set("Content-Type", "text/css")
			w.Write([]byte(`#x{color:blue}`))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<link rel="stylesheet" href="/style.css"><p id="x">样式</p>`))
	}))
	defer server.Close()
	p := New(Options{AllowPrivateNetwork: true})
	defer p.Close()
	ctx := context.Background()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	style, err := p.ComputedStyle(ctx, "#x")
	if err != nil || style["color"] != "blue" {
		t.Fatal(style, err)
	}
	events := p.DrainNetworkEvents()
	if len(events) != 6 {
		t.Fatalf("events = %v", events)
	}
	for i := 0; i < 2; i++ {
		base := i * 3
		if events[base].Phase != "request" || events[base+1].Phase != "response" || events[base+2].Phase != "finished" {
			t.Fatal(events)
		}
		if _, err = p.ResponseBody(events[base].ID); err != nil {
			t.Fatal(err)
		}
	}
	if events[3].Type != "Stylesheet" || !strings.HasSuffix(events[3].URL, "/style.css") {
		t.Fatal(events[3])
	}
	if len(p.DrainNetworkEvents()) != 0 {
		t.Fatal("events repeated")
	}
}

func TestImageResourcesAreOwned(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/logo.png" {
			w.Header().Set("Content-Type", "image/png")
			img := image.NewRGBA(image.Rect(0, 0, 4, 4))
			img.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
			png.Encode(w, img)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<img src="/logo.png" width="20" height="20">`))
	}))
	defer server.Close()
	p := New(Options{AllowPrivateNetwork: true})
	defer p.Close()
	ctx := context.Background()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	s, err := p.Snapshot(ctx)
	if err != nil || len(s.Images) != 1 {
		t.Fatal(err, len(s.Images))
	}
	key := server.URL + "/logo.png"
	s.Images[key][0] = 0
	again, _ := p.Snapshot(ctx)
	if again.Images[key][0] != 137 {
		t.Fatal("snapshot resource aliases page memory")
	}
}
