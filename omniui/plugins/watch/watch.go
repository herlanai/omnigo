// Package watch reports bounded filesystem snapshots. It uses polling, not a
// platform event backend; changes between polls can be coalesced or missed.
package watch

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Options struct {
	// Ignore receives slash-separated paths relative to the watched directory.
	// Ignored directories are pruned. Callbacks must not call this Watcher.
	Ignore    func(path string, isDir bool) bool
	Recursive bool
	// Debounce defaults to 50ms; negative disables it. Poll always scans immediately.
	Debounce time.Duration
	// MaxDelay bounds continuous changes; defaults to 500ms, at least Debounce.
	MaxDelay   time.Duration
	Interval   time.Duration
	MaxEntries int
}
type Event struct {
	IsDir   bool   `json:"isDir"`
	Op      string `json:"op"`
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"`
}
type Watcher struct {
	mu                      sync.Mutex
	root                    *os.Root
	file                    string
	fileGone, ended         bool
	rootPath                string
	rootIdentity            fs.FileInfo
	rootGone                bool
	options                 Options
	previous                map[string]fs.FileInfo
	observed                map[string]fs.FileInfo
	pending                 bool
	firstChange, lastChange time.Time
	done                    chan struct{}
	closed                  bool
}

var ErrClosed = errors.New("watch: closed")

func Open(path string, o Options) (*Watcher, error) {
	root, file, err := openTarget(path)
	if err != nil {
		return nil, err
	}
	return openRoot(context.Background(), root, o, file)
}

