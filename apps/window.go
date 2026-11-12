package apps

import (
	"fmt"
	"math"
)

type TitleBarStyle string

const (
	TitleBarDefault     TitleBarStyle = "default"
	TitleBarHidden      TitleBarStyle = "hidden"
	TitleBarHiddenInset TitleBarStyle = "hiddenInset"
)

// Point uses logical pixels from the window's top-left corner.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// WindowOptions describes window creation. Native support is driver-specific;
// apps/host consumes these options for its public macOS Web UI shell.
type WindowOptions struct {
	Title, URL                         string
	Width, Height, MinWidth, MinHeight int
	TitleBarStyle                      TitleBarStyle
	TrafficLightPosition               *Point
}

func (o WindowOptions) Validate() error {
	if o.TitleBarStyle != "" && o.TitleBarStyle != TitleBarDefault && o.TitleBarStyle != TitleBarHidden && o.TitleBarStyle != TitleBarHiddenInset {
		return fmt.Errorf("apps: invalid titlebar style")
	}
	if o.Width < 0 || o.Height < 0 || o.MinWidth < 0 || o.MinHeight < 0 {
		return fmt.Errorf("apps: negative window dimensions")
	}
	if o.Width > 0 && o.MinWidth > o.Width || o.Height > 0 && o.MinHeight > o.Height {
		return fmt.Errorf("apps: minimum exceeds initial size")
	}
	if p := o.TrafficLightPosition; p != nil {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) || p.X < 0 || p.Y < 0 || p.X > 256 || p.Y > 128 {
			return fmt.Errorf("apps: invalid traffic light position")
		}
	}
	return nil
}
