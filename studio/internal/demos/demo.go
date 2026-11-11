// Package demos contains executable Go control examples, not browser replicas.
package demos

import (
	"fmt"
	ui "omnigo.dev/omnigo/omniui/baseui"
	"omnigo.dev/omnigo/omniui/baseui/loading"
	"time"
)

var IDs = []string{"accordion", "alert", "alert-dialog", "badge", "button", "button-group", "calendar", "card", "carousel", "checkbox", "code-block", "collapsible", "dialog", "empty", "field", "form", "native-select", "input", "input-group", "kbd", "label", "pagination", "progress", "radio-group", "select", "separator", "slider", "spinner", "switch", "table", "tabs", "textarea", "toggle", "toggle-group", "typography", "loading"}

func Supported(id string) bool {
	for _, v := range IDs {
		if v == id {
			return true
		}
	}
	return false
}

type Demo struct {
	ID                 string
	Count, Page        int
	Value              float64
	Enabled, Open      bool
	Choice, Message    string
	Buffer             *ui.TextBuffer
	Calendar           *ui.CalendarState
	Select             *ui.SelectState
	Code               *ui.CodeBlockState
	Carousel           *ui.CarouselState
	Table              *ui.TableState
	Expanded, Selected map[string]bool
	indicators         []*loading.Indicator
}

func New(id string) *Demo {
	return &Demo{ID: id, Page: 1, Value: .45, Enabled: true, Choice: "design", Message: "Interact with the control to inspect its state.", Buffer: ui.NewTextBuffer("Build something thoughtful."), Calendar: ui.NewCalendarState(ui.Date{Year: 2026, Month: time.November, Day: 11}), Select: &ui.SelectState{}, Code: &ui.CodeBlockState{}, Carousel: &ui.CarouselState{}, Table: &ui.TableState{Limit: 5}, Expanded: map[string]bool{}, Selected: map[string]bool{}, indicators: []*loading.Indicator{loading.New(loading.Ring), loading.New(loading.Dots), loading.New(loading.Bars)}}
}

var options = []ui.Option{{ID: "design", Label: "Design"}, {ID: "develop", Label: "Develop"}, {ID: "ship", Label: "Ship"}, {ID: "disabled", Label: "Unavailable", Disabled: true}}

