package main

import (
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"testing"
)

func TestShowcaseUsesInteractiveControls(t *testing.T) {
	for _, dark := range []bool{false, true} {
		d := newDashboard()
		s, err := ui.NewSurface(1320, 1000, ui.NewTheme(dark))
		if err != nil {
			t.Fatal(err)
		}
		mount := func() {
			t.Helper()
			if err := s.Mount(d.view(dark)); err != nil {
				t.Fatal(err)
			}
		}
		mount()
		if !s.Focus("amount") {
			t.Fatal("slider not focusable")
		}
		if err = s.Key(ui.KeyEvent{Key: "Right"}); err != nil {
			t.Fatal(err)
		}
		mount()
		if d.amount != 2550 {
			t.Fatal("slider did not update state")
		}
		if !s.Focus("notes") {
			t.Fatal("notes not focusable")
		}
		if err = s.TextInput("Monthly payout"); err != nil {
			t.Fatal(err)
		}
		if d.notes.String() != "Monthly payout" {
			t.Fatal("text input failed")
		}
		s.Focus("save")
		if err = s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
			t.Fatal(err)
		}
		mount()
		if !d.saved {
			t.Fatal("save callback failed")
		}
		for _, id := range []string{"history", "payout", "goals", "balance", "notifications", "security", "footer"} {
			r, ok := s.Bounds(id)
			if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 1320 || r.Y+r.H > 1000 {
				t.Fatalf("%s clipped: %+v", id, r)
			}
		}
		s.Close()
	}
}
