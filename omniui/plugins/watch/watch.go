// Package watch reports bounded filesystem snapshots. It uses polling, not a
// platform event backend; changes between polls can be coalesced or missed.
package watch

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Options struct {
	Recursive  bool
	Interval   time.Duration
	MaxEntries int
}
type Event struct{ Op, Path, OldPath string }
type Watcher struct {
	mu       sync.Mutex
	root     *os.Root
	options  Options
	previous map[string]fs.FileInfo
	done     chan struct{}
	closed   bool
}

var ErrClosed = errors.New("watch: closed")

func Open(path string, o Options) (*Watcher, error) {
	if o.Interval <= 0 {
		o.Interval = 100 * time.Millisecond
	}
	if o.MaxEntries <= 0 {
		o.MaxEntries = 100000
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	w := &Watcher{root: root, options: o, done: make(chan struct{})}
	w.previous, err = w.snapshot(context.Background())
	if err != nil {
		root.Close()
		return nil, err
	}
	return w, nil
}
func (w *Watcher) snapshot(ctx context.Context) (map[string]fs.FileInfo, error) {
	result := make(map[string]fs.FileInfo)
	err := fs.WalkDir(w.root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		if len(result) >= w.options.MaxEntries {
			return errors.New("watch: entry limit exceeded")
		}
		info, err := w.root.Lstat(path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(path)] = info
		if d.IsDir() && !w.options.Recursive {
			return fs.SkipDir
		}
		return nil
	})
	return result, err
}

// Poll performs one deterministic scan. Errors preserve the previous snapshot.
func (w *Watcher) Poll(ctx context.Context) ([]Event, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil, ErrClosed
	}
	now, err := w.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	removed := make(map[string]fs.FileInfo)
	added := make(map[string]fs.FileInfo)
	var events []Event
	for path, old := range w.previous {
		cur, ok := now[path]
		if !ok {
			removed[path] = old
			continue
		}
		if !os.SameFile(old, cur) || old.Size() != cur.Size() || old.Mode() != cur.Mode() || !old.ModTime().Equal(cur.ModTime()) {
			events = append(events, Event{Op: "write", Path: path})
		}
	}
	for path, cur := range now {
		if _, ok := w.previous[path]; !ok {
			added[path] = cur
		}
	}
	// Only infer a rename for an unambiguous identity match (hard links are ambiguous).
	for oldPath, old := range removed {
		match := ""
		matches := 0
		for newPath, cur := range added {
			if os.SameFile(old, cur) {
				match = newPath
				matches++
			}
		}
		oldMatches := 0
		for _, candidate := range removed {
			if os.SameFile(old, candidate) {
				oldMatches++
			}
		}
		if matches == 1 && oldMatches == 1 {
			events = append(events, Event{Op: "rename", Path: match, OldPath: oldPath})
			delete(added, match)
		} else {
			events = append(events, Event{Op: "remove", Path: oldPath})
		}
	}
	for path := range added {
		events = append(events, Event{Op: "create", Path: path})
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Path == events[j].Path {
			return events[i].Op < events[j].Op
		}
		return events[i].Path < events[j].Path
	})
	w.previous = now
	return events, nil
}

// Next blocks only the caller. Close and context cancellation wake it immediately.
func (w *Watcher) Next(ctx context.Context) ([]Event, error) {
	timer := time.NewTicker(w.options.Interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-w.done:
			return nil, ErrClosed
		case <-timer.C:
			events, err := w.Poll(ctx)
			if err != nil || len(events) > 0 {
				return events, err
			}
		}
	}
}
func (w *Watcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	close(w.done)
	return w.root.Close()
}
