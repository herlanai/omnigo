package baseui

// NavigationItem callbacks are application-owned; no URL is opened implicitly.
type NavigationItem struct {
	ID, Label string
	Disabled  bool
	Action    func()
}

func navigation(id, label, value string, items []NavigationItem, vertical bool) *Element {
	items = append([]NavigationItem(nil), items...)
	options := make([]Option, len(items))
	for i, v := range items {
		options[i] = Option{ID: v.ID, Label: v.Label, Disabled: v.Disabled}
	}
	validateOptions(options)
	children := []*Element{}
	for i, item := range items {
		i, item := i, item
		b := Button(id+"/"+item.ID, item.Label, item.Action).Variant(Ghost).Disabled(item.Disabled || item.Action == nil).Role("link")
		b.selected = item.ID == value
		b.focusGroup = id
		b.alignStart = vertical
		options[i].Disabled = item.Disabled || item.Action == nil
		b.OnKey(func(ev KeyEvent) bool {
			n := optionKey(ev, i, options)
			if n < 0 {
				return false
			}
			if ev.RequestFocus != nil {
				ev.RequestFocus(id + "/" + items[n].ID)
			}
			return true
		})
		children = append(children, b)
	}
	root := Row(id, children...)
	if vertical {
		root = Column(id, children...).Width(240)
	}
	return root.Role("navigation").Label(label)
}
func NavigationMenu(id, value string, items []NavigationItem) *Element {
	return navigation(id, "Navigation", value, items, false)
}
func Sidebar(id, value string, items []NavigationItem) *Element {
	return navigation(id, "Sidebar", value, items, true)
}
func Breadcrumb(id string, items []NavigationItem) *Element {
	children := []*Element{}
	seen := map[string]bool{}
	for i, item := range items {
		if item.ID == "" || seen[item.ID] {
			panic("baseui: duplicate or empty breadcrumb ID")
		}
		seen[item.ID] = true
		if i > 0 {
			separator := Canvas(id+"/separator/"+item.ID, "", func(p *Painter, r Rect) { paintChevron(p, r.X+r.W/2, r.Y+r.H/2, false, false, p.Theme().MutedText) }).Width(14).Height(14).Role("presentation")
			children = append(children, separator)
		}
		if i == len(items)-1 {
			children = append(children, Text(id+"/"+item.ID, item.Label).FontSize(14).Height(24).Role("text").Description("Current page"))
		} else {
			link := Button(id+"/"+item.ID, item.Label, item.Action).Variant(Link).FontSize(14).Height(24).Role("link").Disabled(item.Disabled || item.Action == nil)
			link.inlineLink = true
			children = append(children, link)
		}
	}
	row := Row(id, children...).Gap(6).Role("navigation").Label("Breadcrumb")
	row.centerItems = true
	return row
}

type MenubarMenu struct {
	ID, Label string
	Disabled  bool
	Items     []MenuItem
}
type MenubarState struct{ menus map[string]*MenuState }

func Menubar(id string, state *MenubarState, menus []MenubarMenu) *Element {
	if state == nil {
		panic("baseui: nil menubar state")
	}
	if state.menus == nil {
		state.menus = map[string]*MenuState{}
	}
	options := make([]Option, len(menus))
	for i, m := range menus {
		options[i] = Option{ID: m.ID, Label: m.Label, Disabled: m.Disabled}
	}
	validateOptions(options)
	children := []*Element{}
	live := map[string]bool{}
	for i, m := range menus {
		i, m := i, m
		live[m.ID] = true
		ms := state.menus[m.ID]
		if ms == nil {
			ms = &MenuState{}
			state.menus[m.ID] = ms
		}
		root := DropdownMenu(id+"/"+m.ID, m.Label, ms, m.Items).Disabled(m.Disabled)
		trigger := root.children[0]
		trigger.focusGroup = id
		trigger.role = "menuitem"
		trigger.Variant(Ghost)
		original := trigger.activate
		trigger.activate = func() {
			for key, s := range state.menus {
				if key != m.ID {
					s.Open = false
					s.Path = nil
				}
			}
			original()
		}
		originalKey := trigger.onKey
		trigger.onKey = func(ev KeyEvent) bool {
			if ev.Key == "Left" || ev.Key == "Right" || ev.Key == "Home" || ev.Key == "End" {
				n := optionKey(ev, i, options)
				if n >= 0 && ev.RequestFocus != nil {
					ev.RequestFocus(id + "/" + menus[n].ID + "/trigger")
				}
				return true
			}
			if originalKey != nil {
				if ev.Key == "Down" || ev.Key == "Up" {
					for key, s := range state.menus {
						if key != m.ID {
							s.Open = false
							s.Path = nil
						}
					}
				}
				return originalKey(ev)
			}
			return false
		}

		// Root menu rows switch the open top-level menu; a Right key on a
		// submenu owner retains the submenu's own navigation behavior.
		for _, item := range m.Items {
			if item.Separator {
				continue
			}
			var row *Element
			for _, candidate := range root.children[1].children[0].children {
				if candidate.id == id+"/"+m.ID+"/item/"+item.ID {
					row = candidate
					break
				}
			}
			if row == nil {
				continue
			}
			oldKey := row.onKey
			row.onKey = func(ev KeyEvent) bool {
				key := ev.Key
				if ev.RTL {
					if key == "Left" {
						key = "Right"
					} else if key == "Right" {
						key = "Left"
					}
				}
				if key == "Left" || (key == "Right" && len(item.Children) == 0) {
					n := optionKey(ev, i, options)
					if n >= 0 && n != i {
						ms.Open = false
						ms.Path = nil
						next := state.menus[menus[n].ID]
						if next == nil {
							next = &MenuState{}
							state.menus[menus[n].ID] = next
						}
						next.Open = true
						next.Path = nil
						first := nextOption(menuOptions(menus[n].Items), len(menus[n].Items)-1, 1)
						if first >= 0 && ev.RequestFocus != nil {
							ev.RequestFocus(id + "/" + menus[n].ID + "/item/" + menus[n].Items[first].ID)
						}
					}
					return true
				}
				return oldKey != nil && oldKey(ev)
			}
		}
		children = append(children, root)
	}
	for key := range state.menus {
		if !live[key] {
			delete(state.menus, key)
		}
	}
	return Row(id, children...).Gap(2).Role("menubar").Label("Menu bar")
}
