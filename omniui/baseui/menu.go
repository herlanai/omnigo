package baseui

import (
	"strings"
	"time"
)

// MenuItem has a stable sibling-unique ID, including separators. Checked is
// optional caller-owned checkbox state; Actions run on the UI goroutine.
type MenuItem struct {
	ID, Label, Shortcut string
	Disabled, Separator bool
	Checked             *bool
	Children            []MenuItem
	Action              func()
}
type MenuState struct {
	Open   bool
	Path   []string
	scroll map[string]*ScrollState
	search typeSearch
}
type typeSearch struct {
	text string
	at   time.Duration
}

func (q *typeSearch) find(text string, now time.Duration, current int, options []Option) int {
	if now-q.at > 700*time.Millisecond || now < q.at {
		q.text = ""
	}
	q.at = now
	text = strings.ToLower(text)
	if text == q.text {
		q.text = ""
	}
	q.text += text
	for n := 1; n <= len(options); n++ {
		i := (current + n + len(options)) % len(options)
		if !options[i].Disabled && strings.HasPrefix(strings.ToLower(options[i].Label), q.text) {
			return i
		}
	}
	return -1
}
func menuOptions(items []MenuItem) []Option {
	result := make([]Option, len(items))
	for i, v := range items {
		result[i] = Option{ID: v.ID, Label: v.Label, Disabled: v.Disabled || v.Separator}
	}
	return result
}
func validateMenu(items []MenuItem, depth int) {
	if depth > 32 {
		panic("baseui: menu nesting exceeds 32")
	}
	validateOptions(menuOptions(items))
	for _, item := range items {
		validateMenu(item.Children, depth+1)
	}
}
func optionKey(ev KeyEvent, index int, options []Option) int {
	if ev.RTL {
		if ev.Key == "Right" {
			ev.Key = "Left"
		} else if ev.Key == "Left" {
			ev.Key = "Right"
		}
	}
	switch ev.Key {
	case "Down", "Right":
		return nextOption(options, index, 1)
	case "Up", "Left":
		return nextOption(options, index, -1)
	case "Home":
		return nextOption(options, len(options)-1, 1)
	case "End":
		return nextOption(options, 0, -1)
	}
	return -1
}

// DropdownMenu supports nested submenus, disabled rows, checkbox actions,
// shortcuts, typeahead and keyboard navigation. Shortcut text is descriptive;
// application-wide shortcut registration remains the application's responsibility.
func DropdownMenu(id, label string, state *MenuState, items []MenuItem) *Element {
	if state == nil {
		panic("baseui: nil menu state")
	}
	validateMenu(items, 0)
	trigger := Button(id+"/trigger", label, func() { state.Open = !state.Open; state.Path = nil }).Variant(Outline)
	trigger.controls = id + "/popup"
	trigger.expanded = state.Open
	trigger.OnKey(func(ev KeyEvent) bool {
		if ev.Key == "Down" || ev.Key == "Up" {
			state.Open = true
			state.Path = nil
			index := nextOption(menuOptions(items), len(items)-1, 1)
			if ev.Key == "Up" {
				index = nextOption(menuOptions(items), 0, -1)
			}
			if index >= 0 {
				ev.RequestFocus(id + "/item/" + items[index].ID)
			}
			return true
		}
		return false
	})
	return menuView(id, trigger, state, items)
}

func menuView(id string, trigger *Element, state *MenuState, items []MenuItem) *Element {
	if state == nil {
		panic("baseui: nil menu state")
	}
	validateMenu(items, 0)
	if state.scroll == nil {
		state.scroll = map[string]*ScrollState{}
	}
	closeAll := func() { state.Open = false; state.Path = nil; state.search = typeSearch{} }
	var build func(string, string, []MenuItem, int) *Element
	build = func(prefix, anchor string, entries []MenuItem, depth int) *Element {
		options := menuOptions(entries)
		children := []*Element{}
		for i, item := range entries {
			rowID := prefix + "/item/" + item.ID
			if item.Separator {
				children = append(children, Separator(rowID))
				continue
			}
			openSub := func() {
				if len(state.Path) > depth {
					state.Path = state.Path[:depth]
				}
				if len(state.Path) == depth {
					state.Path = append(state.Path, item.ID)
				}
			}
			activate := func() {
				if len(item.Children) > 0 {
					openSub()
					return
				}
				if item.Checked != nil {
					*item.Checked = !*item.Checked
				}
				closeAll()
				if item.Action != nil {
					item.Action()
				}
			}
			text := item.Label

			if item.Shortcut != "" {
				text += "   " + item.Shortcut
			}

			row := Button(rowID, text, activate).Variant(Ghost).Disabled(item.Disabled).Role("menuitem")
			row.alignStart = true
			row.CornerRadius(12)
			row.textIndent = 20
			if len(item.Children) > 0 {
				row.submenu = true
				row.controls = rowID + "/popup"
				row.expanded = len(state.Path) > depth && state.Path[depth] == item.ID
			}
			if item.Checked != nil {
				row.role = "menuitemcheckbox"
				row.checked = *item.Checked
			}
			row.OnKey(func(ev KeyEvent) bool {
				if ev.Key == "Right" && len(item.Children) > 0 {
					openSub()
					return true
				}
				if ev.Key == "Left" && depth > 0 {
					state.Path = state.Path[:min(len(state.Path), depth-1)]
					return true
				}
				if ev.Key == "Up" || ev.Key == "Down" || ev.Key == "Home" || ev.Key == "End" {
					n := optionKey(ev, i, options)
					if n >= 0 {
						ev.RequestFocus(prefix + "/item/" + entries[n].ID)
					}
					return true
				}
				return false
			})
			row.onText = func(text string, now time.Duration) string {
				n := state.search.find(text, now, i, options)
				if n >= 0 {
					return prefix + "/item/" + entries[n].ID
				}
				return ""
			}
			children = append(children, row)
			if !item.Disabled && len(item.Children) > 0 && len(state.Path) > depth && state.Path[depth] == item.ID {
				children = append(children, build(rowID, rowID, item.Children, depth+1).Placement(Beside))
			}
		}
		if state.scroll[prefix] == nil {
			state.scroll[prefix] = &ScrollState{}
		}
		contentHeight := float64(max(0, len(entries)-1)) * 2
		for _, item := range entries {
			if item.Separator {
				contentHeight++
			} else {
				contentHeight += 36
			}
		}
		viewport := ScrollArea(prefix+"/scroll", state.scroll[prefix], children...).Height(max(24, min(280, contentHeight))).Gap(2)
		close := closeAll
		if depth > 0 {
			close = func() { state.Path = state.Path[:min(len(state.Path), depth-1)] }
		}
		popup := Popover(prefix+"/popup", anchor, state.Open, close, viewport).Width(260).Padding(6).Role("menu")
		first := nextOption(options, len(options)-1, 1)
		if first >= 0 {
			popup.AutoFocus(prefix + "/item/" + entries[first].ID)
		}
		return popup
	}
	return Column(id, trigger, build(id, trigger.id, items, 0))
}
