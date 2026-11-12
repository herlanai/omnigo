package render

import (
	"bytes"
	"image/color"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser"
	"testing"
	"time"
)

func TestGeneratedPseudoBoxesAndContent(t *testing.T) {
	source := `<style>body{margin:0}p{margin:0;position:relative;width:100px;height:60px}p::before{content:"";display:block;position:absolute;left:10px;top:5px;width:20px;height:20px;background:red}p::after{content:attr(data-label) "\4e2d";color:blue;position:absolute;left:40px;top:5px}</style><p data-label="A"></p>`
	img, e := Screenshot(omnibrowser.Snapshot{HTML: source}, 160, 100, false)
	if e != nil {
		t.Fatal(e)
	}
	if got := img.RGBAAt(15, 10); got != (color.RGBA{255, 0, 0, 255}) {
		t.Fatal("before box", got)
	}
	ink := 0
	for y := 5; y < 40; y++ {
		for x := 40; x < 90; x++ {
			c := img.RGBAAt(x, y)
			if c.B > 150 && c.R < 100 {
				ink++
			}
		}
	}
	if ink < 20 {
		t.Fatal("generated attr and escaped Chinese text absent", ink)
	}
	again, _ := Screenshot(omnibrowser.Snapshot{HTML: source}, 160, 100, false)
	if !bytes.Equal(img.Pix, again.Pix) {
		t.Fatal("pseudo generation must be deterministic")
	}
}
func TestSVGTextOutlinesAndPresentationCascade(t *testing.T) {
	source := `<style>body{margin:0}:root{--ink:#0000ff}svg{width:200px;height:100px}.label{fill:var(--ink);font-size:24px;letter-spacing:2px}</style><svg viewBox="0 0 200 100"><text class="label" x="100" y="40" text-anchor="middle">Go中文</text><rect x="0" y="60" width="200" height="20" fill="red"/></svg>`
	img, e := Screenshot(omnibrowser.Snapshot{HTML: source}, 240, 140, false)
	if e != nil {
		t.Fatal(e)
	}
	left, right, ink := 200, 0, 0
	for y := 0; y < 50; y++ {
		for x := 0; x < 200; x++ {
			c := img.RGBAAt(x, y)
			if c.B > 150 && c.R < 100 {
				left = min(left, x)
				right = max(right, x)
				ink++
			}
		}
	}
	if ink < 100 || left < 40 || right > 160 || left > 80 || right < 120 {
		t.Fatal("anchored Latin/CJK outlines", left, right, ink)
	}
	if c := img.RGBAAt(100, 70); c.R != 255 || c.G != 0 {
		t.Fatal("presentation attribute lost", c)
	}
}
func TestExplicitAnimationTimeAndSVGTransform(t *testing.T) {
	source := `<style>body{margin:0}@keyframes travel{from{transform:translateX(0px)}to{transform:translateX(100px)}}div{width:20px;height:20px;background:red;animation:travel 2s linear forwards}@keyframes orbit{0%{transform:rotate(0)}100%{transform:rotate(360deg)}}.rotor{animation:orbit 4s linear infinite}</style><div></div><svg width="160" height="120" viewBox="-60 -60 120 120"><g class="rotor"><circle cx="40" cy="0" r="6" fill="blue"/></g></svg>`
	snap := omnibrowser.Snapshot{HTML: source}
	a, e := ScreenshotAt(snap, 240, 180, false, 0)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := ScreenshotAt(snap, 240, 180, false, time.Second)
	if a.RGBAAt(5, 5).G != 0 || b.RGBAAt(55, 5).G != 0 || b.RGBAAt(5, 5).G != 255 {
		t.Fatal("linear animation sample")
	}
	if bytes.Equal(a.Pix[240*4*20:], b.Pix[240*4*20:]) {
		t.Fatal("SVG group transform did not animate")
	}
	end, _ := ScreenshotAt(snap, 240, 180, false, 3*time.Second)
	if end.RGBAAt(105, 5).G != 0 {
		t.Fatal("forwards fill")
	}
}
func TestAnimationImportantAndPause(t *testing.T) {
	snap := omnibrowser.Snapshot{HTML: `<style>body{margin:0}@keyframes a{to{transform:translateX(100px)}}div{width:20px;height:20px;background:red;animation:a 2s linear infinite;transform:translateX(30px)!important}</style><div></div>`}
	img, e := ScreenshotAt(snap, 160, 100, false, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	if img.RGBAAt(35, 5).G != 0 || img.RGBAAt(55, 5).G != 255 {
		t.Fatal("animation beat important")
	}
}

func TestPausedAndNegativeDelayAnimation(t *testing.T) {
	for _, tc := range []struct {
		timing string
		time   time.Duration
		x      int
	}{{"2s linear infinite paused", time.Second, 0}, {"2s -1s linear infinite", 0, 50}, {"2s linear infinite alternate", 3 * time.Second, 50}} {
		snap := omnibrowser.Snapshot{HTML: `<style>body{margin:0}@keyframes a{from{transform:translateX(0px)}to{transform:translateX(100px)}}div{width:20px;height:20px;background:red;animation:a ` + tc.timing + `}</style><div></div>`}
		img, e := ScreenshotAt(snap, 160, 100, false, tc.time)
		if e != nil {
			t.Fatal(e)
		}
		if img.RGBAAt(tc.x+5, 5).G != 0 {
			t.Fatal(tc, "wrong sample position")
		}
	}
}
