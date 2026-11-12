// Package demos contains executable Go control examples, not browser replicas.
package demos

import (
	"fmt"
	ui "omnigo.dev/omnigo/omniui/baseui"
	"omnigo.dev/omnigo/omniui/baseui/loading"
	"strings"
	"time"
)

var IDs = append(append(advancedIDs, uiControlIDs...), []string{"tooltip", "hover-card", "context-menu", "input-otp", "file-tree", "aspect-ratio", "combobox", "command", "direction", "dropdown-menu", "popover", "resizable", "scroll-area", "sheet", "skeleton", "accordion", "alert", "alert-dialog", "badge", "button", "button-group", "calendar", "card", "carousel", "checkbox", "code-block", "collapsible", "dialog", "empty", "field", "form", "native-select", "input", "input-group", "kbd", "label", "pagination", "progress", "radio-group", "select", "separator", "slider", "spinner", "switch", "table", "tabs", "textarea", "toggle", "toggle-group", "typography", "markdown", "icon", "loading", "plugin/borderbeam", "plugin/glass", "plugin/scene3d", "plugin/game", "plugin/terminal", "plugin/omnibrowser"}...)

func Supported(id string) bool {
	for _, v := range IDs {
		if v == id {
			return true
		}
	}
	return false
}

type Demo struct {
	browser *browserDemo
	// RunTask executes work away from the UI and dispatches its returned callback.
	RunTask        func(func() func())
	NativeWeb      *ui.WebViewState
	PreviewScroll  ui.ScrollState
	markdown       *markdownDemo
	icons          *iconDemo
	uiControls     *uiControlDemo
	plugins        *pluginState
	WriteClipboard func(string) bool
	advanced       *advancedState
	OTP            *ui.TextBuffer
	Context        *ui.ContextMenuState
	Files          *ui.FileTreeState

	Scroll  *ui.ScrollState
	Menu    *ui.MenuState
	Combo   *ui.ComboboxState
	Command *ui.CommandState
	Split   *ui.SplitState

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
	return &Demo{advanced: newAdvancedState(), OTP: ui.NewTextBuffer(""), Context: &ui.ContextMenuState{}, Files: &ui.FileTreeState{Open: map[string]bool{"src": true}}, Scroll: &ui.ScrollState{}, Menu: &ui.MenuState{}, Combo: &ui.ComboboxState{}, Command: &ui.CommandState{}, Split: &ui.SplitState{Fraction: .5, MinFirst: 100, MinSecond: 100}, ID: id, Page: 1, Value: .45, Enabled: true, Choice: "design", Message: "Interact with the control to inspect its state.", Buffer: ui.NewTextBuffer(initialText(id)), Calendar: ui.NewCalendarState(ui.Date{Year: 2026, Month: time.November, Day: 11}), Select: &ui.SelectState{}, Code: &ui.CodeBlockState{}, Carousel: &ui.CarouselState{}, Table: &ui.TableState{Limit: 5}, Expanded: map[string]bool{}, Selected: map[string]bool{}, indicators: []*loading.Indicator{loading.New(loading.Ring), loading.New(loading.Dots), loading.New(loading.Bars), loading.New(loading.Clock), loading.New(loading.Ripple), loading.New(loading.Orbit), loading.New(loading.Pulse)}}
}

var options = []ui.Option{{ID: "design", Label: "Design"}, {ID: "develop", Label: "Develop"}, {ID: "ship", Label: "Ship"}, {ID: "disabled", Label: "Unavailable", Disabled: true}}

