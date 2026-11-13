package baseui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Option struct {
	ID, Label string
	Disabled  bool
}

func validateOptions(options []Option) {
	seen := map[string]bool{}
	for _, o := range options {
		if o.ID == "" || seen[o.ID] {
			panic("baseui: option IDs must be unique and nonempty")
		}
		seen[o.ID] = true
	}
}

func Field(id, label string, control *Element, description, errorText string) *Element {
	control.Label(label).Description(description).Invalid(errorText != "")
	children := []*Element{Label(id+"/label", label), control}
	if description != "" {
		children = append(children, Text(id+"/description", description).Role("note"))
	}
	if errorText != "" {
		errorLabel := Text(id+"/error", errorText)
		errorLabel.role = "alert"
		children = append(children, errorLabel)
	}
	e := Column(id, children...).Gap(8)
	e.role = "group"
	e.label = label
	return e
}
func Form(id string, children ...*Element) *Element {
	e := Column(id, children...).Gap(24)
	e.role = "form"
	return e
}
func ButtonGroup(id, label string, children ...*Element) *Element {
	e := Row(id, children...).Gap(4)
	e.role = "group"
	e.label = label
	return e
}
func InputGroup(id string, prefix, input, suffix *Element) *Element {
	children := []*Element{}
	if prefix != nil {
		children = append(children, prefix)
	}
	if input == nil {
		panic("baseui: nil grouped input")
	}
	clone := *input
	clone.grow = 1
	clone.groupedInput = true
	if clone.kind == InputKind && clone.height == 0 {
		clone.height = 34
	}
	children = append(children, &clone)
	if suffix != nil {
		children = append(children, suffix)
	}
	e := Row(id, children...).Gap(6).Role("group")
	e.inputGroup = true
	e.centerItems = true
	e.paddingX = 12
	e.minHeight = 36
	e.invalid = clone.invalid
	return e
}

func nextOption(options []Option, current int, delta int) int {
	for range len(options) {
		current = (current + delta + len(options)) % len(options)
		if !options[current].Disabled {
			return current
		}
	}
	return -1
}
func selectionRow(id, label, value string, options []Option, change func(string), radio bool) *Element {
	validateOptions(options)
	children := []*Element{}
	for i, o := range options {
		i, o := i, o
		activate := func() {
			if change != nil {
				change(o.ID)
			}
		}
		var child *Element
		if radio {
			child = Radio(id+"/"+o.ID, o.Label, o.ID == value, activate)
		} else {
			child = Button(id+"/"+o.ID, o.Label, activate).Variant(Ghost)
			child.selected = o.ID == value
			child.role = "tab"
			child.Height(30).FontSize(14).FontWeight(500)
			child.controlPadding = 8
		}
		child.focusGroup = id
		child.Disabled(o.Disabled).OnKey(func(ev KeyEvent) bool {
			index := -1
			if ev.RTL {
				if ev.Key == "Right" {
					ev.Key = "Left"
				} else if ev.Key == "Left" {
					ev.Key = "Right"
				}
			}
			switch ev.Key {
			case "Left", "Up":
				index = nextOption(options, i, -1)
			case "Right", "Down":
				index = nextOption(options, i, 1)
			case "Home":
				index = nextOption(options, len(options)-1, 1)
			case "End":
				index = nextOption(options, 0, -1)
			default:
				return false
			}
			if index >= 0 {
				if change != nil {
					change(options[index].ID)
				}
				if ev.RequestFocus != nil {
					ev.RequestFocus(id + "/" + options[index].ID)
				}
			}
			return true
		})
		children = append(children, child)
	}
	e := Row(id, children...)
	e.label = label
	e.role = "tablist"
	if radio {
		e.role = "radiogroup"
	} else {
		e.Padding(3).Gap(4)
	}
	return e
}
func Tabs(id, label, value string, options []Option, change func(string)) *Element {
	return selectionRow(id, label, value, options, change, false)
}
func RadioGroup(id, label, value string, options []Option, change func(string)) *Element {
	return selectionRow(id, label, value, options, change, true)
}
func ToggleGroup(id, label string, selected map[string]bool, options []Option, change func(string, bool)) *Element {
	validateOptions(options)
	children := []*Element{}
	for _, o := range options {
		o := o
		children = append(children, Toggle(id+"/"+o.ID, o.Label, selected[o.ID], func(v bool) {
			if change != nil {
				change(o.ID, v)
			}
		}).Disabled(o.Disabled))
	}
	return ButtonGroup(id, label, children...)
}

