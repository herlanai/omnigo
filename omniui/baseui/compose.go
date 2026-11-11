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
	children := []*Element{Text(id+"/label", label), control}
	if description != "" {
		children = append(children, Text(id+"/description", description))
	}
	if errorText != "" {
		errorLabel := Text(id+"/error", errorText)
		errorLabel.role = "alert"
		children = append(children, errorLabel)
	}
	e := Column(id, children...)
	e.role = "group"
	e.label = label
	return e
}
func Form(id string, children ...*Element) *Element {
	e := Column(id, children...)
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
	children = append(children, input.Grow(1))
	if suffix != nil {
		children = append(children, suffix)
	}
	return Row(id, children...).Gap(4)
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
		}
		child.Disabled(o.Disabled).OnKey(func(ev KeyEvent) bool {
			index := -1
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
	trigger.selected = open
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
	header := Row(id+"/header", Text(id+"/title", title).Grow(1), Button(id+"/close", "Close", close).Variant(Ghost))
	e := Card(id, append([]*Element{header}, children...)...).Width(440).Hidden(!open)
	e.kind = ModalKind
	e.role = "dialog"
	e.label = title
	e.activate = close
	return e
}
func AlertDialog(id, title, message string, open bool, cancel, confirm func()) *Element {
	e := Dialog(id, title, open, cancel, Text(id+"/message", message), Row(id+"/actions", Button(id+"/cancel", "Cancel", cancel).Variant(Outline), Button(id+"/confirm", "Confirm", confirm)))
	e.role = "alertdialog"
	return e
}
func Alert(id, title, message string) *Element {
	e := Card(id, Text(id+"/title", title), Text(id+"/message", message))
	e.role = "alert"
	e.label = title
	return e
}
func Empty(id, title, description string, actions ...*Element) *Element {
	children := []*Element{Text(id+"/title", title), Text(id+"/description", description)}
	children = append(children, actions...)
	return Card(id, children...)
}
func Kbd(id, text string) *Element { return Badge(id, text).Variant(Secondary) }

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

// SelectState uses Go data only. This initial selection presentation expands
// inline; anchored popup placement and native platform menu integration are pending.
type SelectState struct {
	Value string
	Open  bool
}

func Select(id, label string, state *SelectState, options []Option) *Element {
	if state == nil {
		panic("baseui: nil selection state")
	}
	validateOptions(options)
	shown := "Select..."
	for _, o := range options {
		if o.ID == state.Value {
			shown = o.Label
		}
	}
	trigger := Button(id+"/trigger", shown, func() { state.Open = !state.Open }).Label(label + ": " + shown).Variant(Outline)
	trigger.role = "combobox"
	trigger.selected = state.Open
	children := []*Element{trigger}
	if state.Open {
		for _, o := range options {
			o := o
			b := Button(id+"/option/"+o.ID, o.Label, func() { state.Value = o.ID; state.Open = false }).Variant(Ghost).Disabled(o.Disabled)
			b.role = "option"
			b.selected = o.ID == state.Value
			children = append(children, b)
		}
	}
	return Column(id, children...).Gap(2)
}

// CodeBlock preserves source bytes, blank lines and tabs in CopyText. Syntax
// shaping, selection, scrolling and incremental highlighting are not yet provided.
type CodeBlockState struct{ Expanded bool }

func CodeBlock(id, title, source string, state *CodeBlockState, maxLines int, copyText func(string)) *Element {
	if state == nil {
		panic("baseui: nil code block state")
	}
	lines := strings.Split(source, "\n")
	shown := source
	if maxLines > 0 && !state.Expanded && len(lines) > maxLines {
		shown = strings.Join(lines[:maxLines], "\n") + "\n..."
	}
	controls := []*Element{Text(id+"/title", title).Grow(1)}
	if copyText != nil {
		controls = append(controls, Button(id+"/copy", "Copy", func() { copyText(source) }).Variant(Ghost))
	}
	if maxLines > 0 && len(lines) > maxLines {
		label := "Expand"
		if state.Expanded {
			label = "Collapse"
		}
		controls = append(controls, Button(id+"/expand", label, func() { state.Expanded = !state.Expanded }).Variant(Ghost))
	}
	return Card(id, Row(id+"/header", controls...), Text(id+"/source", shown))
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
