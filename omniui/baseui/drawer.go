package baseui

// DrawerState is UI-thread owned. Dragging is confined to the explicit handle,
// leaving content scrolling and selection independent.
type DrawerState struct {
	Open          bool
	offset, start float64
	dragging      bool
}

func Drawer(id, title string, state *DrawerState, close func(), children ...*Element) *Element {
	if state == nil {
		panic("baseui: nil drawer state")
	}
	dismiss := func() {
		state.Open = false
		state.dragging = false
		state.offset = 0
		if close != nil {
			close()
		}
	}
	if !state.Open {
		state.offset = 0
		state.dragging = false
	}
	handle := Canvas(id+"/handle", "Drag down to close", func(p *Painter, r Rect) { p.Fill(Rect{r.X + (r.W-40)/2, r.Y + 8, 40, 4}, p.theme.MutedText, 2) }).Height(24).Role("button")
	handle.OnKey(func(ev KeyEvent) bool {
		if ev.Key == "Down" || ev.Key == "Enter" || ev.Key == "Space" {
			if !ev.Repeat {
				dismiss()
			}
			return true
		}
		return false
	})
	handle.onPointer = func(ev PointerEvent, _ Rect, screen Rect) {
		switch ev.Kind {
		case "down":
			state.dragging = true
			state.start = ev.Y
			state.offset = 0
		case "move":
			if state.dragging {
				state.offset = max(0, ev.Y-state.start)
			}
		case "cancel":
			state.dragging = false
			state.offset = 0
		case "up":
			if state.dragging && max(0, ev.Y-state.start) >= screen.H*.25 {
				dismiss()
			}
			state.dragging = false
			state.offset = 0
		}
	}
	e := Sheet(id, title, state.Open, Bottom, dismiss, children...)
	e.children = append([]*Element{handle}, e.children...)
	e.drawer = state
	return e
}
