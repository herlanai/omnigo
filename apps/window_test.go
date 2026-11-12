package apps

import (
	"math"
	"testing"
)

func TestWindowTitlebarOptions(t *testing.T) {
	for _, style := range []TitleBarStyle{"", TitleBarDefault, TitleBarHidden, TitleBarHiddenInset} {
		o := WindowOptions{Title: "My App", URL: "/", Width: 1200, Height: 820, MinWidth: 860, MinHeight: 620, TitleBarStyle: style, TrafficLightPosition: &Point{X: 12, Y: 11}}
		if err := o.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, o := range []WindowOptions{{TitleBarStyle: "unknown"}, {Width: 800, MinWidth: 900}, {TrafficLightPosition: &Point{X: math.NaN()}}, {TrafficLightPosition: &Point{Y: -1}}} {
		if o.Validate() == nil {
			t.Fatal("invalid window options accepted", o)
		}
	}
}
