package demos

import (
	ui "omnigo.dev/omnigo/omniui/baseui"
	"strings"
	"testing"
)

func TestEveryDemoMountsAndRenders(t *testing.T) {
	for _, id := range IDs {
		for _, dark := range []bool{false, true} {
			t.Run(id+map[bool]string{false: "/light", true: "/dark"}[dark], func(t *testing.T) {
				s, err := ui.NewSurface(720, 680, ui.NewTheme(dark))
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()
				d := New(id)
				if err := s.Mount(d.View()); err != nil {
					t.Fatal(err)
				}
				if _, err := s.Render(); err != nil {
					t.Fatal(err)
				}
				if len(s.Accessibility()) == 0 {
					t.Fatal("empty accessible tree")
				}
			})
		}
	}
}
func TestActionStateAndDisabled(t *testing.T) {
	d := New("button")
	s, _ := ui.NewSurface(720, 680, ui.NewTheme(false))
	defer s.Close()
	if err := s.Mount(d.View()); err != nil {
		t.Fatal(err)
	}
	click := func(id string) {
		t.Helper()
		b, ok := s.Bounds(id)
		if !ok {
			t.Fatal(id)
		}
		for _, kind := range []string{"down", "up"} {
			if err := s.Pointer(ui.PointerEvent{Kind: kind, X: b.X + b.W/2, Y: b.Y + b.H/2}); err != nil {
				t.Fatal(err)
			}
		}
	}
	click("action")
	click("disabled")
	if d.Count != 1 {
		t.Fatalf("count=%d", d.Count)
	}
	if err := s.Mount(d.View()); err != nil {
		t.Fatal(err)
	}
	fresh := New("button")
	if fresh.Count != 0 {
		t.Fatal("new demo did not reset state")
	}
}
func TestDialogActuallyOpens(t *testing.T) {
	d := New("dialog")
	s, _ := ui.NewSurface(720, 680, ui.NewTheme(false))
	defer s.Close()
	if err := s.Mount(d.View()); err != nil {
		t.Fatal(err)
	}
	s.Focus("open")
	if err := s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
		t.Fatal(err)
	}
	if !d.Open {
		t.Fatal("dialog did not open")
	}
	if err := s.Mount(d.View()); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Bounds("modal/name"); !ok {
		t.Fatal("dialog body absent")
	}
}

func TestTerminalSampleColumns(t *testing.T) {
	lines := strings.Split(terminalSampleOutput(), "\n")
	statusColumn := strings.Index(lines[0], "STATUS")
	durationColumn := strings.Index(lines[0], "DURATION")
	for _, line := range lines[1:3] {
		if strings.Index(line, "ok") != statusColumn {
			t.Fatal("sample status column is not aligned")
		}
		if strings.Index(line, "0.") != durationColumn {
			t.Fatal("sample duration column is not aligned")
		}
	}
}
