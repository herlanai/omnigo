package baseui

import "testing"

func TestActionRowActivation(t *testing.T) {
	s, err := NewSurface(300, 150, ThemePreset(false, StyleNova, AccentBlue, false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	calls := 0
	mount := func(disabled bool) {
		t.Helper()
		if err := s.Mount(ActionRow("profile", "个人信息", func() { calls++ }, Avatar("avatar", "林悦", nil), Text("name", "林悦")).Height(60).Disabled(disabled)); err != nil {
			t.Fatal(err)
		}
	}
	mount(false)
	for _, kind := range []string{"down", "up"} {
		if err := s.Pointer(PointerEvent{Kind: kind, X: 20, Y: 20}); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatal("child prevented row activation")
	}
	if err := s.AccessibilityAction("profile", "press", ""); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("accessibility activation failed")
	}
	mount(true)
	for _, kind := range []string{"down", "up"} {
		s.Pointer(PointerEvent{Kind: kind, X: 20, Y: 20})
	}
	if calls != 2 {
		t.Fatal("disabled row activated")
	}
}
