// Command control-showcase renders a composed, interactive Go control surface.
package main

import (
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	ui "github.com/herlanai/omnigo/omniui/baseui"
)

type dashboard struct {
	amount   float64
	notes    *ui.TextBuffer
	currency *ui.SelectState
	chart    *ui.ChartState
	alerts   bool
	saved    bool
}

func newDashboard() *dashboard {
	return &dashboard{2500, ui.NewTextBuffer(""), &ui.SelectState{Value: "usd"}, &ui.ChartState{Selected: -1}, true, false}
}
func title(id, text, description string) *ui.Element {
	return ui.CardHeader(id, ui.CardTitle(id+"/title", text), ui.CardDescription(id+"/description", description))
}
func metric(id, label, value, detail string) *ui.Element {
	return ui.Card(id, ui.Text(id+"/label", label).FontSize(11).Muted(true), ui.Text(id+"/value", value).FontSize(20).FontWeight(600), ui.Text(id+"/detail", detail).FontSize(12).Muted(true)).Variant(ui.Secondary).Padding(16).Gap(6).Grow(1)
}
func goal(id, label, amount, achieved string, value float64) *ui.Element {
	return ui.Card(id, ui.Text(id+"/label", label).FontSize(11).Muted(true), ui.Text(id+"/amount", amount).FontSize(32).FontWeight(600), ui.Progress(id+"/progress", label, value).Height(8), ui.Row(id+"/detail", ui.Text(id+"/percent", fmt.Sprintf("%.0f%% achieved", value*100)).FontSize(12).Muted(true).Grow(1), ui.Text(id+"/achieved", achieved).FontSize(12))).Variant(ui.Secondary).Padding(20).Gap(16)
}
func (d *dashboard) view(dark bool) *ui.Element {
	bar := color.NRGBA{R: 100, G: 100, B: 108, A: 255}
	if dark {
		bar = color.NRGBA{R: 176, G: 176, B: 188, A: 255}
	}
	history := ui.Card("history", title("history/header", "Contribution history", "A little progress, every month."),
		ui.Row("history/total", ui.Text("history/amount", "$12,840").FontSize(34).FontWeight(600).Grow(1), ui.Badge("history/growth", "+18.6%").Variant(ui.Secondary)),
		ui.Chart("history/chart", []string{"Jun", "Jul", "Aug", "Sep", "Oct", "Nov"}, []ui.ChartSeries{{Label: "Contributions", Values: []float64{1300, 1900, 1600, 2300, 1800, 2700}, Color: bar}}, d.chart, ui.ChartOptions{Height: 190, HideGrid: true, HideLegend: true}),
		ui.Row("history/stats", metric("upcoming", "NEXT CONTRIBUTION", "Nov 25", "$1,000 scheduled"), metric("plan", "SAVINGS PLAN", "Accelerated", "Recurring weekly")).Gap(12),
		ui.Button("report", "View full report", func() { d.chart.Selected = 5 }).Height(40)).Grow(1)
	saveLabel := "Save preferences"
	if d.saved {
		saveLabel = "Preferences saved"
	}
	payout := ui.Card("payout", title("payout/header", "Payout preferences", "Your balance. Your schedule."),
		ui.Field("currency/field", "Preferred currency", ui.Select("currency", "Preferred currency", d.currency, []ui.Option{{ID: "usd", Label: "USD — US Dollar"}, {ID: "eur", Label: "EUR — Euro"}, {ID: "gbp", Label: "GBP — British Pound"}}), "", ""),
		ui.Column("amount/group", ui.Row("amount/heading", ui.Label("amount/label", "Minimum payout").Grow(1), ui.Text("amount/value", fmt.Sprintf("$%.0f", d.amount)).FontSize(24).FontWeight(600)), ui.Slider("amount", "Minimum payout", d.amount, 50, 10000, 50, func(v float64) { d.amount = v; d.saved = false }), ui.Row("amount/limits", ui.Text("amount/min", "$50 minimum").FontSize(12).Muted(true).Grow(1), ui.Text("amount/max", "$10,000 maximum").FontSize(12).Muted(true))).Gap(10),
		ui.Field("notes/field", "Notes", ui.Textarea("notes", "Notes", d.notes).Placeholder("Add a note for your next payout…").Height(96), "Only visible to you.", ""),
		ui.Button("save", saveLabel, func() { d.saved = true }).Height(40)).Grow(1)
	goals := ui.Card("goals", title("goals/header", "Savings targets", "Make room for what comes next."), goal("retirement", "RETIREMENT", "$420,000", "$273,000", .65), goal("home", "YOUR NEXT HOME", "$85,000", "$27,200", .32), ui.Button("goal", "Create a new goal", func() { d.notes.Set("New savings goal") }).Variant(ui.Outline).Height(40)).Grow(1)
	return ui.Column("dashboard",
		ui.Row("masthead", ui.Text("brand", "OMNI / PERSONAL FINANCE").FontSize(11).FontWeight(600).Grow(1), ui.Badge("workspace", "Personal workspace").Variant(ui.Outline)),
		ui.Row("heading", ui.Column("heading/text", ui.Heading("heading/title", "Good things take shape.", 1).FontSize(32), ui.CardDescription("heading/description", "A clearer view of your money, and the moments you are saving for.")).Gap(8).Grow(1), ui.Button("overview", "Overview", nil).Variant(ui.Secondary), ui.Button("export", "Export report", nil).Variant(ui.Outline)).Gap(12),
		ui.Row("cards", history, payout, goals).Gap(20),
		ui.Row("bottom", ui.Card("balance", ui.Text("balance/label", "AVAILABLE BALANCE").FontSize(11).Muted(true), ui.Text("balance/value", "$8,219.42").FontSize(28).FontWeight(600), ui.CardDescription("balance/detail", "Ready when you are.")).Gap(8).Grow(1),
			ui.Card("notifications", ui.CardTitle("notifications/title", "Stay in the loop"), ui.Switch("alerts", "Transaction notifications", d.alerts, func(v bool) { d.alerts = v }), ui.Text("notifications/detail", "Updates that matter, without the noise.").FontSize(12).Muted(true)).Gap(12).Grow(1),
			ui.Card("security", ui.CardTitle("security/title", "Your account, protected"), ui.Text("security/detail", "Two-step verification is enabled.").FontSize(13).Muted(true), ui.Button("security/manage", "Manage security", nil).Variant(ui.Outline).Size(ui.ControlSmall)).Gap(12).Grow(1)).Gap(20),
		ui.Text("footer", "OMNIGO    /    Thoughtful defaults. Native Go controls.").FontSize(11).Muted(true),
	).Padding(32).Gap(24)
}
func main() {
	out := flag.String("out", "build/control-showcase", "PNG output directory")
	style := flag.Int("style", 0, "style: 0 Maia, 1 Nova, 2 Lyra")
	accent := flag.Int("accent", 0, "accent: 0 Neutral, 1 Blue, 2 Violet")
	flag.Parse()
	if *style < 0 || *style > 2 || *accent < 0 || *accent > 2 {
		panic("invalid style/accent")
	}
	if err := os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	for _, dark := range []bool{false, true} {
		d := newDashboard()
		s, err := ui.NewSurface(1320, 1000, ui.ThemePreset(dark, ui.VisualStyle(*style), ui.AccentPalette(*accent), false))
		if err != nil {
			panic(err)
		}
		name := "light"
		if dark {
			name = "dark"
		}
		if err = s.Mount(d.view(dark)); err != nil {
			panic(err)
		}
		for _, state := range []string{"default", "keyboard"} {
			if state == "keyboard" {
				s.Focus("amount")
				if err = s.Key(ui.KeyEvent{Key: "Right"}); err != nil {
					panic(err)
				}
				if err = s.Mount(d.view(dark)); err != nil {
					panic(err)
				}
			}
			img, err := s.RenderScaled(2, nil)
			if err != nil {
				panic(err)
			}
			path := filepath.Join(*out, name+"-"+state+".png")
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
			fmt.Println(path)
		}
		s.Close()
	}
}
