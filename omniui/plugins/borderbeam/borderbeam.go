// Package borderbeam preserves the original OmniGo import path.
// The optional native implementation lives alongside the framework controls.
package borderbeam

import impl "github.com/herlanai/omnigo/omniui/baseui/borderbeam"

type Preset = impl.Preset
type Palette = impl.Palette
type Options = impl.Options
type State = impl.State

const (
	Medium       = impl.Medium
	Small        = impl.Small
	Line         = impl.Line
	PulseInner   = impl.PulseInner
	PulseOutside = impl.PulseOutside
	Theme        = impl.Theme
	Colorful     = impl.Colorful
	Ocean        = impl.Ocean
	Sunset       = impl.Sunset
	Mono         = impl.Mono
)

var Duration = impl.Duration