// openRoot takes ownership even if the initial snapshot fails.
func openRoot(ctx context.Context, root *os.Root, o Options, target ...string) (*Watcher, error) {
	if o.Debounce == 0 {
		o.Debounce = 50 * time.Millisecond
	}
	if o.MaxDelay <= 0 {
		o.MaxDelay = 500 * time.Millisecond
	}
	if o.MaxDelay < o.Debounce {
		o.MaxDelay = o.Debounce
	}
	if o.Interval <= 0 {
		o.Interval = 100 * time.Millisecond
	}
	if o.MaxEntries <= 0 {
		o.MaxEntries = 100000
	}
	w := &Watcher{root: root, options: o, done: make(chan struct{})}
	if len(target) > 0 {
		w.file = target[0]
	}
	if w.file != "" {
		info, err := root.Lstat(w.file)
		if err != nil {
			root.Close()
			return nil, err
		}
		if !info.Mode().IsRegular() {
			root.Close()
			return nil, errors.New("watch: target must be a regular file")
		}
	}
	if err := w.captureRootPath(); err != nil {
		root.Close()
		return nil, err
	}
	var err error
	w.previous, err = w.snapshot(ctx)
	if err == nil && w.rootGone {
		err = fs.ErrNotExist
	}
	if err != nil {
		root.Close()
		return nil, err
	}
	return w, nil
}
func (w *Watcher) snapshot(ctx context.Context) (map[string]fs.FileInfo, error) {
	result := make(map[string]fs.FileInfo)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	gone, err := w.rootPathGone()
	if err != nil {
		return nil, err
	}
	w.rootGone = gone
	if gone {
		return result, nil
	}
	if w.file != "" {
		info, err := w.root.Lstat(w.file)
		if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
			w.fileGone = true
			return result, nil
		}
		if err != nil {
			return nil, err
		}
		w.fileGone = false
		if w.options.Ignore != nil {
			ignored, err := callIgnore(w.options.Ignore, ".", false)
			if err != nil {
				return nil, err
			}
			if ignored {
				return result, nil
			}
		}
		result["."] = info
		return result, nil
	}
	err = fs.WalkDir(w.root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		if w.options.Ignore != nil {
			ignored, err := callIgnore(w.options.Ignore, filepath.ToSlash(path), d.IsDir())
			if err != nil {
				return err
			}
			if ignored {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
		}
		if path == "." {
			return nil
		}
		if len(result) >= w.options.MaxEntries {
			return ErrLimit
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
	return w.scanAt(ctx, time.Time{}, false)
}
func (w *Watcher) scanAt(ctx context.Context, at time.Time, debounce bool) ([]Event, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil, ErrClosed
	}
	if w.ended {
		return nil, io.EOF
	}
	current, err := w.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	// Revalidate after scanning too: do not publish changes from a retained
	// directory handle if the named path disappeared during the scan.
	gone, err := w.rootPathGone()
	if err != nil {
		return nil, err
	}
	w.rootGone = w.rootGone || gone
	if w.fileGone || w.rootGone {
		var events []Event
		if w.rootGone {
			events = []Event{{Op: "remove", Path: ".", IsDir: w.file == ""}}
		} else {
			events, err = snapshotDiff(ctx, w.previous, current)
		}
		if err != nil {
			return nil, err
		}
		w.previous = current
		w.observed = nil
		w.pending = false
		w.ended = true
		close(w.done)
		w.root.Close()
		return events, nil
	}
	if debounce && w.options.Debounce > 0 {
		observed := w.observed
		if observed == nil {
			observed = w.previous
		}
		changes, err := snapshotDiff(ctx, observed, current)
		if err != nil {
			return nil, err
		}
		if len(changes) > 0 {
			if !w.pending {
				w.firstChange = at
				w.pending = true
			}
			w.lastChange = at
		}
		w.observed = current
		if !w.pending || (at.Sub(w.lastChange) < w.options.Debounce && at.Sub(w.firstChange) < w.options.MaxDelay) {
			return nil, nil
		}
	}
	events, err := snapshotDiff(ctx, w.previous, current)
	if err != nil {
		return nil, err
	}
	w.previous = current
	w.observed = nil
	w.pending = false
	return events, nil
}
func snapshotDiff(ctx context.Context, previous, now map[string]fs.FileInfo) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	removed := make(map[string]fs.FileInfo)
	added := make(map[string]fs.FileInfo)
	var events []Event
	for path, old := range previous {
		cur, ok := now[path]
		if !ok {
			removed[path] = old
			continue
		}
		identical := sameEntry(old, cur)
		if !identical && !(old.Mode().IsRegular() && cur.Mode().IsRegular()) {
			removed[path] = old
			added[path] = cur
		} else if !cur.IsDir() && (!identical || contentChanged(old, cur)) {
			events = append(events, Event{Op: "write", Path: path})
		}
	}
	for path, cur := range now {
		if _, ok := previous[path]; !ok {
			added[path] = cur
		}
	}
	var index map[identity]*identityCounts
	if len(removed) > 0 && len(added) > 0 {
		var err error
		index, err = identityIndex(ctx, previous, now, added)
		if err != nil {
			return nil, err
		}
	}
	// Only infer a rename for an unambiguous identity match (hard links are ambiguous).
	for oldPath, old := range removed {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		match := ""
		if index != nil {
			match = uniqueDestination(old, index)
		} else if len(added) > 0 {
			match = compareDestination(old, previous, now, added)
		}
		_, sourceReplaced := now[oldPath]
		_, targetExisted := previous[match]
		if match != "" && !sourceReplaced && !targetExisted {
			events = append(events, Event{Op: "rename", Path: match, OldPath: oldPath, IsDir: old.IsDir()})
			if !added[match].IsDir() && contentChanged(old, added[match]) {
				events = append(events, Event{Op: "write", Path: match})
			}
			delete(added, match)
		} else {
			events = append(events, Event{Op: "remove", Path: oldPath, IsDir: old.IsDir()})
		}
	}
	for path, info := range added {
		events = append(events, Event{Op: "create", Path: path, IsDir: info.IsDir()})
	}
	// Work from the complete set of directory moves so map iteration order
	// cannot decide whether a child rename is redundant.
	movedDirectories := make(map[string]string)
	for _, event := range events {
		if event.Op == "rename" && event.IsDir {
			movedDirectories[event.OldPath] = event.Path
		}
	}
	filtered := events[:0]
	for _, event := range events {
		inherited := false
		if event.Op == "rename" {
			for ancestor := path.Dir(event.OldPath); ancestor != "."; ancestor = path.Dir(ancestor) {
				if destination, ok := movedDirectories[ancestor]; ok {
					suffix := event.OldPath[len(ancestor):]
					if event.Path == destination+suffix {
						inherited = true
						break
					}
				}
			}
		}
		if !inherited {
			filtered = append(filtered, event)
		}
	}
	events = filtered
	sort.Slice(events, func(i, j int) bool {
		if events[i].Path == events[j].Path {
			return eventOrder(events[i].Op) < eventOrder(events[j].Op)
		}
		return events[i].Path < events[j].Path
	})
	return events, nil
}

