// Command gallery renders interactive Studio demos offscreen using Go controls.
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/studio/internal/catalog"
	"github.com/herlanai/omnigo/studio/internal/demos"
)

func main() {
	out := flag.String("out", "build/baseui-interactions", "PNG output directory")
	names := flag.String("controls", "tooltip,hover-card,context-menu,input-otp,file-tree,scroll-area,dropdown-menu,combobox,command,popover,resizable,sheet,direction,aspect-ratio,skeleton", "comma-separated demo IDs")
	chartKind := flag.Int("chart-kind", 0, "chart kind: 0 bar, 1 line, 2 area")
	chartCurve := flag.Int("chart-curve", 0, "chart interpolation: 0 linear, 1 monotone, 2 after, 3 before, 4 step")
	style := flag.Int("style", 0, "style: 0 Maia, 1 Nova, 2 Lyra")
	accent := flag.Int("accent", 0, "accent: 0 Neutral, 1 Blue, 2 Violet")
	boldMenus := flag.Bool("bold-menus", false, "use accent for selected menu rows")
	beamPreset := flag.Int("beam-preset", 0, "Borderbeam preset: 0 Medium, 1 Small, 2 Line, 3 Pulse Inner, 4 Pulse Outside")
	exclude := flag.String("exclude", "", "comma-separated demo IDs to omit")
	scale := flag.Float64("scale", 1, "physical pixels per logical pixel")
	flag.Parse()
	if *beamPreset < 0 || *beamPreset > 4 {
		panic("invalid beam preset")
	}
	if *style < 0 || *style > 2 || *accent < 0 || *accent > 2 {
		panic("invalid style or accent")
	}
	if *chartKind < 0 || *chartKind > 2 || *chartCurve < 0 || *chartCurve > 4 {
		panic("invalid chart kind or curve")
	}
	if err := os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	ids := strings.Split(*names, ",")
	if *names == "all" {
		ids = nil
		for _, entry := range catalog.Entries() {
			if entry.Demo != "" && demos.Supported(entry.Demo) {
				ids = append(ids, entry.Demo)
			}
		}
	}
	filtered := ids[:0]
	for _, id := range ids {
		skip := false
		for _, excluded := range strings.Split(*exclude, ",") {
			if id == excluded {
				skip = true
			}
		}
		if !skip {
			filtered = append(filtered, id)
		}
	}
	ids = filtered
	for _, id := range ids {
		if !demos.Supported(id) {
			panic("unknown demo: " + id)
		}
		for _, dark := range []bool{false, true} {
			d := demos.New(id)
			d.Style = ui.VisualStyle(*style)
			d.Palette = ui.AccentPalette(*accent)
			d.BoldMenus = *boldMenus
			width, height := 1040, 840
			if id == "plugin/borderbeam" {
				width, height = 960, 820
			}
			s, err := ui.NewSurface(width, height, ui.ThemePreset(dark, d.Style, d.Palette, d.BoldMenus))
			if err != nil {
				panic(err)
			}
			if err = s.Mount(d.View()); err != nil {
				panic(err)
			}
			switch id {
			case "date-picker":
				s.Focus("demo/date/trigger")
				err = s.Key(ui.KeyEvent{Key: "Enter"})
			case "data-table":
				s.Focus("demo/table/select/a")
				err = s.Key(ui.KeyEvent{Key: "Enter"})
			case "chart":
				for range *chartKind {
					s.Focus("chart-kind")
					if err = s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
						panic(err)
					}
				}
				for range *chartCurve {
					s.Focus("chart-curve")
					if err = s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
						panic(err)
					}
				}
			case "model-selector":
				s.Focus("demo/model/trigger")
				err = s.Key(ui.KeyEvent{Key: "Down"})

			case "tooltip", "hover-card":
				s.Focus("anchor")
			case "context-menu":
				s.Focus("anchor")
				err = s.Key(ui.KeyEvent{Key: "F10", Shift: true})
			case "context":
				s.Focus("demo/anchor")
			case "inline-citation":
				s.Focus("demo/anchor")
			case "menubar":
				s.Focus("demo/file/trigger")
				err = s.Key(ui.KeyEvent{Key: "Down"})
			case "input-otp":
				s.Focus("otp")
				err = s.Paste("123456")
			case "file-tree":
				s.Focus("files/node/main")

			case "dropdown-menu":
				s.Focus("demo/trigger")
				err = s.Key(ui.KeyEvent{Key: "Enter"})
			case "popover", "sheet", "drawer", "dialog", "alert-dialog":
				s.Focus("open")
				err = s.Key(ui.KeyEvent{Key: "Enter"})
			case "combobox":
				s.Focus("demo/input")
				err = s.TextInput("de")
			case "plugin/borderbeam":
				s.Focus(fmt.Sprintf("beam/preset/%d", *beamPreset))
				err = s.Key(ui.KeyEvent{Key: "Enter"})
			case "command":
				s.Focus("demo/query")
				err = s.TextInput("workspace")
			case "scroll-area":
				err = s.ScrollTo("demo", 0, 220)
			}
			if err != nil {
				panic(err)
			}
			if err = s.Mount(d.View()); err != nil {
				panic(err)
			}
			if id == "plugin/borderbeam" {
				if _, err := s.Render(); err != nil {
					panic(err)
				}
			}
			s.Advance(500 * time.Millisecond)
			img, err := s.RenderScaled(*scale, nil)
			if err != nil {
				panic(err)
			}
			suffix := "light"
			if dark {
				suffix = "dark"
			}
			path := filepath.Join(*out, strings.ReplaceAll(id, "/", "-")+"-"+suffix+".png")
			f, err := os.Create(path)
			if err != nil {
				panic(err)
			}
			if err = png.Encode(f, img); err != nil {
				panic(err)
			}
			if err = f.Close(); err != nil {
				panic(err)
			}
			d.Close()
			if err = s.Close(); err != nil {
				panic(err)
			}
			fmt.Println(path)
		}
	}
	if *names == "all" {
		if err := writeReviewSheets(*out, ids); err != nil {
			panic(err)
		}
	}
}
