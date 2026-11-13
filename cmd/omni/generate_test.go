package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "services.ts")
	calls := 0
	run := func(pkg, out string) error {
		calls++
		if pkg != "./cmd/app" || filepath.Dir(filepath.Dir(out)) != filepath.Dir(path) {
			t.Errorf("%s %s", pkg, out)
		}
		return os.WriteFile(out, []byte("export type Services = {};"), 0600)
	}
	if err := generateWith([]string{"-package", "./cmd/app", "-out", path}, io.Discard, run); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, {"-out", path, "extra"}, {"-package", "-bad", "-out", path}} {
		if err := generateWith(args, io.Discard, run); err == nil {
			t.Fatal("invalid args accepted", args)
		}
	}
	if calls != 1 {
		t.Fatal("invalid args executed application")
	}
}

func TestGenerateDoesNotAcceptStaleFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "services.ts")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	err := generateWith([]string{"-out", path}, io.Discard, func(string, string) error { return nil })
	if err == nil {
		t.Fatal("stale output accepted")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "old" {
		t.Fatal("old file changed")
	}
}
