package baseui

import (
	"strings"
	"time"
)

// SelectState stores application selection plus the scroll/typeahead state.
type SelectState struct {
	Value  string
	Open   bool
	scroll ScrollState
	search typeSearch
}

func Select(id, label string, state *SelectState, options []Option, onChange ...func(string)) *Element {
	if state == nil {
		panic("baseui: nil selection state")
	}
	options = append([]Option(nil), options...)
	validateOptions(options)
	shown := "Select..."
	selected := -1
	for i, o := range options {
		if o.ID == state.Value {
			shown = o.Label
			if !o.Disabled {
				selected = i
			}
		}
	}
	first := selected
	if first < 0 {
		first = nextOption(options, len(options)-1, 1)
	}
	trigger := Button(id+"/trigger", shown, func() { state.Open = !state.Open }).Label(label + ": " + shown).Variant(Outline).Role("combobox")
	trigger.selected = state.Open
	trigger.controls = id + "/popup"
	trigger.expanded = state.Open
	trigger.OnKey(func(ev KeyEvent) bool {
		if ev.Key != "Down" && ev.Key != "Up" {
			return false
		}
		state.Open = true
		n := first
		if ev.Key == "Up" && selected < 0 {
			n = nextOption(options, 0, -1)
		}
		if n >= 0 {
			ev.RequestFocus(id + "/option/" + options[n].ID)
		}
		return true
	})
	children := []*Element{}
	for i, o := range options {
		row := Button(id+"/option/"+o.ID, o.Label, func() {
			state.Value = o.ID
			state.Open = false
			for _, change := range onChange {
				if change != nil {
					change(o.ID)
				}
			}
		}).Variant(Ghost).Disabled(o.Disabled).Role("option")
		row.selected = o.ID == state.Value
		row.alignStart = true
		row.CornerRadius(12)
		row.OnKey(func(ev KeyEvent) bool {
			n := optionKey(ev, i, options)
			if n >= 0 {
				ev.RequestFocus(id + "/option/" + options[n].ID)
				return true
			}
			return false
		})
		row.onText = func(text string, now time.Duration) string {
			n := state.search.find(text, now, i, options)
			if n >= 0 {
				return id + "/option/" + options[n].ID
			}
			return ""
		}
		children = append(children, row)
	}
	if len(children) == 0 {
		children = append(children, Text(id+"/empty", "No options"))
	}
	viewport := ScrollArea(id+"/scroll", &state.scroll, children...).Height(min(280, float64(max(1, len(options)))*40)).Gap(2)
	popup := Popover(id+"/popup", trigger.id, state.Open, func() { state.Open = false }, viewport).Padding(6).Role("listbox")
	if first >= 0 {
		popup.AutoFocus(id + "/option/" + options[first].ID)
	}
	return Column(id, trigger, popup).Gap(2)
}

type ComboboxState struct {
	Value      string
	Query      *TextBuffer
	Open       bool
	active     string
	lastActive string
	scroll     ScrollState
}

// Combobox filters by label without changing Value until a result is selected.
// Query is a normal text buffer, retaining selection, undo and IME composition.
func Combobox(id, label string, state *ComboboxState, options []Option, change func(string)) *Element {
	if state == nil {
		panic("baseui: nil combobox state")
	}
	if state.Query == nil {
		state.Query = NewTextBuffer("")
	}
	options = append([]Option(nil), options...)
	validateOptions(options)
	filtered := []Option{}
	query := strings.ToLower(state.Query.String())
	for _, o := range options {
		if strings.Contains(strings.ToLower(o.Label), query) {
			filtered = append(filtered, o)
		}
	}
	active := -1
	for i, o := range filtered {
		if o.ID == state.active && !o.Disabled {
			active = i
		}
	}
	if active < 0 {
		active = nextOption(filtered, len(filtered)-1, 1)
	}
	selectOption := func(o Option) {
		state.Value = o.ID
		state.Query.Set(o.Label)
		state.Open = false
		state.active = o.ID
		if change != nil {
			change(o.ID)
		}
	}
	input := Input(id+"/input", label, state.Query).Placeholder("Search...").Role("combobox")
	input.controls = id + "/popup"
	input.expanded = state.Open
	input.onChange = func(string) { state.Open = true; state.active = ""; state.scroll.Y = 0 }
	input.OnKey(func(ev KeyEvent) bool {
		if state.Query.composition != "" {
			return false
		}
		switch ev.Key {
		case "Down", "Up":
			wasOpen := state.Open
			state.Open = true
			n := active
			if wasOpen {
				n = optionKey(ev, active, filtered)
			}
			if n >= 0 {
				state.active = filtered[n].ID
			}
			return true
		case "Enter":
			if state.Open && active >= 0 {
				selectOption(filtered[active])
				return true
			}
		}
		return false
	})
	rows := []*Element{}
	for i, o := range filtered {
		row := Button(id+"/option/"+o.ID, o.Label, func() { selectOption(o) }).Variant(Ghost).Disabled(o.Disabled).Role("option")
		row.selected = i == active
		row.alignStart = true
		row.CornerRadius(12)
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		rows = append(rows, Text(id+"/empty", "No results found."))
	}
	viewport := ScrollArea(id+"/scroll", &state.scroll, rows...).Height(min(280, float64(max(1, len(rows)))*40)).Gap(2)
	popup := Popover(id+"/popup", input.id, state.Open, func() { state.Open = false }, viewport).Padding(6).Role("listbox").AutoFocus(input.id)
	popup.allowAnchorFocus = true
	if active >= 0 {
		if state.lastActive != filtered[active].ID {
			popup.revealChild = id + "/option/" + filtered[active].ID
		}
		state.lastActive = filtered[active].ID
	} else {
		state.lastActive = ""
	}
	return Column(id, input, popup)
}
