package baseui

import (
	"bytes"
	"testing"
)

func TestCompositeRowFocusUsesFillWithoutClippedRing(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, style := range []VisualStyle{StyleMaia, StyleNova, StyleLyra} {
			for _, bold := range []bool{false, true} {
				for _, role := range []string{"menuitem", "menuitemcheckbox", "option"} {
					s, err := NewSurface(280, 180, ThemePreset(dark, style, AccentBlue, bold))
					if err != nil {
						t.Fatal(err)
					}
					row := Button("item", "Open workspace", func() {}).Variant(Ghost).Role(role)
					row.alignStart = true
					mount(t, s, Card("card", ScrollArea("scroll", &ScrollState{}, row).Height(36)).Padding(6))
					idle, _ := s.Render()
					if !s.Focus("item") {
						t.Fatal("row not focusable")
					}
					for _, scale := range []float64{1, 1.25, 2} {
						s.focusVisible = false
						pointer, err := s.RenderScaled(scale, nil)
						if err != nil {
							t.Fatal(err)
						}
						s.focusVisible = true
						keyboard, err := s.RenderScaled(scale, nil)
						if err != nil {
							t.Fatal(err)
						}
						if !bytes.Equal(pointer.Pix, keyboard.Pix) {
							t.Fatalf("extra focus outline: role=%s style=%s dark=%t bold=%t scale=%g", role, style, dark, bold, scale)
						}
						if scale == 1 && bytes.Equal(idle.Pix, keyboard.Pix) {
							t.Fatal("keyboard focus has no visual cue")
						}
					}
					s.Close()
				}
			}
		}
	}
}

func TestSelectAndTabsFollowLiveStyleRadius(t *testing.T) {
	s, err := NewSurface(400, 360, NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state := &SelectState{Open: true, Value: "one"}
	options := []Option{{ID: "one", Label: "One"}, {ID: "two", Label: "Two"}}
	mount(t, s, Column("root", Select("select", "Choice", state, options), selectionRow("tabs", "Tabs", "one", options, nil, false)))
	for _, style := range []VisualStyle{StyleMaia, StyleNova, StyleLyra} {
		if err := s.SetTheme(ThemePreset(false, style, AccentNeutral, false)); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"select/option/one", "tabs/one"} {
			e := s.entries[s.index[id]].element
			want := map[VisualStyle]float64{StyleMaia: 12, StyleNova: 6, StyleLyra: 0}[style]
			if id == "tabs/one" && style == StyleNova {
				want = 8
			}
			if got := controlRadius(s.theme, e); got != want {
				t.Fatalf("%s %s radius=%g want=%g", id, style, got, want)
			}
		}
	}
}
