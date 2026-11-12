package demos

import (
	"fmt"
	ui "omnigo.dev/omnigo/omniui/baseui"
	"testing"
	"time"
)

// This sweep checks shared integration invariants, not reference parity. Each
// control still needs its own capability and native-window acceptance tests.
func TestAllDemoInteractionSweep(t *testing.T) {
	for _, id := range IDs {
		for _, dark := range []bool{false, true} {
			for _, size := range [][2]int{{320, 480}, {720, 680}} {
				t.Run(fmt.Sprintf("%s/dark=%t/%d", id, dark, size[0]), func(t *testing.T) {
					s, err := ui.NewSurface(size[0], size[1], ui.NewTheme(dark))
					if err != nil {
						t.Fatal(err)
					}
					defer s.Close()
					d := New(id)
					defer d.Close()
					rebuild := func() {
						t.Helper()
						if err := s.Mount(d.View()); err != nil {
							t.Fatal(err)
						}
					}
					rebuild()
					events := []ui.KeyEvent{{Key: "Tab"}, {Key: "Enter"}, {Key: "Down"}, {Key: "Right"}, {Key: "End"}, {Key: "Escape"}, {Key: "Tab"}, {Key: "Space"}, {Key: "Space", Up: true}, {Key: "Tab", Shift: true}}
					for i, event := range events {
						if err := s.Advance(time.Duration(i) * 20 * time.Millisecond); err != nil {
							t.Fatal(err)
						}
						if err := s.Key(event); err != nil {
							t.Fatalf("%+v: %v", event, err)
						}
						rebuild()
					}
					if err := s.TextInput("a9"); err != nil {
						t.Fatal(err)
					}
					s.SetComposition("中")
					if err := s.CommitComposition(); err != nil {
						t.Fatal(err)
					}
					rebuild()
					if err := s.Wheel(ui.WheelEvent{X: float64(size[0]) / 2, Y: float64(size[1]) / 2, DeltaY: 90}); err != nil {
						t.Fatal(err)
					}
					rebuild()
					if err := s.Pointer(ui.PointerEvent{Kind: "cancel"}); err != nil {
						t.Fatal(err)
					}
					if _, err := s.Render(); err != nil {
						t.Fatal(err)
					}
					if err := s.Mount(d.View().Disabled(true)); err != nil {
						t.Fatal(err)
					}
					for _, node := range s.Accessibility() {
						if s.Focus(node.ID) {
							t.Fatalf("disabled descendant focused: %s", node.ID)
						}
					}
					if _, err := s.Render(); err != nil {
						t.Fatal(err)
					}
					if err := s.Mount(d.View().ReadOnly(true)); err != nil {
						t.Fatal(err)
					}
					if err := s.Key(ui.KeyEvent{Key: "Tab"}); err != nil {
						t.Fatal(err)
					}
					before := s.Accessibility()
					if err := s.TextInput("readonly"); err != nil {
						t.Fatal(err)
					}
					s.SetComposition("readonly")
					if err := s.CommitComposition(); err != nil {
						t.Fatal(err)
					}
					after := s.Accessibility()
					if len(before) != len(after) {
						t.Fatal("read-only text changed tree")
					}
					for i := range before {
						if before[i].Value != after[i].Value {
							t.Fatalf("read-only value changed: %s", before[i].ID)
						}
					}
					if err := s.Mount(ui.Text("empty", "Unloaded")); err != nil {
						t.Fatal(err)
					}
					if _, scheduled := s.NextWakeup(); scheduled {
						t.Fatal("unmounted demo still schedules work")
					}
				})
			}
		}
	}
}