func Collapsible(id, title string, open bool, change func(bool), children ...*Element) *Element {
	marker := "+ "
	if open {
		marker = "- "
	}
	trigger := Button(id+"/trigger", marker+title, func() {
		if change != nil {
			change(!open)
		}
	}).Variant(Ghost)
	trigger.alignStart = true
	trigger.expanded = open
	trigger.controls = id + "/content"
	trigger.description = "expands " + id + "/content"
	body := Column(id+"/content", children...).Padding(8).Hidden(!open)
	return Column(id, trigger, body).Gap(4)
}

type AccordionItem struct {
	ID, Title string
	Content   *Element
	Disabled  bool
}

func Accordion(id string, items []AccordionItem, open map[string]bool, multiple bool, change func(map[string]bool)) *Element {
	children := []*Element{}
	for _, item := range items {
		item := item
		children = append(children, Collapsible(id+"/"+item.ID, item.Title, open[item.ID], func(next bool) {
			values := map[string]bool{}
			if multiple {
				for k, v := range open {
					values[k] = v
				}
			}
			values[item.ID] = next
			if change != nil {
				change(values)
			}
		}, item.Content).Disabled(item.Disabled))
	}
	return Card(id, children...).Gap(4)
}

func Dialog(id, title string, open bool, close func(), children ...*Element) *Element {
	header := Row(id+"/header", Heading(id+"/title", title, 3).Grow(1), Button(id+"/close", "Close", close).Variant(Ghost))
	e := Card(id, append([]*Element{header}, children...)...).Padding(24).Gap(16).Width(440).Hidden(!open)
	e.kind = ModalKind
	e.role = "dialog"
	e.label = title
	e.activate = close
	e.dismissOutside = true
	return e
}
func AlertDialog(id, title, message string, open bool, cancel, confirm func()) *Element {
	e := Dialog(id, title, open, cancel, Text(id+"/message", message), Row(id+"/actions", Button(id+"/cancel", "Cancel", cancel).Variant(Outline), Button(id+"/confirm", "Confirm", confirm)))
	e.role = "alertdialog"
	e.dismissOutside = false
	e.autoFocus = id + "/cancel"
	return e
}
func Alert(id, title, message string, destructive ...bool) *Element {
	heading, body := Label(id+"/title", title), paragraph(id+"/message", message).FontSize(14)
	if len(destructive) > 0 && destructive[0] {
		heading.Role("alert")
		body.Role("alert")
	}
	e := Card(id, heading, body).Padding(16).Gap(4)
	e.role = "alert"
	e.label = title
	return e
}
func Empty(id, title, description string, actions ...*Element) *Element {
	children := []*Element{Heading(id+"/title", title, 3), Text(id+"/description", description).Role("note").Wrap(true)}
	children = append(children, actions...)
	return Card(id, children...)
}
func Kbd(id, text string) *Element { return Badge(id, text).Variant(Secondary).CornerRadius(5) }

func Pagination(id string, page, count int, change func(int)) *Element {
	if count < 1 || page < 1 || page > count {
		panic("baseui: invalid pagination bounds")
	}
	set := func(n int) func() {
		return func() {
			if change != nil {
				change(n)
			}
		}
	}
	children := []*Element{Button(id+"/previous", "Previous", set(page-1)).Disabled(page == 1).Variant(Outline)}
	start := max(1, min(page-2, count-4))
	for i := start; i <= min(count, start+4); i++ {
		button := Button(id+"/page/"+strconv.Itoa(i), strconv.Itoa(i), set(i)).Variant(Ghost)
		button.selected = i == page
		children = append(children, button)
	}
	children = append(children, Button(id+"/next", "Next", set(page+1)).Disabled(page == count).Variant(Outline))
	e := Row(id, children...).Gap(4)
	e.role = "navigation"
	e.label = "Pagination"
	return e
}
func Stepper(id, label string, value, min, max, step float64, change func(float64)) *Element {
	if _, err := Snap(value, min, max, step); err != nil {
		panic(err)
	}
	update := func(v float64) func() {
		return func() {
			next, err := Snap(v, min, max, step)
			if err == nil && change != nil {
				change(next)
			}
		}
	}
	return Field(id, label, Row(id+"/control", Button(id+"/decrease", "-", update(value-step)).Disabled(value <= min).Variant(Outline), Text(id+"/value", strconv.FormatFloat(value, 'f', -1, 64)).Grow(1), Button(id+"/increase", "+", update(value+step)).Disabled(value >= max).Variant(Outline)), "", "")
}

