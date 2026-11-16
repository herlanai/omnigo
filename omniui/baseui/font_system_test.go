package baseui

import (
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"testing"
)

func TestInstalledPingFangUI(t *testing.T) {
	if !UseSystemUIFont() {
		t.Skip("PingFang not installed")
	}
	defer preferSystemUI.Store(false)
	for weight, want := range map[int]string{400: "PingFangSC-Regular", 500: "PingFangSC-Medium", 600: "PingFangSC-Semibold", 700: "PingFangSC-Semibold"} {
		f := interfaceFont(weight)
		name, _ := f.Name(nil, sfnt.NameIDPostScript)
		if name != want {
			t.Fatalf("weight %d: got %s", weight, name)
		}
		if weight == 400 && richFont(0) != f {
			t.Fatal("default and styled UI use different fonts")
		}
		for _, size := range []float64{12, 24, 48} {
			face := weightedFace(weight, false, size)
			for _, r := range "OmniTeams协作空间林悦，。" {
				b, mask, _, advance, ok := face.Glyph(fixed.P(0, 64), r)
				if !ok || b.Empty() || mask == nil || advance <= 0 {
					t.Fatalf("missing glyph %c at %v/%d", r, size, weight)
				}
			}
			face.Close()
		}
	}
}
