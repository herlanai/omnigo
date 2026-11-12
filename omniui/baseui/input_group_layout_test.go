package baseui

import "testing"

func TestInputGroupReferenceSizingAndIntrinsicHeight(t *testing.T) {
	for _, width := range []float64{160, 360, 900} {
		s := surface(t)
		input := Input("input", "Domain", NewTextBuffer("docs.omnigo.dev"))
		suffix := Button("go", "Go", nil).Size(ControlXS)
		mount(t, s, Column("root", InputGroup("group", Text("prefix", "https://"), input, suffix).Width(width)))
		group, _ := s.Bounds("group")
		field, _ := s.Bounds("input")
		button, _ := s.Bounds("go")
		prefix, _ := s.Bounds("prefix")
		if group.H != 36 || field.H != 34 || button.H != 24 {
			t.Fatal("incorrect reference dimensions", group, field, button)
		}
		if button.Y-group.Y != 6 || group.Y+group.H-button.Y-button.H != 6 {
			t.Fatal("button not inset equally", group, button)
		}
		if prefix.X+prefix.W > field.X || field.X+field.W > button.X || button.X+button.W > group.X+group.W-12 {
			t.Fatal("overlapping input addons")
		}
		if input.height != 0 || suffix.height != 24 {
			t.Fatal("caller-owned children mutated")
		}
		mount(t, s, Column("root", InputGroup("tall", Text("prefix", "Notes"), Textarea("input", "Notes", NewTextBuffer("hello")).Height(120), Button("go", "Send", nil).Height(140)).Width(width)))
		group, _ = s.Bounds("tall")
		button, _ = s.Bounds("go")
		field, _ = s.Bounds("input")
		if group.H != 140 || field.Y+field.H > group.Y+group.H || button.H != 140 {
			t.Fatal("tall content clipped", group, field, button)
		}
	}
}
