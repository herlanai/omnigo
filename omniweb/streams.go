package omniweb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"
)

var ErrChannelClosed = errors.New("omniweb: channel closed")

type streamFrame struct {
	Ask     bool            `json:"ask,omitempty"`
	Token   string          `json:"token,omitempty"`
	Seq     uint64          `json:"seq,omitempty"`
	Version int             `json:"v"`
	ID      string          `json:"id"`
	Kind    string          `json:"kind"`
	Data    json.RawMessage `json:"data,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Fault          `json:"error,omitempty"`
}
type streamEntry struct {
	permission string
	prepare    func(json.RawMessage) (func(context.Context, *streamWriter) (json.RawMessage, error), error)
}

// Streams registers request-scoped typed streaming methods. Handlers run off the
// UI thread and must cooperate with context cancellation.
type Streams struct {
	sessions map[string]*streamFlow
	mu       sync.RWMutex
	methods  map[string]streamEntry
	slots    chan struct{}
}

func NewStreams(maxConcurrent int) *Streams {
	if maxConcurrent <= 0 {
		maxConcurrent = 16
	}
	return &Streams{sessions: map[string]*streamFlow{}, methods: map[string]streamEntry{}, slots: make(chan struct{}, maxConcurrent)}
}

// Channel sends values in order. Close ends values but does not end the method;
// its final result is sent after the handler returns. Send must not run on the UI thread.
type Channel[T any] struct{ writer *streamWriter }

func (c *Channel[T]) Send(value T) error {
	if c == nil || c.writer == nil {
		return ErrChannelClosed
	}
	return c.writer.send(value)
}

// SendContext also stops waiting for the consumption window when ctx ends.
// The call's own cancellation remains effective.
func (c *Channel[T]) SendContext(ctx context.Context, value T) error {
	if c == nil || c.writer == nil {
		return ErrChannelClosed
	}
	combined, cancel := context.WithCancel(c.writer.ctx)
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	defer cancel()
	if ctx.Err() != nil {
		cancel()
	}
	return c.writer.sendContext(combined, value)
}
func (c *Channel[T]) Close() error {
	if c == nil || c.writer == nil {
		return nil
	}
	return c.writer.closeValues()
}
func BindStream[Request, Value, Result any](s *Streams, name, permission string, handler func(context.Context, Request, *Channel[Value]) (Result, error)) error {
	if s == nil || handler == nil || !methodName.MatchString(name) {
		return ErrInvalidRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.methods == nil || s.slots == nil {
		return ErrInvalidRequest
	}
	if _, exists := s.methods[name]; exists {
		return errors.New("omniweb: duplicate stream method")
	}
	s.methods[name] = streamEntry{permission: permission, prepare: func(raw json.RawMessage) (func(context.Context, *streamWriter) (json.RawMessage, error), error) {
		var input Request
		if err := decode(raw, &input); err != nil {
			return nil, ErrInvalidRequest
		}
		return func(ctx context.Context, w *streamWriter) (json.RawMessage, error) {
			result, err := handler(ctx, input, &Channel[Value]{writer: w})
			if err != nil {
				return nil, err
			}
			encoded, err := json.Marshal(result)
			if len(encoded) > 1<<20 {
				return nil, ErrTooLarge
			}
			return encoded, err
		}, nil
	}}
	return nil
}

type streamWriter struct {
	unaskedBytes int
	flow         *streamFlow
	ctx          context.Context
	write        func(streamFrame) error
	gate         chan struct{}
	closed       bool
	failure      error
}

func (w *streamWriter) lock() error { return w.lockContext(w.ctx) }
func (w *streamWriter) lockContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case w.gate <- struct{}{}:
		return nil
	}
}
func (w *streamWriter) unlock()              { <-w.gate }
func (w *streamWriter) send(value any) error { return w.sendContext(w.ctx, value) }
func (w *streamWriter) sendContext(ctx context.Context, value any) error {
	if err := w.lockContext(ctx); err != nil {
		return err
	}
	defer w.unlock()
	if w.closed {
		return ErrChannelClosed
	}
	if w.failure != nil {
		return w.failure
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(raw) > 256<<10 {
		return ErrTooLarge
	}
	seq, err := w.flow.reserve(ctx, len(raw)+256)
	if err != nil {
		return err
	}
	w.unaskedBytes += len(raw) + 256
	ask := w.unaskedBytes >= streamWindow/2
	if ask {
		w.unaskedBytes = 0
	}
	w.failure = w.write(streamFrame{Kind: "value", Data: raw, Seq: seq, Ask: ask})
	return w.failure
}
func (w *streamWriter) closeValues() error {
	if err := w.lock(); err != nil {
		return err
	}
	defer w.unlock()
	if w.closed {
		return w.failure
	}
	w.closed = true
	if w.failure == nil {
		w.failure = w.write(streamFrame{Kind: "end"})
	}
	return w.failure
}
func (h *HTTP) stream(w http.ResponseWriter, r *http.Request, raw []byte) {
	fail := func(status int, code string) {
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(Fault{Code: code, Message: http.StatusText(status)})
	}
	s := h.options.Streams
	if s == nil {
		fail(404, "not_found")
		return
	}
	var request Request
	if decode(raw, &request) != nil || validate(request) != nil {
		fail(400, "invalid_request")
		return
	}
	s.mu.RLock()
	entry, exists := s.methods[request.Method]
	s.mu.RUnlock()
	if !exists {
		fail(404, "not_found")
		return
	}
	allowed := entry.permission == ""
	for _, grant := range h.options.Grants {
		if grant == entry.permission {
			allowed = true
		}
	}
	if !allowed {
		fail(403, "forbidden")
		return
	}
	invoke, err := entry.prepare(request.Params)
	if err != nil {
		fail(400, "invalid_params")
		return
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		fail(503, "busy")
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	controller := http.NewResponseController(w)
	w.Header().Set("Content-Type", "application/x-ndjson")
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		fail(500, "internal")
		return
	}
	token := hex.EncodeToString(tokenBytes)
	flow := newStreamFlow(entry.permission)
	s.mu.Lock()
	s.sessions[token] = flow
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.sessions, token); s.mu.Unlock() }()
	writer := &streamWriter{flow: flow, ctx: ctx, gate: make(chan struct{}, 1)}
	writer.write = func(frame streamFrame) error {
		frame.Version = Version
		frame.ID = request.ID
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := controller.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			cancel()
			return err
		}
		if err := json.NewEncoder(w).Encode(frame); err != nil {
			cancel()
			return err
		}
		if err := controller.Flush(); err != nil {
			cancel()
			return err
		}
		return nil
	}
	if err := writer.write(streamFrame{Kind: "ready", Token: token}); err != nil {
		return
	}
	result, callErr := invokeStream(ctx, writer, invoke)
	if err := writer.closeValues(); err != nil {
		return
	}
	// End the HTTP response once production ends. Waiting for the final ACK here
	// can deadlock buffering WebViews. The SDK exposes result only after it has
	// consumed preceding value frames; the outstanding tail is window-bounded.
	// Serialize the final frame with late sends/Close; all later sends see closed.
	if err := writer.lock(); err != nil {
		return
	}
	defer writer.unlock()
	frame := streamFrame{Kind: "result", Result: result}
	if callErr != nil {
		code := "internal"
		if errors.Is(callErr, ErrTooLarge) {
			code = "too_large"
		}
		frame.Result = nil
		frame.Error = &Fault{Code: code, Message: "Streaming method failed"}
	}
	_ = writer.write(frame)
}
func invokeStream(ctx context.Context, w *streamWriter, invoke func(context.Context, *streamWriter) (json.RawMessage, error)) (result json.RawMessage, err error) {
	defer func() {
		if recover() != nil {
			result = nil
			err = ErrInternal
		}
	}()
	return invoke(ctx, w)
}
