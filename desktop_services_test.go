package omnigo

import (
	"errors"
	"testing"
	"testing/fstest"
)

func TestDesktopServicesBeforeReadiness(t *testing.T) {
	app, err := New(Options{Assets: fstest.MapFS{"index.html": {Data: []byte("test")}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.OpenDialog(t.Context(), FileDialogOptions{})
	if !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
	_, err = app.SaveDialog(t.Context(), FileDialogOptions{})
	if !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
	_, err = app.MessageDialog(t.Context(), MessageDialogOptions{})
	if !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
	_, err = app.NewTray(t.Context(), TrayOptions{ID: "x", Title: "X"})
	if !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
	_, err = app.WatchSystemEvents(t.Context())
	if !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
}
