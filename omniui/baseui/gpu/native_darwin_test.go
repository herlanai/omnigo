//go:build darwin && (amd64 || arm64)

package gpu

import (
	"image/color"
	"os"
	"runtime"
	"testing"
)

func TestMetalNativePixels(t *testing.T) {
	if os.Getenv("OMNIGO_NATIVE_TEST") != "1" {
		t.Skip("explicit native Metal test")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	d, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	frame, err := d.Render(32, 32, color.NRGBA{B: 255, A: 255}, []Rect{{X: 4, Y: 4, Width: 20, Height: 20, Color: color.NRGBA{R: 255, A: 255}}, {X: 12, Y: 12, Width: 40, Height: 40, Color: color.NRGBA{G: 255, A: 128}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := frame.RGBAAt(0, 0); got != (color.RGBA{B: 255, A: 255}) {
		t.Fatal("background", got)
	}
	if got := frame.RGBAAt(5, 5); got != (color.RGBA{R: 255, A: 255}) {
		t.Fatal("rectangle", got)
	}
	if got := frame.RGBAAt(15, 15); got != (color.RGBA{R: 127, G: 128, A: 255}) {
		t.Fatal("blend", got)
	}
	if got := frame.RGBAAt(31, 31); got != (color.RGBA{G: 128, B: 127, A: 255}) {
		t.Fatal("clipped blend", got)
	}
}
