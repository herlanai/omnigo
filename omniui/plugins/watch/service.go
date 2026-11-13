package watch

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/herlanai/omnigo/omniweb"
)

type Root struct {
	// Ignore always receives paths relative to this named root, even when the
	// request watches a subdirectory. Callbacks may run concurrently.
	Ignore func(path string, isDir bool) bool
	Path   string
	// Recursive permits watching child directories and descendant files as well
	// as requesting recursive scans. Without it, only this root and direct files
	// can be selected by a page.
	Recursive bool
}
type Request struct {
	DebounceMS int    `json:"debounceMs,omitempty"`
	Root       string `json:"root"`
	Path       string `json:"path"`
	Recursive  bool   `json:"recursive"`
}
type Batch struct {
	Ready  bool    `json:"ready,omitempty"`
	Events []Event `json:"events"`
}
type ServiceOptions struct {
	// Resolve is called for names absent from Roots, once per request. Results
	// are not cached. It must cooperate with cancellation and concurrent calls.
	Resolve func(context.Context, string) (Root, bool)
	Roots   map[string]Root
	Allow   func(context.Context, Request) bool
	// MaxWatchesPerSource limits each server-owned HTTP host (default 8).
	// Browser tabs sharing one host share this quota. Direct Go calls use one group.
	MaxWatchesPerSource int
	MaxWatches          int
	Scan                Options
}
type namedRoot struct {
	file      string
	ignore    func(string, bool) bool
	root      *os.Root
	recursive bool
}

type reservation struct {
	ctx     context.Context
	source  *omniweb.RequestSource
	counted bool
}

// Service exposes only application-named directories over OmniWeb. It owns
// directory handles and must be closed by the application at shutdown.
type Service struct {
	mu           sync.Mutex
	roots        map[string]namedRoot
	options      ServiceOptions
	active       int
	bySource     map[*omniweb.RequestSource]int
	reservations map[*reservation]struct{}
	closed       bool
	ctx          context.Context
	cancel       context.CancelFunc
	next         func(context.Context, *Watcher) ([]Event, error)
}

var rootName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
var ErrDenied = errors.New("watch: request denied")
var ErrBusy = errors.New("watch: watch limit reached")

// ErrEncoding means an event path cannot be represented faithfully in JSON.
// Local Go watchers can still observe arbitrary filesystem path bytes.
var ErrEncoding = errors.New("watch: path is not valid UTF-8")

