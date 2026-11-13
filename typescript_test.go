package omnigo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestGenerationModeDoesNotStartApplication(t *testing.T) {
	app, err := New(Options{Assets: fstest.MapFS{"index.html": {}}, Services: []Service{{ID: "test.never", Start: func(context.Context) error { t.Error("started service"); return nil }}}, Registrations: []Registration{Query("app.version", func() string { t.Error("invoked handler"); return "1" })}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ui", "services.ts")
	t.Setenv("OMNIGO_GENERATE", path)
	if err = app.RunMain(); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = app.RunMain(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("identical generated content rewritten")
	}
	if err = Bind(app, "invalid", "", func(context.Context, chan int) (int, error) { return 0, nil }); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(path)
	if err = app.WriteTypeScript(path); err == nil {
		t.Fatal("unsupported type accepted")
	}
	current, _ := os.ReadFile(path)
	if string(original) != string(current) {
		t.Fatal("generation failure overwrote previous contracts")
	}
}
