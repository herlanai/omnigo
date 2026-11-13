package watch

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func mustWatch(t *testing.T, dir string) *Watcher {
	t.Helper()
	w, err := Open(dir, Options{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close() })
	return w
}
func expectEvents(t *testing.T, w *Watcher, want ...Event) {
	t.Helper()
	got, err := w.Poll(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events: %+v; want %+v", got, want)
	}
}
func mustDo(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestWatchDirectoryAndPermissionChangesDoNotWrite(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	mustDo(t, os.Mkdir(sub, 0700))
	file := filepath.Join(sub, "file")
	mustDo(t, os.WriteFile(file, []byte("old"), 0600))
	w := mustWatch(t, dir)
	mustDo(t, os.Chmod(file, 0640))
	mustDo(t, os.Chtimes(sub, time.Unix(100, 0), time.Unix(100, 0)))
	expectEvents(t, w)
	mustDo(t, os.WriteFile(filepath.Join(sub, "child"), nil, 0600))
	expectEvents(t, w, Event{Op: "create", Path: "sub/child"})
	mustDo(t, os.WriteFile(file, []byte("longer"), 0600))
	expectEvents(t, w, Event{Op: "write", Path: "sub/file"})
}
func TestWatchTypeReplacementAndDirectoryFlags(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "entry")
	mustDo(t, os.WriteFile(target, nil, 0600))
	w := mustWatch(t, dir)
	mustDo(t, os.Remove(target))
	mustDo(t, os.Mkdir(target, 0700))
	expectEvents(t, w, Event{Op: "remove", Path: "entry"}, Event{Op: "create", Path: "entry", IsDir: true})
	mustDo(t, os.Rename(target, filepath.Join(dir, "moved")))
	expectEvents(t, w, Event{Op: "rename", Path: "moved", OldPath: "entry", IsDir: true})
	mustDo(t, os.Remove(filepath.Join(dir, "moved")))
	expectEvents(t, w, Event{Op: "remove", Path: "moved", IsDir: true})
}
func TestWatchAtomicReplacementEvenWithSameSizeAndTimestamp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "file")
	mustDo(t, os.WriteFile(target, []byte("old"), 0600))
	stamp := time.Unix(100, 0)
	mustDo(t, os.Chtimes(target, stamp, stamp))
	w := mustWatch(t, dir)
	temp := filepath.Join(dir, "save.tmp")
	mustDo(t, os.WriteFile(temp, []byte("new"), 0600))
	mustDo(t, os.Chtimes(temp, stamp, stamp))
	mustDo(t, os.Rename(temp, target))
	expectEvents(t, w, Event{Op: "write", Path: "file"})
}
func TestWatchRenameAndWriteBothDelivered(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "before")
	next := filepath.Join(dir, "after")
	mustDo(t, os.WriteFile(old, []byte("x"), 0600))
	w := mustWatch(t, dir)
	mustDo(t, os.Rename(old, next))
	mustDo(t, os.WriteFile(next, []byte("changed"), 0600))
	expectEvents(t, w, Event{Op: "rename", Path: "after", OldPath: "before"}, Event{Op: "write", Path: "after"})
}
func TestWatchRetainedHardLinkMakesRenameAmbiguous(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old")
	mustDo(t, os.WriteFile(old, nil, 0600))
	if err := os.Link(old, filepath.Join(dir, "retained")); err != nil {
		t.Skip(err)
	}
	w := mustWatch(t, dir)
	mustDo(t, os.Rename(old, filepath.Join(dir, "new")))
	expectEvents(t, w, Event{Op: "create", Path: "new"}, Event{Op: "remove", Path: "old"})
}

func TestWatchDirectoryMoveSuppressesDescendantRenames(t *testing.T) {
	dir := t.TempDir()
	mustDo(t, os.MkdirAll(filepath.Join(dir, "a", "nested"), 0700))
	mustDo(t, os.WriteFile(filepath.Join(dir, "a", "nested", "file"), []byte("value"), 0600))
	w := mustWatch(t, dir)
	mustDo(t, os.Rename(filepath.Join(dir, "a"), filepath.Join(dir, "z")))
	expectEvents(t, w, Event{Op: "rename", Path: "z", OldPath: "a", IsDir: true})
	// The new path is now the baseline; later writes use it.
	mustDo(t, os.WriteFile(filepath.Join(dir, "z", "nested", "file"), []byte("updated"), 0600))
	expectEvents(t, w, Event{Op: "write", Path: "z/nested/file"})
}
func TestWatchDirectoryMovePreservesIndependentChanges(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	z := filepath.Join(dir, "z")
	mustDo(t, os.MkdirAll(filepath.Join(a, "sub"), 0700))
	for _, name := range []string{"sub/file", "gone", "relabel"} {
		mustDo(t, os.WriteFile(filepath.Join(a, filepath.FromSlash(name)), []byte("old"), 0600))
	}
	w := mustWatch(t, dir)
	mustDo(t, os.Rename(a, z))
	mustDo(t, os.Rename(filepath.Join(z, "sub"), filepath.Join(z, "other")))
	mustDo(t, os.Rename(filepath.Join(z, "relabel"), filepath.Join(z, "changed")))
	mustDo(t, os.WriteFile(filepath.Join(z, "other", "file"), []byte("updated content"), 0600))
	mustDo(t, os.WriteFile(filepath.Join(z, "new"), []byte("created"), 0600))
	mustDo(t, os.Remove(filepath.Join(z, "gone")))
	expectEvents(t, w,
		Event{Op: "remove", Path: "a/gone"},
		Event{Op: "rename", Path: "z", OldPath: "a", IsDir: true},
		Event{Op: "rename", Path: "z/changed", OldPath: "a/relabel"},
		Event{Op: "create", Path: "z/new"},
		Event{Op: "rename", Path: "z/other", OldPath: "a/sub", IsDir: true},
		Event{Op: "write", Path: "z/other/file"},
	)
}
func TestWatchSiblingPrefixIsNotADirectoryAncestor(t *testing.T) {
	dir := t.TempDir()
	mustDo(t, os.Mkdir(filepath.Join(dir, "a"), 0700))
	mustDo(t, os.Mkdir(filepath.Join(dir, "ab"), 0700))
	mustDo(t, os.WriteFile(filepath.Join(dir, "ab", "file"), nil, 0600))
	w := mustWatch(t, dir)
	mustDo(t, os.Rename(filepath.Join(dir, "a"), filepath.Join(dir, "z")))
	mustDo(t, os.Rename(filepath.Join(dir, "ab", "file"), filepath.Join(dir, "z", "file")))
	expectEvents(t, w, Event{Op: "rename", Path: "z", OldPath: "a", IsDir: true},
		Event{Op: "rename", Path: "z/file", OldPath: "ab/file"})
}
