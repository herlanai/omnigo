// Package clipboard provides immutable data snapshots and optional system adapters.
package clipboard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime"
	"sort"
	"strings"
	"sync"
)

const (
	Text     = "text/plain"
	HTML     = "text/html"
	PNG      = "image/png"
	URIList  = "text/uri-list"
	Markdown = "text/markdown"
)
const MaxBytes = 64 << 20

var ErrFormat = errors.New("clipboard: format unavailable")
var ErrClosed = errors.New("clipboard: closed")
var ErrUnsupported = errors.New("clipboard: unsupported operation")

// Source advertises either copied bytes or a deferred provider. Providers must
// be short, run on the calling UI goroutine, and may not wait on UI dispatch.
type Source struct {
	Format  string
	Bytes   []byte
	Provide func() ([]byte, error)
}
type Item []Source
type cached struct {
	source Source
	once   sync.Once
	data   []byte
	err    error
}
type Data struct{ items []map[string]*cached }

func New(items ...Item) (Data, error) {
	if len(items) > 128 {
		return Data{}, errors.New("clipboard: item budget exceeded")
	}
	d := Data{items: make([]map[string]*cached, len(items))}
	total := 0
	for i, item := range items {
		if len(item) == 0 || len(item) > 64 {
			return Data{}, errors.New("clipboard: invalid format count")
		}
		d.items[i] = map[string]*cached{}
		for _, s := range item {
			f, err := format(s.Format)
			if err != nil {
				return Data{}, err
			}
			if _, ok := d.items[i][f]; ok {
				return Data{}, errors.New("clipboard: duplicate format")
			}
			if s.Provide != nil && s.Bytes != nil {
				return Data{}, errors.New("clipboard: source cannot contain bytes and provider")
			}
			total += len(s.Bytes)
			if total > MaxBytes {
				return Data{}, errors.New("clipboard: data budget exceeded")
			}
			s.Format = f
			s.Bytes = bytes.Clone(s.Bytes)
			d.items[i][f] = &cached{source: s}
		}
	}
	return d, nil
}
func format(s string) (string, error) {
	if strings.ContainsAny(s, "\x00\r\n") {
		return "", errors.New("clipboard: invalid format")
	}
	t, p, err := mime.ParseMediaType(s)
	if err != nil || len(p) != 0 || !strings.Contains(t, "/") {
		return "", errors.New("clipboard: expected MIME type without parameters")
	}
	return strings.ToLower(t), nil
}
func (d Data) Len() int { return len(d.items) }
func (d Data) Formats(item int) []string {
	if item < 0 || item >= len(d.items) {
		return nil
	}
	out := make([]string, 0, len(d.items[item]))
	for f := range d.items[item] {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}
func (d Data) Read(ctx context.Context, item int, f string) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("clipboard: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := format(f)
	if err != nil {
		return nil, err
	}
	if item < 0 || item >= len(d.items) {
		return nil, ErrFormat
	}
	v := d.items[item][f]
	if v == nil {
		return nil, ErrFormat
	}
	v.once.Do(func() {
		defer func() {
			if p := recover(); p != nil {
				v.data = nil
				v.err = fmt.Errorf("clipboard: provider panic: %v", p)
			}
		}()
		b := v.source.Bytes
		if v.source.Provide != nil {
			b, v.err = v.source.Provide()
		}
		if len(b) > MaxBytes {
			v.err = errors.New("clipboard: provider exceeds budget")
		}
		if v.err == nil {
			v.data = bytes.Clone(b)
		}
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return bytes.Clone(v.data), v.err
}

// Snapshot gives each write a new lazy-provider cache. No provider runs here.
func (d Data) Snapshot() Data {
	out := Data{items: make([]map[string]*cached, len(d.items))}
	for i, item := range d.items {
		out.items[i] = map[string]*cached{}
		for f, v := range item {
			out.items[i][f] = &cached{source: v.source}
		}
	}
	return out
}
func (d Data) materialize(ctx context.Context) ([]map[string][]byte, error) {
	out := make([]map[string][]byte, len(d.items))
	total := 0
	for i := range out {
		out[i] = map[string][]byte{}
		for _, f := range d.Formats(i) {
			b, err := d.Read(ctx, i, f)
			if err != nil {
				return nil, err
			}
			total += len(b)
			if total > MaxBytes {
				return nil, errors.New("clipboard: materialized budget exceeded")
			}
			out[i][f] = b
		}
	}
	return out, nil
}

type Options struct {
	Owner uintptr
	Name  string
}
type adapter interface {
	read([]string) ([]map[string][]byte, error)
	write([]map[string][]byte) error
	close()
}

// Clipboard and its adapters belong to the UI goroutine. Writes eagerly
// materialize providers before touching system ownership; the OS owns copies.
type Clipboard struct {
	backend adapter
	closed  bool
}

func Open(o Options) (*Clipboard, error) {
	if strings.ContainsRune(o.Name, 0) {
		return nil, errors.New("clipboard: name contains NUL")
	}
	b, err := openAdapter(o)
	if err != nil {
		return nil, err
	}
	return &Clipboard{backend: b}, nil
}
func (c *Clipboard) Write(ctx context.Context, data Data, released func()) error {
	if c.closed {
		return ErrClosed
	}
	if ctx == nil {
		return errors.New("clipboard: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	items, err := data.Snapshot().materialize(ctx)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = c.backend.write(items); err != nil {
		return err
	}
	if released != nil {
		released()
	}
	return nil
}
func (c *Clipboard) Read(ctx context.Context, formats ...string) (Data, error) {
	if c.closed {
		return Data{}, ErrClosed
	}
	if ctx == nil {
		return Data{}, errors.New("clipboard: nil context")
	}
	if err := ctx.Err(); err != nil {
		return Data{}, err
	}
	normalized := make([]string, len(formats))
	for i, f := range formats {
		var err error
		normalized[i], err = format(f)
		if err != nil {
			return Data{}, err
		}
	}
	values, err := c.backend.read(normalized)
	if err != nil {
		return Data{}, err
	}
	if err := ctx.Err(); err != nil {
		return Data{}, err
	}
	var items []Item
	total := 0
	for _, v := range values {
		var item Item
		for f, b := range v {
			total += len(b)
			if total > MaxBytes {
				return Data{}, errors.New("clipboard: native data budget exceeded")
			}
			item = append(item, Source{Format: f, Bytes: b})
		}
		if len(item) > 0 {
			items = append(items, item)
		}
	}
	if len(formats) > 0 && len(items) == 0 {
		return Data{}, ErrFormat
	}
	return New(items...)
}
func (c *Clipboard) Clear(ctx context.Context) error { return c.Write(ctx, Data{}, nil) }
func (c *Clipboard) Close() {
	if !c.closed {
		c.closed = true
		c.backend.close()
	}
}

// Flush is immediate because supported system adapters receive materialized copies.
func (c *Clipboard) Flush() error {
	if c.closed {
		return ErrClosed
	}
	return nil
}

func (c *Clipboard) ReadText(ctx context.Context) (string, error) {
	d, err := c.Read(ctx, Text)
	if err != nil {
		return "", err
	}
	b, err := d.Read(ctx, 0, Text)
	return string(b), err
}
func (c *Clipboard) WriteText(ctx context.Context, text string) error {
	d, err := New(Item{{Format: Text, Bytes: []byte(text)}})
	if err != nil {
		return err
	}
	return c.Write(ctx, d, nil)
}
