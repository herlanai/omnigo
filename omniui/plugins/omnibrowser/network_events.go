package omnibrowser

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type resourceTypeKey struct{}
type NetworkEvent struct {
	ID, Phase, URL, Method, Type, Error string
	Status                              int
	Headers                             map[string]string
	Timestamp                           float64
	Size                                int64
}
type networkLog struct {
	mu     sync.Mutex
	next   uint64
	events []NetworkEvent
	bodies map[string][]byte
	bytes  int
}

func (n *networkLog) add(e NetworkEvent) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.events) < 1024 {
		n.events = append(n.events, e)
	}
}
func (n *networkLog) body(id string, b []byte) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.bodies == nil {
		n.bodies = map[string][]byte{}
	}
	if n.bytes+len(b) > 8<<20 {
		return
	}
	n.bodies[id] = append([]byte(nil), b...)
	n.bytes += len(b)
}

type observedTransport struct {
	base  http.RoundTripper
	log   *networkLog
	limit int64
}

func (t observedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.log.mu.Lock()
	t.log.next++
	id := fmt.Sprint(t.log.next)
	t.log.mu.Unlock()
	kind, _ := r.Context().Value(resourceTypeKey{}).(string)
	if kind == "" {
		kind = "Fetch"
	}
	event := NetworkEvent{ID: id, Phase: "request", URL: r.URL.String(), Method: r.Method, Type: kind, Timestamp: float64(time.Now().UnixNano()) / 1e9, Headers: map[string]string{}}
	for k, v := range r.Header {
		if len(v) > 0 {
			event.Headers[k] = v[0]
		}
	}
	t.log.add(event)
	response, err := t.base.RoundTrip(r)
	if err != nil {
		event.Phase = "failed"
		event.Error = err.Error()
		t.log.add(event)
		return nil, err
	}
	event.Phase = "response"
	event.Status = response.StatusCode
	event.Headers = map[string]string{}
	for k, v := range response.Header {
		if len(v) > 0 {
			event.Headers[k] = v[0]
		}
	}
	t.log.add(event)
	response.Body = &observedBody{ReadCloser: response.Body, event: event, log: t.log, limit: t.limit}
	return response, nil
}

type observedBody struct {
	io.ReadCloser
	event    NetworkEvent
	log      *networkLog
	data     []byte
	limit    int64
	finished bool
	size     int64
}

func (b *observedBody) finish(err error) {
	if b.finished {
		return
	}
	b.finished = true
	b.event.Size = b.size
	b.event.Timestamp = float64(time.Now().UnixNano()) / 1e9
	if err != nil && err != io.EOF {
		b.event.Phase = "failed"
		b.event.Error = err.Error()
	} else {
		b.event.Phase = "finished"
		if b.size <= b.limit {
			b.log.body(b.event.ID, b.data)
		}
	}
	b.log.add(b.event)
}
func (b *observedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.size += int64(n)
	if int64(len(b.data))+int64(n) <= b.limit {
		b.data = append(b.data, p[:n]...)
	}
	if err != nil {
		b.finish(err)
	}
	return n, err
}
func (b *observedBody) Close() error {
	err := b.ReadCloser.Close()
	if !b.finished {
		b.finish(fmt.Errorf("response body closed before EOF"))
	}
	return err
}
func (p *Page) DrainNetworkEvents() []NetworkEvent {
	n := p.client.log
	n.mu.Lock()
	defer n.mu.Unlock()
	events := n.events
	n.events = nil
	return events
}
func (p *Page) ResponseBody(id string) (string, error) {
	n := p.client.log
	n.mu.Lock()
	defer n.mu.Unlock()
	b, ok := n.bodies[id]
	if !ok {
		return "", fmt.Errorf("response body unavailable or quota exceeded")
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
func resourceContext(ctx context.Context, kind string) context.Context {
	return context.WithValue(ctx, resourceTypeKey{}, kind)
}
