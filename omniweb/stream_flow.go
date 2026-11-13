package omniweb

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
)

const streamWindow = 1 << 20

// Account for payload plus framing so tiny values cannot create unbounded metadata.
type streamFlow struct {
	mu         sync.Mutex
	sent, ack  uint64
	bytes      int
	pending    map[uint64]int
	changed    chan struct{}
	permission string
}

func newStreamFlow(permission string) *streamFlow {
	return &streamFlow{pending: map[uint64]int{}, changed: make(chan struct{}), permission: permission}
}
func (f *streamFlow) reserve(ctx context.Context, size int) (uint64, error) {
	if size <= 0 || size > streamWindow {
		return 0, ErrTooLarge
	}
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		f.mu.Lock()
		if f.bytes+size <= streamWindow {
			if f.sent >= 1<<53-1 {
				f.mu.Unlock()
				return 0, ErrTooLarge
			}
			f.sent++
			seq := f.sent
			f.pending[seq] = size
			f.bytes += size
			f.mu.Unlock()
			return seq, nil
		}
		wait := f.changed
		f.mu.Unlock()
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-wait:
		}
	}
}
func (f *streamFlow) confirm(seq uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if seq > f.sent {
		return ErrInvalidRequest
	}
	if seq <= f.ack {
		return nil
	}
	for n := f.ack + 1; n <= seq; n++ {
		f.bytes -= f.pending[n]
		delete(f.pending, n)
	}
	f.ack = seq
	close(f.changed)
	f.changed = make(chan struct{})
	return nil
}
func (h *HTTP) streamAck(w http.ResponseWriter, r *http.Request, raw []byte) {
	fail := func(status int) {
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(Fault{Code: "stream_ack", Message: http.StatusText(status)})
	}
	var input struct {
		Version int    `json:"v"`
		Token   string `json:"token"`
		Seq     uint64 `json:"seq"`
	}
	if decode(raw, &input) != nil || input.Version != Version || len(input.Token) != 64 {
		fail(400)
		return
	}
	s := h.options.Streams
	if s == nil {
		fail(404)
		return
	}
	s.mu.RLock()
	flow := s.sessions[input.Token]
	s.mu.RUnlock()
	if flow == nil {
		fail(http.StatusGone)
		return
	}
	allowed := flow.permission == ""
	for _, g := range h.options.Grants {
		if g == flow.permission {
			allowed = true
		}
	}
	if !allowed {
		fail(403)
		return
	}
	if err := flow.confirm(input.Seq); err != nil {
		fail(400)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
