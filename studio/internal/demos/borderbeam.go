package demos

import (
	"fmt"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/omniui/baseui/borderbeam"
	"image/color"
	"time"
)

func (d *Demo) borderBeamContent() *ui.Element {
	s := d.plugins
	if !s.beamInitialized {
		s.beamInitialized = true
		s.beamSpeed = 1.96
		s.beamStrength = 1
		s.beamGlow = 28
	}
	names := []string{"Medium", "Small", "Line", "Pulse Inner", "Pulse Outside"}
	status := "Playing · " + names[s.preset]
	if s.paused {
		status = "Paused · clear Pause to continue"
	}
	if s.beamHidden {
		status = "Effect hidden · clear Hide effect to show"
	}
	presets := make([]*ui.Element, 0, 5)
	for i, name := range names {
		presets = append(presets, ui.Toggle(fmt.Sprintf("beam/preset/%d", i), name, int(s.preset) == i, func(bool) { s.preset = borderbeam.Preset(i); s.beamSpeed = borderbeam.Duration(s.preset).Seconds() }).Size(ui.ControlSmall))
	}
	slider := func(id, label string, value, low, high float64, change func(float64)) *ui.Element {
		return ui.Column(id, ui.Text(id+"/label", label), ui.Slider(id+"/slider", label, value, low, high, .01, change)).Gap(8).Grow(1)
	}
	o := borderbeam.Options{Preset: s.preset, Duration: time.Duration(s.beamSpeed * float64(time.Second)), Strength: s.beamStrength, Glow: s.beamGlow, Paused: s.paused, Disabled: s.beamHidden}
	action := "Pause animation"
	if s.paused {
		action = "Resume animation"
	}
	hero := s.beam.Box("beam", o, ui.Text("beam/title", "Give your ideas an edge.").FontSize(24).FontWeight(700), ui.Text("beam/description", "Default colors follow the Studio theme. Explore both light and dark appearances.").Wrap(true), ui.Button("beam/action", action, func() { s.paused = !s.paused }).Variant(ui.Outline).Width(160)).Padding(28).Gap(16)
	samples := []struct {
		name, description string
		palette           borderbeam.Palette
		accent            color.NRGBA
		colors            []color.NRGBA
	}{
		{"Prism", "Red, blue, green and violet intertwine in a local light field.", borderbeam.Colorful, color.NRGBA{167, 139, 250, 255}, nil},
		{"Ocean", "Cyan and deep blue fade together for tools and workspaces.", borderbeam.Ocean, color.NRGBA{56, 189, 248, 255}, nil},
		{"Sunset", "Coral, warm orange and pink-violet, soft and bright.", borderbeam.Sunset, color.NRGBA{251, 146, 60, 255}, nil},
		{"Aurora", "Custom mint, ice blue and violet light spots.", borderbeam.Theme, color.NRGBA{52, 211, 153, 255}, []color.NRGBA{{52, 211, 153, 255}, {103, 232, 249, 255}, {167, 139, 250, 255}, {34, 211, 238, 255}}},
	}
	cards := make([]*ui.Element, 4)
	for i, sample := range samples {
		option := o
		option.Palette = sample.palette
		option.Tint = sample.accent
		option.Colors = sample.colors
		id := fmt.Sprintf("beam/sample/%d", i)
		dots := ui.Canvas(id+"/dots", "Palette", func(p *ui.Painter, r ui.Rect) {
			for j := 0; j < 3; j++ {
				c := sample.accent
				c.A = uint8(255 - j*50)
				p.Fill(ui.Rect{X: r.X + float64(j)*13, Y: r.Y, W: 7, H: 7}, c, 4)
			}
		}).Height(7)
		cards[i] = s.beamSamples[i].Box(id, option, dots, ui.Text(id+"/name", sample.name).FontSize(19).FontWeight(700), ui.Text(id+"/description", sample.description).Muted(true).Wrap(true), ui.Text(id+"/caption", "NATIVE COLOR FIELD").FontSize(10).Muted(true)).Padding(24).Gap(12).Grow(1)
	}
	return ui.Column("beam/page", d.windowHeader(ui.Text("heading", "Border Beam").FontSize(28).FontWeight(700)), ui.Text("beam/subtitle", "Five native animations · color fields and masks · synchronized controls").Wrap(true), ui.Text("beam/status", status).FontSize(13).Muted(true),
		ui.Row("beam/toggles", ui.Checkbox("pause", "Pause", s.paused, func(v bool) { s.paused = v }), ui.Checkbox("beam/hide", "Hide effect", s.beamHidden, func(v bool) { s.beamHidden = v })).Gap(16), ui.Row("beam/presets", presets...).Gap(8),
		ui.Row("beam/sliders", slider("beam/speed", "Duration (seconds)", s.beamSpeed, 1, 8, func(v float64) { s.beamSpeed = v }), slider("beam/strength", "Strength", s.beamStrength, .1, 1, func(v float64) { s.beamStrength = v }), slider("beam/glow", "Inner glow", s.beamGlow, 8, 48, func(v float64) { s.beamGlow = v })).Gap(20), hero,
		ui.Text("beam/studies", "COLOR STUDIES").FontSize(12).Muted(true), ui.Column("beam/samples", ui.Row("beam/samples/first", cards[0], cards[1]).Gap(16), ui.Row("beam/samples/second", cards[2], cards[3]).Gap(16)).Gap(16),
		ui.Text("beam/footer", "Reduced motion freezes the effect; invisible controls do not request animation frames.").Wrap(true)).Gap(20)
}
