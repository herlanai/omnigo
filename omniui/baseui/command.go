package baseui

import "strings"

type CommandItem struct {
	ID, Label, Group, Shortcut string
	Keywords                   []string
	Disabled                   bool
	Action                     func()
}
type CommandState struct {
	lastActive string

	Query  *TextBuffer
	active string
	scroll ScrollState
}

// Command is an inline searchable command list. It supports group labels,
// keywords, disabled actions and keyboard selection. Put it in Dialog to build
// a command palette; opening shortcuts belong to the application.
func Command(id string, state *CommandState, items []CommandItem) *Element {
	if state == nil {
		panic("baseui: nil command state")
	}
	if state.Query == nil {
		state.Query = NewTextBuffer("")
	}
	ids := make([]Option, len(items))
	for i, item := range items {
		ids[i] = Option{ID: item.ID}
	}
	validateOptions(ids)
	query := strings.ToLower(state.Query.String())
	filtered := []CommandItem{}
	options := []Option{}
	for _, item := range items {
		haystack := strings.ToLower(item.Label + " " + item.Group + " " + strings.Join(item.Keywords, " "))
		if strings.Contains(haystack, query) {
			filtered = append(filtered, item)
			options = append(options, Option{ID: item.ID, Label: item.Label, Disabled: item.Disabled})
		}
	}
	active := -1
	for i, item := range filtered {
		if item.ID == state.active && !item.Disabled {
			active = i
		}
	}
	if active < 0 {
		active = nextOption(options, len(options)-1, 1)
	}
	input := Input(id+"/query", "Search commands", state.Query).Placeholder("Type a command...")
	input.onChange = func(string) { state.active = ""; state.scroll.Y = 0 }
	input.OnKey(func(ev KeyEvent) bool {
		if state.Query.composition != "" {
			return false
		}
		if ev.Key == "Down" || ev.Key == "Up" {
			n := optionKey(ev, active, options)
			if n >= 0 {
				state.active = options[n].ID

			}
			return true
		}
		if ev.Key == "Enter" && active >= 0 {
			if !ev.Repeat && filtered[active].Action != nil {
				filtered[active].Action()
			}
			return true
		}
		return false
	})
	children := []*Element{}
	group := ""
	for i, item := range filtered {
		if item.Group != "" && item.Group != group {
			group = item.Group
			children = append(children, Text(id+"/group/"+item.ID, item.Group).Role("note"))
		}
		text := item.Label
		if item.Shortcut != "" {
			text += "   " + item.Shortcut
		}
		row := Button(id+"/item/"+item.ID, text, item.Action).Disabled(item.Disabled).Variant(Ghost).Role("option")
		row.selected = i == active
		row.alignStart = true
		row.OnKey(func(ev KeyEvent) bool {
			n := optionKey(ev, i, options)
			if n >= 0 {
				state.active = options[n].ID
				ev.RequestFocus(id + "/item/" + options[n].ID)
				return true
			}
			return false
		})
		children = append(children, row)
	}
	if len(children) == 0 {
		children = append(children, Text(id+"/empty", "No commands found."))
	}
	list := ScrollArea(id+"/list", &state.scroll, children...).Height(240).Gap(4).Role("listbox")
	if active >= 0 {
		if state.lastActive != filtered[active].ID {
			list.revealChild = id + "/item/" + filtered[active].ID
		}
		state.lastActive = filtered[active].ID
	} else {
		state.lastActive = ""
	}
	return Column(id, input, list).Gap(8)
}
