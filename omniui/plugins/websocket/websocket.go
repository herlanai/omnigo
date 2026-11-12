// Package websocket provides bounded, opt-in WebSocket connections. Incoming
// data is read on demand; no unbounded background receive queue is allocated.
package websocket

import (
	"context"
	"errors"
	ws "github.com/coder/websocket"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"unicode/utf8"
)

type MessageType int

const (
	Text   MessageType = 1
	Binary MessageType = 2
)

type Options struct {
	Header          http.Header
	Protocols       []string
	Client          *http.Client
	MaxMessageBytes int64
	Allow           func(*url.URL) bool
}
type Conn struct {
	raw      *ws.Conn
	limit    int64
	write    sync.Mutex
	buffered atomic.Int64
	stop     func() bool
}

func Dial(ctx context.Context, address string, o Options) (*Conn, error) {
	u, e := url.Parse(address)
	if e != nil || u.Host == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "ws" && u.Scheme != "wss") || (o.Allow != nil && !o.Allow(u)) {
		return nil, errors.New("websocket: URL denied")
	}
	client := http.Client{}
	if o.Client != nil {
		client = *o.Client
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("websocket: redirects disabled") }
	raw, _, e := ws.Dial(ctx, address, &ws.DialOptions{HTTPClient: &client, HTTPHeader: o.Header.Clone(), Subprotocols: append([]string(nil), o.Protocols...), CompressionMode: ws.CompressionDisabled})
	if e != nil {
		return nil, e
	}
	if o.MaxMessageBytes <= 0 {
		o.MaxMessageBytes = 8 << 20
	}
	raw.SetReadLimit(o.MaxMessageBytes)
	c := &Conn{raw: raw, limit: o.MaxMessageBytes}
	c.stop = context.AfterFunc(ctx, func() { raw.CloseNow() })
	return c, nil
}
func (c *Conn) Protocol() string      { return c.raw.Subprotocol() }
func (c *Conn) BufferedAmount() int64 { return c.buffered.Load() }
func (c *Conn) Send(ctx context.Context, kind MessageType, data []byte) error {
	if kind != Text && kind != Binary {
		return errors.New("websocket: invalid message type")
	}
	if int64(len(data)) > c.limit {
		return errors.New("websocket: message limit exceeded")
	}
	if kind == Text && !utf8.Valid(data) {
		return errors.New("websocket: invalid UTF-8")
	}
	// At most one message is retained by this API. Concurrent callers wait before
	// copying or handing bytes to the transport; the caller owns data until return.
	c.write.Lock()
	defer c.write.Unlock()
	c.buffered.Store(int64(len(data)))
	defer c.buffered.Store(0)
	return c.raw.Write(ctx, ws.MessageType(kind), data)
}
func (c *Conn) Receive(ctx context.Context) (MessageType, []byte, error) {
	kind, data, e := c.raw.Read(ctx)
	return MessageType(kind), data, e
}
func (c *Conn) Close(code int, reason string) error {
	defer c.stop()
	return c.raw.Close(ws.StatusCode(code), reason)
}
func (c *Conn) Abort() error  { c.stop(); return c.raw.CloseNow() }
func CloseCode(err error) int { return int(ws.CloseStatus(err)) }
