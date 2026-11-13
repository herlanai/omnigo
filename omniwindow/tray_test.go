package omniwindow

import "testing"

func TestTrayValidationAndSnapshot(t *testing.T) {
	invalid := []TrayOptions{{}, {ID: "x"}, {ID: "x", Title: "X", Icon: []byte("not PNG")}, {ID: "x", Title: "X", Menu: []MenuItem{{ID: "a", Label: "A"}, {ID: "a", Label: "B"}}}, {ID: "x", Title: "X", Menu: []MenuItem{{Separator: true, Items: []MenuItem{{ID: "a", Label: "A"}}}}}}
	for _, o := range invalid {
		if _, err := PrepareTray(o); err == nil {
			t.Fatal("invalid tray accepted", o)
		}
	}
	original := TrayOptions{ID: "x", Title: "X", Menu: []MenuItem{{ID: "parent", Label: "Parent", Items: []MenuItem{{ID: "child", Label: "Child"}}}}}
	copy, err := PrepareTray(original)
	if err != nil {
		t.Fatal(err)
	}
	original.Menu[0].Items[0].Label = "changed"
	if copy.Menu[0].Items[0].Label != "Child" {
		t.Fatal("submenu snapshot aliases caller")
	}
	nested := []MenuItem{{ID: "leaf", Label: "Leaf"}}
	for i := range 10 {
		nested = []MenuItem{{ID: string(rune('a' + i)), Label: "Nested", Items: nested}}
	}
	if _, err := PrepareTray(TrayOptions{ID: "deep", Title: "X", Menu: nested}); err == nil {
		t.Fatal("deep menu accepted")
	}
}
