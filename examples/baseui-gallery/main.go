// The gallery is rendered entirely in Go. It writes offscreen PNGs; no browser
// or native window is required. Studio also has an experimental macOS host.
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/omniui/baseui/loading"
)

type records struct{}

func (records) Len() int         { return 4 }
func (records) Key(i int) string { return fmt.Sprint(i) }
func (records) Cell(i int, column string) string {
	if column == "name" {
		return []string{"Controls", "Keyboard", "Rendering", "Extensions"}[i]
	}
	return []string{"Go", "Go", "CPU", "Opt-in"}[i]
}

func view() *baseui.Element {
	name := baseui.NewTextBuffer("OmniGo workspace")
	comment := baseui.NewTextBuffer("Go-owned controls.\nOne theme, one input model.")
	calendar := baseui.NewCalendarState(baseui.Date{Year: 2026, Month: time.November, Day: 11})
	code := &baseui.CodeBlockState{}
	forms := baseui.Card("forms",
		baseui.Text("forms/title", "01 / INPUT & ACTIONS"),
		baseui.Field("name-field", "Workspace", baseui.Input("name", "Workspace", name), "Your application, written in Go.", ""),
		baseui.Row("buttons", baseui.Button("save", "Save changes", nil), baseui.Button("cancel", "Cancel", nil).Variant(baseui.Outline)),
		baseui.Button("disabled", "Unavailable action", nil).Disabled(true).Variant(baseui.Secondary),
		baseui.Separator("form-separator"),
		baseui.Checkbox("check", "Enable notifications", true, nil),
		baseui.MixedCheckbox("mixed", "Some items selected", baseui.Mixed, nil),
		baseui.Switch("switch", "Sync workspace", true, nil),
		baseui.RadioGroup("radios", "Density", "normal", []baseui.Option{{ID: "normal", Label: "Normal"}, {ID: "compact", Label: "Compact"}}, nil),
		baseui.Slider("volume", "Volume", .65, 0, 1, .05, nil),
		baseui.Textarea("notes", "Notes", comment),
		baseui.Row("badges", baseui.Badge("status", "GO NATIVE"), baseui.Badge("version", "BASEUI / 0.1").Variant(baseui.Secondary)),
	).Width(300)
	navigation := baseui.Column("navigation",
		baseui.Card("tabs-card", baseui.Text("tabs/title", "02 / NAVIGATION"), baseui.Tabs("tabs", "Views", "overview", []baseui.Option{{ID: "overview", Label: "Overview"}, {ID: "activity", Label: "Activity"}, {ID: "settings", Label: "Settings"}}, nil), baseui.Text("tabs/description", "A shared keyboard and focus model."), baseui.Collapsible("details", "Control details", true, nil, baseui.Text("details/body", "Stable IDs preserve focus.\nDisabled controls reject input."))),
		baseui.Calendar("calendar", calendar, baseui.CalendarOptions{WeekStart: time.Monday}),
		baseui.Card("loading-card", baseui.Text("loading/title", "GO-RENDERED INDICATORS"), baseui.Row("loading/row", loading.New(loading.Ring).View("ring", "Loading", loading.Options{}), loading.New(loading.Dots).View("dots", "Loading", loading.Options{}), loading.New(loading.Bars).View("bars", "Loading", loading.Options{})).Gap(24)),
	).Width(300).Gap(16)
	data := baseui.Column("data",
		baseui.Card("table-card", baseui.Text("table/title", "03 / DATA & FEEDBACK"), baseui.Table("table", &baseui.TableState{Limit: 4}, []baseui.TableColumn{{ID: "name", Title: "Capability", Width: 150}, {ID: "runtime", Title: "Runtime", Width: 100}}, records{}, nil), baseui.Progress("progress", "Migration progress example", .42), baseui.Text("progress/label", "42% — illustrative progress value")),
		baseui.CodeBlock("code", "app.go", "baseui.Button(\n  \"save\", \"Save\",\n  func() { save() },\n)", code, 8, nil),
		baseui.Alert("notice", "Native foundation", "Layout, input and drawing in Go.\nmacOS host: experimental."),
	).Width(300).Gap(16)
	return baseui.Column("root", baseui.Row("header", baseui.Text("brand", "OMNIGO  /  OMNIUI  /  BASEUI").Grow(1), baseui.Badge("header/badge", "PURE GO").Variant(baseui.Secondary)), baseui.Text("subtitle", "Unified controls • Go behavior • Shared theme • CPU rendering"), baseui.Separator("rule"), baseui.Row("columns", forms, navigation, data).Gap(20)).Padding(24).Gap(16)
}

func main() {
	out := flag.String("out", "build/baseui-gallery", "PNG output directory")
	scale := flag.Float64("scale", 1, "physical pixels per logical pixel")
	flag.Parse()
	if err := os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	for _, dark := range []bool{false, true} {
		s, err := baseui.NewSurface(1000, 860, baseui.NewTheme(dark))
		if err != nil {
			panic(err)
		}
		if err := s.Mount(view()); err != nil {
			panic(err)
		}
		s.Focus("name")
		s.Advance(250 * time.Millisecond)
		img, err := s.RenderScaled(*scale, nil)
		if err != nil {
			panic(err)
		}
		name := "light.png"
		if dark {
			name = "dark.png"
		}
		path := filepath.Join(*out, name)
		f, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		if err := png.Encode(f, img); err != nil {
			panic(err)
		}
		if err := f.Close(); err != nil {
			panic(err)
		}
		s.Close()
		fmt.Println(path)
	}
}
