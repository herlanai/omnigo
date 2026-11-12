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

	ui "omnigo.dev/omnigo/omniui/baseui"
	"omnigo.dev/omnigo/studio/internal/catalog"
	"omnigo.dev/omnigo/studio/internal/demos"
)

func main() {
	out := flag.String("out", "build/baseui-interactions", "PNG output directory")
	names := flag.String("controls", "tooltip,hover-card,context-menu,input-otp,file-tree,scroll-area,dropdown-menu,combobox,command,popover,resizable,sheet,direction,aspect-ratio,skeleton", "comma-separated demo IDs")
	chartKind := flag.Int("chart-kind", 0, "chart kind: 0 bar, 1 line, 2 area")
	chartCurve := flag.Int("chart-curve", 0, "chart interpolation: 0 linear, 1 monotone, 2 after, 3 before, 4 step")
	scale := flag.Float64("scale", 1, "physical pixels per logical pixel")
	flag.Parse()
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
	for _, id := range ids {
		if !demos.Supported(id) {
			panic("unknown demo: " + id)
		}
		for _, dark := range []bool{false, true} {
			d := demos.New(id)
			s, err := ui.NewSurface(1040, 840, ui.NewTheme(dark))
			if err != nil {
				panic(err)
			}
			if err = s.Mount(d.View()); err != nil {
				panic(err)
			}
			switch id {
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
