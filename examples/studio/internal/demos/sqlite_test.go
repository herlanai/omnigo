package demos

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ui "github.com/herlanai/omnigo/omniui/baseui"
)

func TestSQLiteDemoInteractions(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := map[bool]string{false: "light", true: "dark"}[dark]
		t.Run(name, func(t *testing.T) {
			d := New("plugin/sqlite")
			defer func() { d.Close(); <-d.sqlite.done }()
			s, err := ui.NewSurface(1040, 840, ui.NewTheme(dark))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			mount := func() {
				t.Helper()
				if err := s.Mount(d.View()); err != nil {
					t.Fatal(err)
				}
			}
			click := func(id string) {
				t.Helper()
				mount()
				s.Focus(id)
				if err := s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
					t.Fatal(err)
				}
				mount()
			}
			mount()
			click("sqlite/add")
			if d.sqlite.db != nil {
				t.Fatal("disabled action opened database")
			}
			click("sqlite/open")
			if !d.sqlite.ready || len(d.sqlite.rows) != 3 {
				t.Fatal(d.sqlite.message)
			}
			d.sqlite.title.Set("A real persisted row")
			click("sqlite/add")
			if len(d.sqlite.rows) != 4 {
				t.Fatal(d.sqlite.message)
			}
			click("sqlite/rollback")
			if len(d.sqlite.rows) != 4 || !strings.Contains(d.sqlite.message, "Rollback verified") {
				t.Fatal(d.sqlite.message)
			}
			// Toggle via the checkbox's pointer handler.
			b, ok := s.Bounds("sqlite/done/4")
			if !ok {
				t.Fatal("missing checkbox")
			}
			for _, kind := range []string{"down", "up"} {
				if err := s.Pointer(ui.PointerEvent{Kind: kind, X: b.X + 8, Y: b.Y + b.H/2}); err != nil {
					t.Fatal(err)
				}
			}
			if !d.sqlite.rows[3].Done {
				t.Fatal("checkbox update not persisted", d.sqlite.message)
			}
			d.sqlite.filter.Set("persisted")
			click("sqlite/search-button")
			if len(d.sqlite.rows) != 1 || d.sqlite.rows[0].ID != 4 {
				t.Fatal("filter failed", d.sqlite.message)
			}
			click("sqlite/delete/4")
			if len(d.sqlite.rows) != 0 {
				t.Fatal("delete failed", d.sqlite.message)
			}
			d.sqlite.filter.Set("")
			click("sqlite/refresh")
			if len(d.sqlite.rows) != 3 {
				t.Fatal("refresh failed")
			}
			img, err := s.Render()
			if err != nil {
				t.Fatal(err)
			}
			if dir := os.Getenv("OMNIGO_SQLITE_SCREENSHOTS"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(dir, "sqlite-"+name+".png"))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(f, img)
				closeErr := f.Close()
				if err != nil {
					t.Fatal(err)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
			}
		})
	}
}

func TestSQLiteDemoWorkerOwnershipAndClose(t *testing.T) {
	d := New("plugin/sqlite")
	d.sqliteContent()
	var work func() func()
	d.RunTask = func(fn func() func()) { work = fn }
	d.sqliteAction(nil)
	if !d.sqlite.busy || d.sqlite.ready || work == nil {
		t.Fatal("work was not scheduled")
	}
	completed := make(chan func(), 1)
	go func() { completed <- work() }()
	apply := <-completed
	if d.sqlite.ready || len(d.sqlite.rows) != 0 {
		t.Fatal("worker mutated UI state")
	}
	apply()
	if !d.sqlite.ready || len(d.sqlite.rows) != 3 {
		t.Fatal(d.sqlite.message)
	}
	d.sqliteAction(nil)
	d.Close() // Cancellation before the queued worker starts; discard its callback.
	go func() { completed <- work() }()
	<-completed
	<-d.sqlite.done
	if _, err := d.sqlite.db.Query(context.Background(), "SELECT 1"); err == nil {
		t.Fatal("database leaked after closure")
	}
	d.Close()
}