// CodeBlock preserves source bytes, blank lines and tabs in CopyText. Syntax
// shaping, selection and incremental highlighting are not yet provided.
type CodeBlockState struct {
	Expanded bool
	Scroll   ScrollState
	// Wrap uses the shared rich text layout, preserving indentation and copy bytes.
	Wrap bool
	// Language enables the shared editor lexer (currently Go; others plain).
	Language string
}

func CodeBlock(id, title, source string, state *CodeBlockState, maxLines int, copyText func(string)) *Element {
	if state == nil {
		panic("baseui: nil code block state")
	}
	lines := strings.Split(source, "\n")
	shown := source
	if maxLines > 0 && !state.Expanded && len(lines) > maxLines {
		shown = strings.Join(lines[:maxLines], "\n") + "\n..."
	}
	controls := []*Element{Text(id+"/title", title).FontSize(13).FontWeight(600).Grow(1)}
	if copyText != nil {
		controls = append(controls, Button(id+"/copy", "Copy", func() { copyText(source) }).Variant(Ghost).Size(ControlXS))
	}
	if maxLines > 0 && len(lines) > maxLines {
		label := "Expand"
		if state.Expanded {
			label = "Collapse"
		}
		controls = append(controls, Button(id+"/expand", label, func() { state.Expanded = !state.Expanded }).Variant(Ghost))
	}
	if state.Wrap || state.Language != "" {
		var spans []TextSpan
		rows := codeLines(expandCodeTabs(shown), state.Language)
		for i, row := range rows {
			if i > 0 {
				spans = append(spans, TextSpan{Text: "\n", Monospace: true})
			}
			for _, run := range row {
				spans = append(spans, TextSpan{Text: run.text, Monospace: true, codeStyle: run.style})
			}
		}
		body := RichText(id+"/source", spans, RichTextOptions{Size: 12, LineHeight: 1.4, PreserveWhitespace: true, NoWrap: !state.Wrap})
		if !state.Wrap {
			body = ScrollArea(id+"/viewport", &state.Scroll, body).HorizontalScroll(true).Height(min(320, float64(len(rows))*16.8+10))
		}
		return Card(id, Row(id+"/header", controls...), body)
	}
	body := codeText(id+"/source", shown, false)
	viewport := ScrollArea(id+"/viewport", &state.Scroll, body).HorizontalScroll(true).Height(min(320, body.height+10))
	return Card(id, Row(id+"/header", controls...), viewport)
}

type CarouselState struct{ Index int }

func Carousel(id, label string, state *CarouselState, slides []*Element) *Element {
	if state == nil || len(slides) == 0 || state.Index < 0 || state.Index >= len(slides) {
		panic("baseui: invalid carousel state")
	}
	e := Card(id, slides[state.Index], Row(id+"/controls", Button(id+"/previous", "Previous", func() { state.Index-- }).Disabled(state.Index == 0).Variant(Outline), Text(id+"/position", fmt.Sprintf("%d / %d", state.Index+1, len(slides))).Grow(1), Button(id+"/next", "Next", func() { state.Index++ }).Disabled(state.Index == len(slides)-1).Variant(Outline)))
	e.role = "group"
	e.label = label
	return e
}

func Meter(id, label string, value, min, max float64) *Element {
	if !finite(value) || !finite(min) || !finite(max) || max <= min {
		panic("baseui: invalid meter range")
	}
	e := Progress(id, label, math.Max(0, math.Min(1, (value-min)/(max-min))))
	e.role = "meter"
	return e
}

func inputTextInset(e *Element) float64 {
	if e.codeRows != nil {
		return 52
	}
	if e.groupedInput {
		return 0
	}
	return 12
}

// Card sections share the reference's 24 DIP outer inset and explicit hierarchy.
func CardHeader(id string, children ...*Element) *Element { return Column(id, children...).Gap(8) }
func CardTitle(id, title string) *Element                 { return Text(id, title).FontSize(16).FontWeight(500) }
func CardDescription(id, text string) *Element {
	return Text(id, text).FontSize(14).Muted(true).Wrap(true)
}
func CardContent(id string, children ...*Element) *Element { return Column(id, children...).Gap(16) }
func CardFooter(id string, children ...*Element) *Element  { return Row(id, children...).Gap(8) }
