package demos

import (
	"fmt"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/omniui/plugins/borderbeam"
	dshgo "github.com/herlanai/omnigo/omniui/plugins/dsh-go"
	"github.com/herlanai/omnigo/omniui/plugins/game"
	"github.com/herlanai/omnigo/omniui/plugins/glass"
	"github.com/herlanai/omnigo/omniui/plugins/scene3d"
	"github.com/herlanai/omnigo/omniui/plugins/terminal"
	"image/color"
	"math"
	"strings"
	"time"
)

type pluginState struct {
	panel                             *dshgo.Panel
	terminal                          *terminal.Screen
	terminalCounter                   int
	beam                              borderbeam.State
	beamSamples                       [4]borderbeam.State
	beamInitialized                   bool
	beamHidden                        bool
	beamSpeed, beamStrength, beamGlow float64
	preset                            borderbeam.Preset
	paused, opaque                    bool
	loop                              game.Loop
	position                          float64
	angle                             float64
}

func (d *Demo) pluginContent() *ui.Element {
	if !strings.HasPrefix(d.ID, "plugin/") {
		return nil
	}
	if d.plugins == nil {
		d.plugins = &pluginState{}
	}
	s := d.plugins
	switch d.ID {
	case "plugin/omnibrowser":
		return d.browserContent()
	case "plugin/terminal":
		if s.terminal == nil {
			s.terminal, _ = terminal.NewScreen(64, 14)
			s.terminal.Feed([]byte("OmniGo VT viewport\r\n\x1b[32mREADY\x1b[0m  ANSI colors and cursor control\r\n\x1b[1mBold text\x1b[0m and \x1b[38;2;50;150;230mtrue color\x1b[0m\r\n\r\nKeyboard input is echoed below.\r\n> "))
		}
		return ui.Column("plugin", ui.Text("terminal/info", "VT preview · input is echoed; no shell is launched").Wrap(true), ui.Row("terminal/actions", ui.Button("terminal/append", "Append output", func() {
			s.terminalCounter++
			s.terminal.Feed([]byte(fmt.Sprintf("\r\n\x1b[36mOutput %d\x1b[0m\r\n> ", s.terminalCounter)))
		}).Variant(ui.Outline), ui.Button("terminal/clear", "Clear screen", func() { s.terminal.Feed([]byte("\x1b[2J\x1b[H> ")) }).Variant(ui.Outline)), ui.ScrollArea("terminal/scroll", d.Scroll, terminal.View("terminal/view", s.terminal, terminal.ViewOptions{OnInput: func(data []byte) {
			if string(data) == "\r" {
				data = []byte("\r\n> ")
			}
			s.terminal.Feed(data)
		}})).HorizontalScroll(true).Height(280))

	case "plugin/borderbeam":
		return d.borderBeamContent()
	case "plugin/glass":
		return ui.Column("plugin", ui.Switch("opaque", "Reduce transparency", s.opaque, func(v bool) { s.opaque = v }), ui.Column("backdrop", glass.Box("glass", glass.Options{Refraction: 5, Opaque: s.opaque}, ui.Text("glass/title", "Backdrop blur and refraction"), ui.Button("glass/action", "Try interaction", d.click).Variant(ui.Outline))).Padding(24).PaintSurface(func(p *ui.Painter, r ui.Rect) {
			for i := 0; i < 12; i++ {
				p.Fill(ui.Rect{X: r.X + float64(i)*r.W/12, Y: r.Y, W: r.W / 12, H: r.H}, color.NRGBA{uint8(40 + i*14), uint8(140 - i*7), 180, 255}, 0)
			}
		}))
	case "plugin/scene3d":
		return ui.Column("plugin", ui.Button("rotate", "Rotate scene", func() { s.angle += math.Pi / 8 }).Variant(ui.Outline), ui.Canvas("scene", "Depth-tested 3D triangles", func(p *ui.Painter, r ui.Rect) {
			rotate := func(v scene3d.Vec3) scene3d.Vec3 {
				return scene3d.Vec3{X: v.X*math.Cos(s.angle) + v.Z*math.Sin(s.angle), Y: v.Y, Z: -v.X*math.Sin(s.angle) + v.Z*math.Cos(s.angle)}
			}
			triangles := []scene3d.Triangle{{A: rotate(scene3d.Vec3{X: -1, Y: -.8}), B: rotate(scene3d.Vec3{X: 1, Y: -.8}), C: rotate(scene3d.Vec3{Y: 1}), Color: p.Theme().Accent}, {A: rotate(scene3d.Vec3{X: -.7, Y: -.6, Z: .5}), B: rotate(scene3d.Vec3{X: .7, Y: -.6, Z: .5}), C: rotate(scene3d.Vec3{Y: .6, Z: .5}), Color: color.NRGBA{60, 170, 220, 255}}}
			img, _, err := scene3d.Render(scene3d.Scene{Triangles: triangles, Background: p.Theme().Surface}, scene3d.DefaultCamera(), max(1, min(960, int(r.W))), max(1, min(960, int(r.H))))
			if err == nil {
				p.Image(r, img, 12)
			}
		}).Height(280))
	case "plugin/game":
		return ui.Column("plugin", ui.Button("pause", "Pause / resume", func() { s.paused = !s.paused }).Variant(ui.Outline), ui.Canvas("game", "Fixed-step moving ball", func(p *ui.Painter, r ui.Rect) {
			s.loop.Advance(p.Now(), s.paused || p.Theme().ReduceMotion, func(dt time.Duration) { s.position = math.Mod(s.position+dt.Seconds()*100, 10000) })
			p.Fill(r, p.Theme().Surface, 12)
			x := r.X + 16 + math.Mod(s.position, max(1, r.W-32))
			p.Fill(ui.Rect{X: x - 8, Y: r.Y + r.H/2 - 8, W: 16, H: 16}, p.Theme().Accent, 8)
		}).Height(160).Animate(!s.paused))
	}
	return nil
}
