// Package fetch provides opt-in streaming HTTP with explicit session lifetime.
package fetch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"
)

type Redirect uint8

const (
	Follow Redirect = iota
	Manual
	Reject
)

type Options struct {
	Client *http.Client
	// Allow runs for the initial request and every redirect, before network IO.
	Allow        func(*url.URL) bool
	MaxBodyBytes int64
}
type Request struct {
	URL, Method string
	Header      http.Header
	Body        io.Reader
	Redirect    Redirect
}
type Client struct {
	mu      sync.Mutex
	options Options
	closed  bool
	next    uint64
	active  map[uint64]context.CancelFunc
}

func New(o Options) *Client {
	if o.MaxBodyBytes <= 0 {
		o.MaxBodyBytes = 64 << 20
	}
	c := http.Client{}
	if o.Client != nil {
		c = *o.Client
	}
	o.Client = &c
	return &Client{options: o, active: make(map[uint64]context.CancelFunc)}
}

var ErrClosed = errors.New("fetch: client closed")
var ErrBodyLimit = errors.New("fetch: response body exceeds limit")

func (c *Client) allowed(u *url.URL) bool {
	return u != nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil && (c.options.Allow == nil || c.options.Allow(u))
}
func (c *Client) Do(ctx context.Context, r Request) (*http.Response, error) {
	if ctx == nil {
		return nil, errors.New("fetch: nil context")
	}
	if r.Redirect > Reject {
		return nil, errors.New("fetch: invalid redirect mode")
	}
	u, err := url.Parse(r.URL)
	if err != nil || !c.allowed(u) {
		return nil, errors.New("fetch: URL denied")
	}
	ctx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		cancel()
		return nil, ErrClosed
	}
	c.next++
	id := c.next
	c.active[id] = cancel
	c.mu.Unlock()
	var once sync.Once
	release := func() { once.Do(func() { cancel(); c.mu.Lock(); delete(c.active, id); c.mu.Unlock() }) }
	req, err := http.NewRequestWithContext(ctx, r.Method, r.URL, r.Body)
	if err != nil {
		release()
		return nil, err
	}
	req.Header = r.Header.Clone()
	client := *c.options.Client
	originalRedirect := client.CheckRedirect
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if r.Redirect == Manual {
			return http.ErrUseLastResponse
		}
		if r.Redirect == Reject {
			return errors.New("fetch: redirect rejected")
		}
		if !c.allowed(next.URL) {
			return errors.New("fetch: redirect URL denied")
		}
		if len(via) >= 10 {
			return errors.New("fetch: too many redirects")
		}
		if originalRedirect != nil {
			return originalRedirect(next, via)
		}
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		release()
		return nil, err
	}
	// Page-facing consumers must never receive cookie-setting headers.
	resp.Header = resp.Header.Clone()
	resp.Header.Del("Set-Cookie")
	resp.Header.Del("Set-Cookie2")
	if resp.ContentLength > c.options.MaxBodyBytes {
		resp.Body.Close()
		release()
		return nil, ErrBodyLimit
	}
	resp.Body = &body{source: resp.Body, remaining: c.options.MaxBodyBytes, release: release}
	return resp, nil
}
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	cancels := make([]context.CancelFunc, 0, len(c.active))
	for _, v := range c.active {
		cancels = append(cancels, v)
	}
	c.active = make(map[uint64]context.CancelFunc)
	c.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	return nil
}

type body struct {
	source    io.ReadCloser
	remaining int64
	release   func()
	once      sync.Once
}

func (b *body) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if b.remaining == 0 {
		var probe [1]byte
		n, err := b.source.Read(probe[:])
		if n > 0 {
			err = ErrBodyLimit
		}
		if err != nil {
			b.Close()
		}
		return 0, err
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.source.Read(p)
	b.remaining -= int64(n)
	if err != nil {
		b.Close()
	}
	return n, err
}
func (b *body) Close() error {
	var err error
	b.once.Do(func() { err = b.source.Close(); b.release() })
	return err
}
