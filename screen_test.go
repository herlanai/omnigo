package omnigo

import (
	"errors"
	"testing"
	"testing/fstest"
)

func TestDesktopQueriesRequireReadyHost(t *testing.T) {
	app, err := New(Options{Assets: fstest.MapFS{"index.html": {Data: []byte("test")}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Screen(t.Context()); !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
	if _, err := app.Appearance(t.Context()); !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
	if _, err := app.SetAppearance(t.Context(), AppearanceDark); !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
}