func (d *Demo) click()           { d.Count++; d.Message = fmt.Sprintf("Action received %d times", d.Count) }
func (d *Demo) choice(v string)  { d.Choice = v; d.Message = "Selected: " + v }
func (d *Demo) enabled(v bool)   { d.Enabled = v; d.Message = fmt.Sprintf("Enabled: %t", v) }
func (d *Demo) number(v float64) { d.Value = v; d.Message = fmt.Sprintf("Value: %.2f", v) }
func (d *Demo) content() *ui.Element {
	for _, id := range uiControlIDs {
		if d.ID == id {
			return d.uiControlContent()
		}
	}
	if view := d.pluginContent(); view != nil {
		return view
	}
	if content := d.advancedContent(); content != nil {
		return content
	}
	switch d.ID {
	case "web-view":
		if d.NativeWeb == nil {
			panic("web-view requires a native host")
		}
		return ui.WebView("native-web", d.NativeWeb)

	case "icon":
		return d.iconContent()
	case "markdown":
		return d.markdownContent()
	case "tooltip":
		return ui.Column("demo", ui.Button("anchor", "Hover or focus for help", d.click).Variant(ui.Outline), ui.Tooltip("tip", "anchor", "Create a workspace for your next project."))
	case "hover-card":
		return ui.Column("demo", ui.Button("anchor", "Workspace details", nil).Variant(ui.Outline), ui.HoverCard("hover", "anchor", ui.Text("hover/title", "OmniGo workspace"), ui.Text("hover/body", "Move into this card to keep it open."), ui.Button("hover/action", "Inspect workspace", d.click)))
	case "context-menu":
		return ui.ContextMenu("demo", ui.Button("anchor", "Right-click or press Shift+F10", d.click).Variant(ui.Outline), d.Context, []ui.MenuItem{{ID: "open", Label: "Open workspace", Action: func() { d.Message = "Workspace opened" }}, {ID: "pin", Label: "Pinned", Checked: &d.Enabled}, {ID: "disabled", Label: "Unavailable", Disabled: true}})
	case "input-otp":
		return ui.Column("demo", ui.Text("otp/help", "Enter six digits. Paste, select and undo are supported."), ui.InputOTP("otp", "Verification code", d.OTP, 6), ui.Button("verify", "Verify", func() { d.Message = "Entered code: " + d.OTP.String() }).Variant(ui.Outline))
	case "file-tree":
		return ui.FileTree("files", d.Files, []ui.FileNode{{ID: "src", Name: "src", Directory: true, Children: []ui.FileNode{{ID: "main", Name: "main.go", Path: "src/main.go", Change: "m"}, {ID: "ui", Name: "ui", Directory: true, Children: []ui.FileNode{{ID: "window", Name: "window.go", Path: "src/ui/window.go"}}}}}, {ID: "readme", Name: "README.md", Path: "README.md"}, {ID: "mod", Name: "go.mod", Path: "go.mod"}}, func(n ui.FileNode) { d.Message = "Open callback: " + n.Path })

	case "scroll-area":
		items := []*ui.Element{}
		for i := range 60 {
			items = append(items, ui.Button(fmt.Sprintf("row/%d", i), fmt.Sprintf("Workspace %02d", i+1), func() { d.Message = fmt.Sprintf("Selected row %d", i+1) }).Variant(ui.Ghost))
		}
		return ui.ScrollArea("demo", d.Scroll, items...).Height(300)
	case "dropdown-menu":
		return ui.DropdownMenu("demo", "Workspace actions", d.Menu, []ui.MenuItem{
			{ID: "create", Label: "New workspace", Shortcut: "Cmd N", Action: d.click},
			{ID: "line", Separator: true},
			{ID: "view", Label: "View", Children: []ui.MenuItem{{ID: "notifications", Label: "Notifications", Checked: &d.Enabled}, {ID: "reset", Label: "Reset view", Action: func() { d.Message = "View reset" }}}},
			{ID: "disabled", Label: "Unavailable", Disabled: true},
		})
	case "combobox":
		return ui.Combobox("demo", "Workspace", d.Combo, options, d.choice)
	case "command":
		return ui.Command("demo", d.Command, []ui.CommandItem{{ID: "new", Label: "New workspace", Group: "Workspace", Keywords: []string{"create"}, Action: d.click}, {ID: "search", Label: "Search projects", Group: "Workspace", Action: func() { d.Message = "Search command received" }}, {ID: "settings", Label: "Preferences", Group: "Settings", Action: func() { d.Message = "Preferences command received" }}, {ID: "disabled", Label: "Unavailable action", Group: "Settings", Disabled: true}})
	case "popover":
		return ui.Column("demo", ui.Button("open", "Workspace details", func() { d.Open = !d.Open }).Variant(ui.Outline), ui.Popover("popup", "open", d.Open, func() { d.Open = false }, ui.Text("popup/title", "Workspace settings"), ui.Input("popup/name", "Name", d.Buffer), ui.Button("popup/save", "Save", func() { d.click(); d.Open = false })))
	case "sheet":
		return ui.Column("demo", ui.Button("open", "Open settings", func() { d.Open = true }), ui.Sheet("sheet", "Settings", d.Open, ui.Right, func() { d.Open = false }, ui.Input("sheet/name", "Name", d.Buffer), ui.Switch("sheet/enabled", "Notifications", d.Enabled, d.enabled)))
	case "resizable":
		return ui.Resizable("demo", d.Split, false, ui.Card("left", ui.Text("left/title", "Projects")), ui.Card("right", ui.Text("right/title", "Drag the divider or use arrow keys."))).Width(560).Height(240)
	case "aspect-ratio":
		return ui.AspectRatio("demo", 16./9, ui.Card("content", ui.Text("content/label", "16 : 9 content"))).Width(480)
	case "direction":
		return ui.Direction("demo", ui.RightToLeft, ui.Text("direction/label", "RTL row order and keyboard navigation"), ui.RadioGroup("choices", "Workflow", d.Choice, options[:3], d.choice))
	case "skeleton":
		return ui.Column("demo", ui.Skeleton("line1", 300, 24), ui.Skeleton("line2", 220, 16), ui.Skeleton("body", 480, 160)).Gap(16)

	case "button":
		return ui.Column("demo",
			ui.Row("variants/first", ui.Button("action", "Default", d.click), ui.Button("outline", "Outline", d.click).Variant(ui.Outline), ui.Button("secondary", "Secondary", d.click).Variant(ui.Secondary)),
			ui.Row("variants/second", ui.Button("ghost", "Ghost", d.click).Variant(ui.Ghost), ui.Button("danger", "Destructive", d.click).Variant(ui.Destructive), ui.Button("link", "Link", d.click).Variant(ui.Link)),
			ui.Row("sizes", ui.Button("xs", "Button", d.click).Size(ui.ControlXS), ui.Button("small", "Button", d.click).Size(ui.ControlSmall), ui.Button("normal", "Button", d.click), ui.Button("large", "Button", d.click).Size(ui.ControlLarge), ui.Button("disabled", "Disabled", d.click).Disabled(true))).Gap(16)

	case "button-group":
		return ui.ButtonGroup("demo", "Actions", ui.Button("save", "Save", d.click), ui.Button("preview", "Preview", d.click).Variant(ui.Outline))
	case "input":
		return ui.Column("input-states", ui.Input("demo", "Email", d.Buffer).Placeholder("you@example.com"), ui.Input("input/invalid", "Invalid input", d.Buffer).Invalid(true), ui.Input("input/disabled", "Disabled input", d.Buffer).Disabled(true), ui.Input("input/readonly", "Read-only input", d.Buffer).ReadOnly(true)).Gap(16)
	case "textarea":
		return ui.Textarea("demo", "Notes", d.Buffer).Placeholder("Write a message…").Height(120)
	case "field", "form":
		return ui.Form("demo", ui.Field("name", "Project name", ui.Input("input", "Project name", d.Buffer), "A descriptive name for your project.", ""))
	case "input-group":
		return ui.InputGroup("demo", ui.Text("prefix", "https://"), ui.Input("input", "Domain", d.Buffer), ui.Button("go", "Go", d.click).Size(ui.ControlXS))
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
		return ui.Card("demo", ui.CardHeader("card/header", ui.CardTitle("title", "Create project"), ui.CardDescription("description", "Start building with OmniGo.")), ui.CardContent("card/content", ui.Input("card/name", "Project name", d.Buffer).Placeholder("Project name")), ui.CardFooter("card/footer", ui.Button("create", "Create", d.click)))
	case "badge":
		return ui.Column("demo", ui.Row("badge/first", ui.Badge("primary", "Default"), ui.Badge("outline", "Outline").Variant(ui.Outline), ui.Badge("secondary", "Secondary").Variant(ui.Secondary)), ui.Row("badge/second", ui.Badge("ghost", "Ghost").Variant(ui.Ghost), ui.Badge("danger", "Destructive").Variant(ui.Destructive), ui.Badge("link", "Link").Variant(ui.Link))).Gap(8)

	case "alert":
		return ui.Alert("demo", "Changes saved", "Your local workspace is up to date.")
	case "empty":
		return ui.Empty("demo", "A fresh start", "Create your first project to get going.", ui.Button("create", "Create project", d.click))
	case "kbd":
		return ui.Row("demo", ui.Kbd("cmd", "Cmd"), ui.Kbd("plus", "+"), ui.Kbd("key", "K"))
	case "label", "typography":
		return ui.Column("demo", ui.Heading("headline", "Make space for good ideas.", 2), ui.Text("body", "Clear hierarchy, comfortable spacing and consistent controls.").Wrap(true), ui.Text("note", "Supporting text stays quieter without losing readability.").Role("note"))
	case "separator":
		return ui.Column("demo", ui.Text("before", "Workspace"), ui.Separator("rule"), ui.Text("after", "Preferences"))
	case "code-block":
		return ui.CodeBlock("demo", "main.go", "baseui.Button(\n    \"save\", \"Save changes\",\n    func() { save() },\n)", d.Code, 3, d.copyText)
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
		rows := []*ui.Element{}
		for i, name := range []string{"Ring", "Dots", "Bars", "Clock", "Ripple", "Orbit", "Pulse"} {
			rows = append(rows, ui.Row("loading/"+name, ui.Text("loading/"+name+"/name", name).Width(100), d.indicators[i].View("loading/"+name+"/view", name, loading.Options{Paused: d.Selected["loading-paused"]})).Gap(24))
		}
		rows = append(rows, ui.Button("loading/pause", "Pause / resume", func() { d.Selected["loading-paused"] = !d.Selected["loading-paused"] }))
		return ui.Column("demo", rows...).Gap(12)
	}
	return ui.Empty("missing", "Not implemented", "This capability remains in the migration audit.")
}
func (d *Demo) View() *ui.Element {
	name := strings.ReplaceAll(strings.TrimPrefix(d.ID, "plugin/"), "-", " ")
	if len(name) > 0 {
		name = strings.ToUpper(name[:1]) + name[1:]
	}
	header := ui.Row("header", ui.Text("heading", "OmniUI").FontSize(26).FontWeight(600).Grow(1), ui.Badge("brand/status", "Go native").Variant(ui.Secondary))
	playground := ui.Card("playground", ui.CardHeader("playground/header", ui.CardTitle("preview/title", name), ui.CardDescription("caption", "Interactive preview")), ui.CardContent("playground/content", d.content()))
	return ui.Column("root", header, ui.CardDescription("intro", "Native components · appearance, states and interaction"), ui.ScrollArea("preview/scroll", &d.PreviewScroll, playground).Grow(1), ui.Text("state", d.Message).FontSize(13).Muted(true)).Padding(32).Gap(24)
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

func (d *Demo) copyText(value string) {
	if d.WriteClipboard == nil {
		d.Message = "Clipboard unavailable in this host"
		return
	}
	if d.WriteClipboard(value) {
		d.Message = "Copied to clipboard"
	} else {
		d.Message = "Clipboard write failed"
	}
}

func initialText(id string) string {
	if id == "input-group" {
		return "docs.omnigo.dev"
	}
	return "Build something thoughtful."
}