func NewService(o ServiceOptions) (*Service, error) {
	if o.MaxWatchesPerSource <= 0 {
		o.MaxWatchesPerSource = 8
	}
	if o.MaxWatches <= 0 {
		o.MaxWatches = 128
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{reservations: map[*reservation]struct{}{}, bySource: map[*omniweb.RequestSource]int{}, roots: map[string]namedRoot{}, options: o, ctx: ctx, cancel: cancel, next: func(ctx context.Context, w *Watcher) ([]Event, error) { return w.Next(ctx) }}
	// Do not retain the caller's mutable roots map.
	s.options.Roots = nil
	for name, r := range o.Roots {
		if !rootName.MatchString(name) || !filepath.IsAbs(r.Path) {
			s.Close()
			return nil, ErrDenied
		}
		handle, file, err := openTarget(r.Path)
		if err != nil {
			s.Close()
			return nil, err
		}
		s.roots[name] = namedRoot{root: handle, file: file, recursive: r.Recursive, ignore: r.Ignore}
	}
	return s, nil
}

// Bind registers a typed stream. Its first batch acknowledges that the initial
// snapshot is complete, allowing the page to begin editing without a start race.
// Later batches contain relative paths only. Break/close/navigation cancel it.
// The final value is an Outcome; callers must check Code even when the transport
// succeeded. An empty Code means normal completion.
func (s *Service) Bind(streams *omniweb.Streams, name, permission string) error {
	if s == nil {
		return ErrClosed
	}
	return omniweb.BindStream(streams, name, permission, func(ctx context.Context, r Request, ch *omniweb.Channel[Batch]) (Outcome, error) {
		err := s.observe(ctx, r, func(ctx context.Context, b Batch) error { return ch.SendContext(ctx, b) })
		if ctx.Err() != nil {
			return Outcome{}, nil
		}
		return outcomeFor(err), nil
	})
}
func (s *Service) observe(ctx context.Context, r Request, emit func(context.Context, Batch) error) error {
	if r.Path == "" {
		r.Path = "."
	}
	if r.DebounceMS > 60000 {
		return ErrBusy
	}
	if !rootName.MatchString(r.Root) || !fs.ValidPath(r.Path) || strings.Contains(r.Path, "\\") {
		return ErrDenied
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	local, err := filepath.Localize(r.Path)
	if err != nil {
		return ErrDenied
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrClosed
	}
	// Observe cancellation synchronously as well as through AfterFunc: admission
	// must not depend on when the cancellation callback happens to be scheduled.
	for lease := range s.reservations {
		if lease.ctx.Err() != nil {
			s.releaseSourceLocked(lease)
		}
	}
	source := omniweb.Source(ctx)
	if s.active >= s.options.MaxWatches || s.bySource[source] >= s.options.MaxWatchesPerSource {
		s.mu.Unlock()
		return ErrBusy
	}
	lease := &reservation{ctx: ctx, source: source, counted: true}
	s.reservations[lease] = struct{}{}
	s.active++
	s.bySource[source]++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.active--
		s.releaseSourceLocked(lease)
		delete(s.reservations, lease)
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	releaseQuota := context.AfterFunc(ctx, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.releaseSourceLocked(lease)
	})
	defer releaseQuota()
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	if s.ctx.Err() != nil {
		cancel()
	}
	root, release, err := s.resolveRoot(ctx, r.Root)
	if err != nil {
		return err
	}
	defer release()
	if (r.Recursive && !root.recursive) || (root.file != "" && r.Path != ".") {
		return ErrDenied
	}
	// A shallow grant cannot be expanded by watching a child directory directly.
	// This check precedes Allow: application hooks can narrow a grant, not widen it.
	if !root.recursive && root.file == "" && r.Path != "." {
		if strings.Contains(r.Path, "/") {
			return ErrDenied
		}
		info, err := root.root.Lstat(local)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return ErrDenied
		}
	}
	if s.options.Allow != nil && !s.options.Allow(ctx, r) {
		return ErrDenied
	}
	if s.ctx.Err() != nil {
		cancel()
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	// Resolve each component through owned directory handles. Page-selected
	// symlinks are forbidden even if their destinations remain inside the root.
	sub, file, err := openRequestedTarget(root, local)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	o := s.options.Scan
	o.Recursive = r.Recursive
	if r.DebounceMS < 0 {
		o.Debounce = -1
	} else if r.DebounceMS > 0 {
		o.Debounce = time.Duration(r.DebounceMS) * time.Millisecond
	}
	scanIgnore := o.Ignore
	if root.ignore != nil {
		o.Ignore = func(p string, isDir bool) bool {
			return root.ignore(path.Join(r.Path, p), isDir) || (scanIgnore != nil && scanIgnore(p, isDir))
		}
	}
	w, err := openRoot(ctx, sub, o, file)
	if err != nil {
		return err
	}
	defer w.Close()
	if err = emit(ctx, Batch{Ready: true, Events: []Event{}}); err != nil {
		return err
	}
	for {
		events, err := s.next(ctx, w)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		// Validate the whole batch before emitting anything: encoding/json replaces
		// invalid UTF-8 with U+FFFD, which could name a different real file.
		for _, event := range events {
			if !utf8.ValidString(event.Path) || !utf8.ValidString(event.OldPath) {
				return ErrEncoding
			}
		}
		if err = emit(ctx, Batch{Events: events}); err != nil {
			return err
		}
	}
}
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.cancel()
	var result error
	for _, root := range s.roots {
		result = errors.Join(result, root.root.Close())
	}
	return result
}

// Dynamic roots are scoped to a single subscription. Retaining their parent
// handle prevents later filesystem path replacement from granting a new root.
func (s *Service) resolveRoot(ctx context.Context, name string) (namedRoot, func(), error) {
	empty := namedRoot{}
	s.mu.Lock()
	root, ok := s.roots[name]
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return empty, nil, ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return empty, nil, err
	}
	if ok {
		return root, func() {}, nil
	}
	if s.options.Resolve == nil {
		return empty, nil, ErrDenied
	}
	resolved, allowed := s.options.Resolve(ctx, name)
	if err := ctx.Err(); err != nil {
		return empty, nil, err
	}
	if !allowed || !filepath.IsAbs(resolved.Path) {
		return empty, nil, ErrDenied
	}
	handle, file, err := openTarget(resolved.Path)
	if err != nil {
		return empty, nil, err
	}
	release := func() { handle.Close() }
	if err = ctx.Err(); err != nil {
		release()
		return empty, nil, err
	}
	return namedRoot{root: handle, file: file, recursive: resolved.Recursive, ignore: resolved.Ignore}, release, nil
}

// Caller holds mu. Global occupancy is retained until the handler exits.
func (s *Service) releaseSourceLocked(lease *reservation) {
	if !lease.counted {
		return
	}
	lease.counted = false
	s.bySource[lease.source]--
	if s.bySource[lease.source] == 0 {
		delete(s.bySource, lease.source)
	}
}
