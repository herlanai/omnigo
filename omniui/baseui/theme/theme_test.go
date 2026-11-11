package theme

import (
	"math"
	"strconv"
	"testing"
)

func luminance(hex string) float64 {
	var v [3]float64
	for i := range v {
		n, _ := strconv.ParseUint(hex[1+i*2:3+i*2], 16, 8)
		x := float64(n) / 255
		if x <= .04045 {
			v[i] = x / 12.92
		} else {
			v[i] = math.Pow((x+.055)/1.055, 2.4)
		}
	}
	return .2126*v[0] + .7152*v[1] + .0722*v[2]
}
func TestTextContrastAndIsolation(t *testing.T) {
	tokens := Default()
	for _, p := range []Palette{tokens.Light, tokens.Dark} {
		for _, pair := range [][2]string{{"foreground", "background"}, {"foreground", "surface"}, {"mutedForeground", "background"}, {"primaryForeground", "primary"}, {"danger", "surface"}} {
			a, b := luminance(p[pair[0]]), luminance(p[pair[1]])
			ratio := (math.Max(a, b) + .05) / (math.Min(a, b) + .05)
			if ratio < 4.5 {
				t.Errorf("%v contrast %.2f", pair, ratio)
			}
		}
	}
	tokens.Light["background"] = "bad"
	if Default().Light["background"] == "bad" {
		t.Fatal("shared mutable palette")
	}
	if tokens.ControlHeight < 24 || tokens.TouchHeight < 44 {
		t.Fatal("invalid control geometry")
	}
}
