package demos

import (
	ui "omnigo.dev/omnigo/omniui/baseui"
	"strings"
	"testing"
)

func TestDemoBrandAndInputGroupPresentation(t *testing.T) {
	s, err := ui.NewSurface(1040, 840, ui.NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d := New("input-group")
	if err = s.Mount(d.View()); err != nil {
		t.Fatal(err)
	}
	for _, node := range s.Accessibility() {
		if strings.Contains(strings.ToLower(node.Label+node.Value+node.Description), "maia") {
			t.Fatal("reference theme name remains in UI", node.ID)
		}
	}
	frame, _ := s.Bounds("demo")
	button, _ := s.Bounds("go")
	card, _ := s.Bounds("playground")
	if frame.H != 36 || button.H != 24 || card.H > 220 {
		t.Fatal("incorrect or excessively empty preview", frame, button, card)
	}
	if d.Buffer.String() != "docs.omnigo.dev" {
		t.Fatal("domain demo contains unrelated text")
	}
}
