package baseui

// ActionRow is a keyboard-accessible button containing noninteractive content.
// Children must not contain nested interactive controls.
func ActionRow(id, label string, action func(), children ...*Element) *Element {
	e := Row(id, children...)
	e.role, e.label, e.activate = "button", label, action
	e.onKey = func(KeyEvent) bool { return false }
	hovered := false
	e.onHover = func(ev PointerEvent, _ Rect) { hovered = ev.Kind != "cancel" }
	e.paint = func(p *Painter, r Rect) {
		c := p.Theme().Surface
		if hovered {
			c = p.Theme().Hover
		}
		p.Fill(r, c, p.Theme().Radius)
	}
	return e
}
