package render

import (
	"bytes"
	"fmt"
	"image/color"
	browser "github.com/herlanai/omnigo/omniui/plugins/omnibrowser"
	"strconv"
	"strings"
	"testing"
)

func TestCSSPixelsAndPDFOffsets(t *testing.T) {
	s := browser.Snapshot{HTML: `<style>body{margin:0}div{width:80px;height:40px;background:red}#b{background:blue!important}</style><div id="b" style="background:green"></div>`}
	img, err := Screenshot(s, 128, 128, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.RGBAAt(20, 20); got != (color.RGBA{0, 0, 255, 255}) {
		t.Fatalf("cascade paint = %v", got)
	}
	pdf, err := PDF(s, 128, 128)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) || !bytes.Contains(pdf, []byte("/MediaBox [0 0 96.00 96.00]")) {
		t.Fatal("invalid PDF header/page")
	}
	tail := strings.Split(string(pdf), "startxref\n")
	offset, err := strconv.Atoi(strings.Split(tail[len(tail)-1], "\n")[0])
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(pdf[offset:]), "\n")
	if lines[0] != "xref" {
		t.Fatal("wrong xref offset")
	}
	for i := 1; i <= 5; i++ {
		n, err := strconv.Atoi(lines[i+2][:10])
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(pdf[n:], []byte(fmt.Sprintf("%d 0 obj", i))) {
			t.Fatal("wrong object offset", i)
		}
	}
	if _, err = PDF(s, 5000, 5000); err == nil {
		t.Fatal("unbounded PDF")
	}
}

func TestPositionedElementsDoNotConsumeFlexWidth(t *testing.T) {
	s := browser.Snapshot{HTML: `<style>body{margin:0}.row{display:flex;position:relative;min-height:100vh}.overlay{position:fixed;left:0;top:0;width:30px;height:30px;background:red}.main{flex:1;height:70px;background:blue}</style><div class="row"><div class="overlay"></div><div class="main"></div></div>`}
	img, err := Screenshot(s, 200, 200, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.RGBAAt(150, 40); got != (color.RGBA{0, 0, 255, 255}) {
		t.Fatal("fixed child consumed flex width", got)
	}
	if got := img.RGBAAt(10, 10); got != (color.RGBA{255, 0, 0, 255}) {
		t.Fatal("fixed positioning", got)
	}
}
func TestSVGAndRoundedBorder(t *testing.T) {
	s := browser.Snapshot{HTML: `<div style="width:100px;height:60px;border:2px solid red;border-radius:12px;background:white"></div><svg width="20" height="20" viewBox="0 0 20 20"><rect width="20" height="20" fill="blue"/></svg>`}
	img, err := Screenshot(s, 128, 128, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.RGBAAt(15, 75); got.B < 200 || got.R > 30 {
		t.Fatal("SVG not painted", got)
	}
	if got := img.RGBAAt(50, 8); got.R < 200 || got.G > 30 {
		t.Fatal("border missing", got)
	}
	if got := img.RGBAAt(8, 8); got != (color.RGBA{255, 255, 255, 255}) {
		t.Fatal("corner not rounded", got)
	}
}

func TestHiddenInputsInlineFlowAndOverflow(t *testing.T) {
	hidden := browser.Snapshot{HTML: `<input type="hidden" value="should-never-paint" style="display:block;background:red;width:100px;height:100px">`}
	img, err := Screenshot(hidden, 128, 128, false)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			if img.RGBAAt(x, y) != (color.RGBA{255, 255, 255, 255}) {
				t.Fatal("hidden input rendered")
			}
		}
	}
	s := browser.Snapshot{HTML: `<div style="font-size:16px"><span style="display:inline-block;width:40px;height:20px;background:red"></span><span style="display:inline-block;width:40px;height:20px;background:blue"></span></div><div style="width:40px;height:20px;overflow:hidden"><div style="width:100px;height:50px;background:green"></div></div>`}
	img, err = Screenshot(s, 128, 128, false)
	if err != nil {
		t.Fatal(err)
	}
	if img.RGBAAt(20, 15).R != 255 || img.RGBAAt(60, 15).B != 255 {
		t.Fatal("inline boxes did not share line")
	}
	if img.RGBAAt(60, 35) != (color.RGBA{255, 255, 255, 255}) {
		t.Fatal("overflow leaked")
	}
}
func TestPercentHeightUsesParentAndContentBox(t *testing.T) {
	s := browser.Snapshot{HTML: `<div style="height:100px"><div style="height:50%;background:blue"></div></div><div style="width:40px;height:20px;padding:10px;border:2px solid red;background:green"></div>`}
	img, err := Screenshot(s, 200, 200, false)
	if err != nil {
		t.Fatal(err)
	}
	if img.RGBAAt(20, 40).B != 255 || img.RGBAAt(20, 70) != (color.RGBA{255, 255, 255, 255}) {
		t.Fatal("percentage height used viewport")
	}
	if img.RGBAAt(60, 140).G == 255 || img.RGBAAt(60, 140).R == 255 {
		t.Fatal("padding missing from content-box")
	}
}