func (d *Demo) click()           { d.Count++; d.Message = fmt.Sprintf("Action received %d times", d.Count) }
func (d *Demo) choice(v string)  { d.Choice = v; d.Message = "Selected: " + v }
func (d *Demo) enabled(v bool)   { d.Enabled = v; d.Message = fmt.Sprintf("Enabled: %t", v) }
func (d *Demo) number(v float64) { d.Value = v; d.Message = fmt.Sprintf("Value: %.2f", v) }
func (d *Demo) content() *ui.Element {
	switch d.ID {
	case "button":
		return ui.Column("demo", ui.Button("action", "Create workspace", d.click), ui.Button("secondary", "Secondary", d.click).Variant(ui.Secondary), ui.Button("outline", "Outline", d.click).Variant(ui.Outline), ui.Button("disabled", "Unavailable", d.click).Disabled(true))
	case "button-group":
		return ui.ButtonGroup("demo", "Actions", ui.Button("save", "Save", d.click), ui.Button("preview", "Preview", d.click).Variant(ui.Outline))
	case "input":
		return ui.Input("demo", "Workspace name", d.Buffer)
	case "textarea":
		return ui.Textarea("demo", "Notes", d.Buffer)
	case "field", "form":
		return ui.Form("demo", ui.Field("name", "Project name", ui.Input("input", "Project name", d.Buffer), "A descriptive name for your project.", ""))
	case "input-group":
		return ui.InputGroup("demo", ui.Text("prefix", "https://"), ui.Input("input", "Domain", d.Buffer), ui.Button("go", "Go", d.click))
	case "checkbox":
		return ui.Column("demo", ui.Checkbox("check", "Enable notifications", d.Enabled, d.enabled), ui.MixedCheckbox("mixed", "Partially selected", ui.Mixed, nil))
	case "switch":
		return ui.Switch("demo", "Automatic updates", d.Enabled, d.enabled)
	case "toggle":
		return ui.Toggle("demo", "Pin workspace", d.Enabled, d.enabled)
	case "toggle-group":
		return ui.ToggleGroup("demo", "Tools", d.Selected, options, func(v string, b bool) { d.Selected[v] = b; d.Message = fmt.Sprintf("%s: %t", v, b) })
	case "radio-group":
		return ui.RadioGroup("demo", "Workflow", d.Choice, options, d.choice)
	case "tabs":
		return ui.Column("demo", ui.Tabs("tabs", "Workflow", d.Choice, options, d.choice), ui.Card("panel", ui.Text("panel/text", "Current panel: "+d.Choice)))
	case "slider":
		return ui.Slider("demo", "Volume", d.Value, 0, 1, .05, d.number)
	case "progress":
		return ui.Column("demo", ui.Progress("progress", "Completion", d.Value), ui.Slider("adjust", "Adjust progress", d.Value, 0, 1, .05, d.number))
	case "select", "native-select":
		return ui.Select("demo", "Workspace", d.Select, options)
	case "calendar":
		return ui.Calendar("demo", d.Calendar, ui.CalendarOptions{WeekStart: time.Monday})
	case "pagination":
		return ui.Pagination("demo", d.Page, 12, func(v int) { d.Page = v; d.Message = fmt.Sprintf("Page: %d", v) })
	case "accordion":
		return ui.Accordion("demo", []ui.AccordionItem{{ID: "one", Title: "One coherent system", Content: ui.Text("one/body", "Shared tokens, layout and event handling.")}, {ID: "two", Title: "Written in Go", Content: ui.Text("two/body", "The native host forwards input to baseui.")}}, d.Expanded, true, func(v map[string]bool) { d.Expanded = v })
	case "collapsible":
		return ui.Collapsible("demo", "Implementation details", d.Open, func(v bool) { d.Open = v }, ui.Text("details", "This content participates in Go layout."))
	case "dialog", "alert-dialog":
		close := func() { d.Open = false }
		var modal *ui.Element
		if d.ID == "dialog" {
			modal = ui.Dialog("modal", "Create workspace", d.Open, close, ui.Input("modal/name", "Name", d.Buffer), ui.Button("modal/create", "Create", func() { d.click(); close() }))
		} else {
			modal = ui.AlertDialog("modal", "Discard changes?", "This demonstration only resets local state.", d.Open, close, func() { d.Message = "Confirmed"; close() })
		}
		return ui.Column("demo", ui.Button("open", "Open dialog", func() { d.Open = true }), modal)
	case "card":
		return ui.Card("demo", ui.Badge("tag", "WORKSPACE"), ui.Text("title", "Your next idea starts here."), ui.Text("description", "A consistent foundation for desktop interfaces."), ui.Button("create", "Create project", d.click))
	case "badge":
		return ui.Row("demo", ui.Badge("primary", "Active"), ui.Badge("secondary", "Draft").Variant(ui.Secondary), ui.Badge("outline", "Preview").Variant(ui.Outline))
	case "alert":
		return ui.Alert("demo", "Changes saved", "Your local workspace is up to date.")
	case "empty":
		return ui.Empty("demo", "A fresh start", "Create your first project to get going.", ui.Button("create", "Create project", d.click))
	case "kbd":
		return ui.Row("demo", ui.Kbd("cmd", "Cmd"), ui.Kbd("plus", "+"), ui.Kbd("key", "K"))
	case "label", "typography":
		return ui.Column("demo", ui.Text("headline", "Make space for good ideas."), ui.Text("body", "Typography follows the shared Go theme."))
	case "separator":
		return ui.Column("demo", ui.Text("before", "Workspace"), ui.Separator("rule"), ui.Text("after", "Preferences"))
	case "code-block":
		return ui.CodeBlock("demo", "main.go", "baseui.Button(\n    \"save\", \"Save changes\",\n    func() { save() },\n)", d.Code, 3, func(v string) { d.Message = "Copy callback received (clipboard not connected)" })
	case "carousel":
		return ui.Carousel("demo", "Projects", d.Carousel, []*ui.Element{ui.Card("slide1", ui.Text("slide1/title", "01 / Design")), ui.Card("slide2", ui.Text("slide2/title", "02 / Develop")), ui.Card("slide3", ui.Text("slide3/title", "03 / Ship"))})
	case "table":
		return ui.Column("demo", ui.Table("table", d.Table, []ui.TableColumn{{ID: "name", Title: "Project", Width: 220}, {ID: "status", Title: "Status", Width: 160}}, rows{}, nil), ui.Pagination("page", d.Table.Offset/5+1, 20, func(v int) {
			d.Table.Offset = (v - 1) * 5
			d.Message = fmt.Sprintf("Visible rows %d–%d of 100", (v-1)*5+1, v*5)
		}))
	case "spinner":
		return ui.Spinner("demo", "Working")
	case "loading":
		return ui.Row("demo", d.indicators[0].View("ring", "Ring", loading.Options{}), d.indicators[1].View("dots", "Dots", loading.Options{}), d.indicators[2].View("bars", "Bars", loading.Options{})).Gap(32)
	}
	return ui.Empty("missing", "Not implemented", "This capability remains in the migration audit.")
}
func (d *Demo) View() *ui.Element {
	return ui.Column("root", ui.Row("brand", ui.Text("brand/name", "OMNI STUDIO").Grow(1), ui.Badge("brand/status", "GO / BASEUI")), ui.Separator("top/rule"), ui.Text("heading", d.ID), ui.Text("caption", "Native CPU rendering · Tab to focus · Space / Enter to activate"), ui.Card("playground", d.content()).Padding(24), ui.Separator("bottom/rule"), ui.Text("state", d.Message), ui.Text("limit", "Preview implementation. See Studio for remaining parity requirements.")).Padding(28).Gap(20)
}

type rows struct{}

func (rows) Len() int         { return 100 }
func (rows) Key(i int) string { return fmt.Sprint(i) }
func (rows) Cell(i int, c string) string {
	if c == "name" {
		return fmt.Sprintf("Workspace %03d", i+1)
	}
	return "Active"
}
