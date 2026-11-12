//go:build darwin && (amd64 || arm64)

package host

import (
	"fmt"

	"github.com/ebitengine/purego/objc"
	"github.com/herlanai/omnigo/omniwindow"
)

type windowChrome struct {
	window          objc.ID
	style           omniwindow.TitleBarStyle
	position        *omniwindow.Point
	originalButtons [3]rect
}

func newWindowChrome(window objc.ID) *windowChrome {
	c := &windowChrome{window: window, style: omniwindow.TitleBarDefault}
	for i := range c.originalButtons {
		c.originalButtons[i] = objc.Send[rect](window.Send(sel("standardWindowButton:"), uint64(i)), sel("frame"))
	}
	return c
}
func (c *windowChrome) apply(request TitleBarRequest) (TitleBarState, error) {
	if request.Style == "" {
		return c.layout(), nil
	}
	if err := (omniwindow.WindowOptions{TitleBarStyle: request.Style, TrafficLightPosition: request.TrafficLightPosition}).Validate(); err != nil {
		return TitleBarState{}, err
	}
	if c.window.Send(sel("styleMask"))&(1<<14) != 0 {
		return TitleBarState{}, fmt.Errorf("leave fullscreen before switching titlebar")
	}
	c.style = request.Style
	c.position = nil
	if request.TrafficLightPosition != nil {
		p := *request.TrafficLightPosition
		c.position = &p
	} else if c.style == omniwindow.TitleBarHiddenInset {
		c.position = &omniwindow.Point{X: 12, Y: 11}
	}
	old := objc.Send[rect](c.window, sel("frame"))
	hidden := c.style != omniwindow.TitleBarDefault
	mask := uint64(c.window.Send(sel("styleMask")))
	if hidden {
		mask |= 1 << 15
	} else {
		mask &^= 1 << 15
	}
	c.window.Send(sel("setStyleMask:"), mask)
	visibility := int64(0)
	if hidden {
		visibility = 1
	}
	c.window.Send(sel("setTitleVisibility:"), visibility)
	c.window.Send(sel("setTitlebarAppearsTransparent:"), hidden)
	if c.style == omniwindow.TitleBarHiddenInset {
		toolbar := objc.ID(objc.GetClass("NSToolbar")).Send(sel("alloc")).Send(sel("initWithIdentifier:"), stringID("omnigo.hidden-inset"))
		toolbar.Send(sel("setShowsBaselineSeparator:"), false)
		c.window.Send(sel("setToolbar:"), toolbar)
		toolbar.Send(sel("release"))
	} else {
		c.window.Send(sel("setToolbar:"), objc.ID(0))
	}
	c.window.Send(sel("setFrame:display:"), old, true)
	for i := range c.originalButtons {
		b := c.window.Send(sel("standardWindowButton:"), uint64(i))
		b.Send(sel("setHidden:"), false)
		if !hidden {
			b.Send(sel("setFrame:"), c.originalButtons[i])
		}
	}
	return c.layout(), nil
}
func (c *windowChrome) layout() TitleBarState {
	result := TitleBarState{Style: c.style}
	if c.style == omniwindow.TitleBarDefault || c.window.Send(sel("styleMask"))&(1<<14) != 0 {
		return result
	}
	button := c.window.Send(sel("standardWindowButton:"), uint64(0))
	if button == 0 {
		return result
	}
	if c.position == nil {
		result.TopInset = 32
		return result
	}
	parent := button.Send(sel("superview"))
	container := parent.Send(sel("superview"))
	if container == 0 {
		return result
	}
	b := objc.Send[rect](button, sel("frame"))
	result.TopInset = b.S.H + 2*c.position.Y
	containerFrame := objc.Send[rect](container, sel("frame"))
	windowFrame := objc.Send[rect](c.window, sel("frame"))
	containerFrame.P.Y = windowFrame.S.H - result.TopInset
	containerFrame.S.H = result.TopInset
	container.Send(sel("setFrame:"), containerFrame)
	// Preserve native horizontal spacing instead of assuming a particular OS metric.
	for i, original := range c.originalButtons {
		control := c.window.Send(sel("standardWindowButton:"), uint64(i))
		offset := original.P.X - c.originalButtons[0].P.X
		control.Send(sel("setFrameOrigin:"), point{X: c.position.X + offset, Y: c.position.Y})
	}
	return result
}
