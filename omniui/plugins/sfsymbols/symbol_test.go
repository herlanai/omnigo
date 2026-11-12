package sfsymbols

import (
	"math"
	"testing"
)

func TestInvalidOptionsDoNotEnterAppKit(t *testing.T) {
	for _, o := range []Options{{Size: -1}, {Size: math.NaN()}, {Weight: 2}} {
		if _, e := Render("gearshape", o); e == nil {
			t.Fatal("invalid options accepted")
		}
	}
	if _, e := Render("", Options{}); e == nil {
		t.Fatal("empty symbol accepted")
	}
}
