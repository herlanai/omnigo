package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestDebounceQuietWindowAndNetChanges(t *testing.T) {
	dir := t.TempDir()
	w := mustWatch(t, dir)
	epoch := time.Unix(100, 0)
	scan := func(ms int, want ...Event) {
		t.Helper()
		events, err := w.scanAt(t.Context(), epoch.Add(time.Duration(ms)*time.Millisecond), true)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) == 0 && len(want) == 0 {
			return
		}
		if !reflect.DeepEqual(events, want) {
			t.Fatal(ms, events, want)
		}
	}
	mustDo(t, os.WriteFile(filepath.Join(dir, "file"), []byte("one"), 0600))
	scan(0)
	mustDo(t, os.WriteFile(filepath.Join(dir, "file"), []byte("longer"), 0600))
	scan(30)
	scan(79)
	scan(80, Event{Op: "create", Path: "file"})
	// A file that exists only during the quiet period must not leak a create/remove pair.
	mustDo(t, os.WriteFile(filepath.Join(dir, "temporary"), nil, 0600))
	scan(100)
	mustDo(t, os.Remove(filepath.Join(dir, "temporary")))
	scan(120)
	scan(170)
	if w.pending {
		t.Fatal("empty net batch did not clear pending state")
	}
}
func TestDebounceMaxDelayBoundsContinuousChanges(t *testing.T) {
	dir := t.TempDir()
	w, err := Open(dir, Options{Debounce: 50 * time.Millisecond, MaxDelay: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	epoch := time.Unix(100, 0)
	for i, ms := range []int{0, 30, 60, 90} {
		mustDo(t, os.WriteFile(filepath.Join(dir, "file"), make([]byte, i+1), 0600))
		events, err := w.scanAt(t.Context(), epoch.Add(time.Duration(ms)*time.Millisecond), true)
		if err != nil || len(events) != 0 {
			t.Fatal(ms, events, err)
		}
	}
	if delay := w.nextScanDelay(epoch.Add(90 * time.Millisecond)); delay != 10*time.Millisecond {
		t.Fatal(delay)
	}
	events, err := w.scanAt(t.Context(), epoch.Add(100*time.Millisecond), true)
	if err != nil || len(events) != 1 || events[0].Op != "create" {
		t.Fatal(events, err)
	}
}
func TestDebounceFailureAndCancellationPreservePendingBatch(t *testing.T) {
	dir := t.TempDir()
	fail := false
	w, err := Open(dir, Options{Ignore: func(string, bool) bool {
		if fail {
			panic("bad filter")
		}
		return false
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	epoch := time.Unix(100, 0)
	mustDo(t, os.WriteFile(filepath.Join(dir, "file"), nil, 0600))
	if _, err = w.scanAt(t.Context(), epoch, true); err != nil {
		t.Fatal(err)
	}
	fail = true
	if _, err = w.scanAt(t.Context(), epoch.Add(30*time.Millisecond), true); err == nil {
		t.Fatal("missing error")
	}
	fail = false
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err = w.scanAt(ctx, epoch.Add(50*time.Millisecond), true); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	events, err := w.scanAt(t.Context(), epoch.Add(50*time.Millisecond), true)
	if err != nil || len(events) != 1 {
		t.Fatal(events, err)
	}
}
func TestNegativeDebounceAndExplicitPollRemainImmediate(t *testing.T) {
	for _, immediatePoll := range []bool{false, true} {
		dir := t.TempDir()
		o := Options{Debounce: -1}
		if immediatePoll {
			o.Debounce = time.Hour
		}
		w, err := Open(dir, o)
		if err != nil {
			t.Fatal(err)
		}
		mustDo(t, os.WriteFile(filepath.Join(dir, "file"), nil, 0600))
		var events []Event
		if immediatePoll {
			events, err = w.Poll(t.Context())
		} else {
			events, err = w.scanAt(t.Context(), time.Unix(0, 0), true)
		}
		if err != nil || len(events) != 1 {
			t.Fatal(events, err)
		}
		w.Close()
	}
}
func TestNextCancellationWhileDebouncingDoesNotLoseBatch(t *testing.T) {
	dir := t.TempDir()
	w := mustWatch(t, dir)
	mustDo(t, os.WriteFile(filepath.Join(dir, "file"), nil, 0600))
	epoch := time.Unix(100, 0)
	if _, err := w.scanAt(t.Context(), epoch, true); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := w.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	events, err := w.scanAt(t.Context(), epoch.Add(50*time.Millisecond), true)
	if err != nil || len(events) != 1 {
		t.Fatal(events, err)
	}
}