// Next blocks only the caller. Close and context cancellation wake it immediately.
func (w *Watcher) Next(ctx context.Context) ([]Event, error) {
	timer := time.NewTimer(w.options.Interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-w.done:
			w.mu.Lock()
			ended := w.ended
			w.mu.Unlock()
			if ended {
				return nil, io.EOF
			}
			return nil, ErrClosed
		case <-timer.C:
			events, err := w.scanAt(ctx, time.Now(), true)
			if err != nil || len(events) > 0 {
				return events, err
			}
			timer.Reset(w.nextScanDelay(time.Now()))
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
	if w.ended {
		return nil
	}
	close(w.done)
	return w.root.Close()
}

// A failed application filter must not leak the newly opened directory handle
// or replace the previous successful snapshot.
func callIgnore(fn func(string, bool) bool, path string, isDir bool) (ignored bool, err error) {
	defer func() {
		if recover() != nil {
			ignored = false
			err = errors.New("watch: ignore callback panicked")
		}
	}()
	return fn(path, isDir), nil
}

func sameEntry(a, b fs.FileInfo) bool { return a.Mode().Type() == b.Mode().Type() && os.SameFile(a, b) }
func contentChanged(a, b fs.FileInfo) bool {
	return a.Size() != b.Size() || !a.ModTime().Equal(b.ModTime())
}
func eventOrder(op string) int {
	switch op {
	case "remove":
		return 0
	case "create":
		return 1
	case "rename":
		return 2
	default:
		return 3
	}
}

// The quiet deadline may be earlier than the ordinary polling interval.
func (w *Watcher) nextScanDelay(at time.Time) time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	delay := w.options.Interval
	if w.pending {
		for _, deadline := range []time.Time{w.lastChange.Add(w.options.Debounce), w.firstChange.Add(w.options.MaxDelay)} {
			if remaining := deadline.Sub(at); remaining < delay {
				delay = remaining
			}
		}
	}
	if delay < 0 {
		return 0
	}
	return delay
}

// Application paths may be files or directories. Resolve an application-chosen
// symlink once; subsequent file observations never follow replacement symlinks.
func openTarget(name string) (*os.Root, string, error) {
	canonical, err := filepath.EvalSymlinks(name)
	if err != nil {
		return nil, "", err
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return nil, "", err
	}
	if info.IsDir() {
		root, err := os.OpenRoot(canonical)
		return root, "", err
	}
	if !info.Mode().IsRegular() {
		return nil, "", errors.ErrUnsupported
	}
	root, err := os.OpenRoot(filepath.Dir(canonical))
	return root, filepath.Base(canonical), err
}

// Retain both the path and handle identity. A retained directory descriptor can
// remain usable after rename/deletion, but the requested watch path no longer is.
func (w *Watcher) captureRootPath() error {
	// openTarget has already canonicalized the application path. Resolving it
	// again here would silently adopt a link installed after authorization.
	var err error
	w.rootPath, err = filepath.Abs(w.root.Name())
	if err != nil {
		return err
	}
	if _, err = os.Lstat(w.rootPath); err != nil {
		return err
	}
	valid, err := ordinaryDirectoryPath(w.rootPath)
	if err != nil {
		return err
	}
	if !valid {
		return errors.New("watch: directory path changed while opening")
	}
	handle, err := w.root.Open(".")
	if err != nil {
		return err
	}
	w.rootIdentity, err = handle.Stat()
	handle.Close()
	if err != nil {
		return err
	}
	gone, err := w.rootPathGone()
	if err != nil {
		return err
	}
	if gone {
		return errors.New("watch: directory changed while opening")
	}
	return nil
}
func (w *Watcher) rootPathGone() (bool, error) {
	info, err := os.Lstat(w.rootPath)
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return !info.IsDir() || !sameEntry(w.rootIdentity, info), nil
}

// A canonical watch path must stay an ordinary directory at every component.
// Even a link back to the original inode changes the authorized path.
func ordinaryDirectoryPath(name string) (bool, error) {
	for {
		info, err := os.Lstat(name)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !info.IsDir() {
			return false, nil
		}
		parent := filepath.Dir(name)
		if parent == name {
			return true, nil
		}
		name = parent
	}
}
