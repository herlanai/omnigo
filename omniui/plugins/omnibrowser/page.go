// Package omnibrowser implements a pure-Go headless document engine.
// It does not launch Chromium. See README.md for the supported Web API subset.
package omnibrowser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andybalholm/cascadia"
	"github.com/dop251/goja"
	"golang.org/x/net/html"
)

var ErrClosed = errors.New("omnibrowser: page closed")

// Options are copied at construction. Zero limits select bounded defaults.
type Options struct {
	Timeout             time.Duration
	MaxBytes            int64
	MaxNodes            int
	AllowPrivateNetwork bool
	// StrictScripts makes script exceptions fail loading; otherwise they remain in Snapshot.ScriptErrors.
	StrictScripts bool
}

func (o Options) defaults() Options {
	if o.Timeout <= 0 {
		o.Timeout = 2 * time.Second
	}
	if o.MaxBytes <= 0 {
		o.MaxBytes = 2 << 20
	}
	if o.MaxNodes <= 0 {
		o.MaxNodes = 50000
	}
	return o
}

// Page serializes operations. Close cancels an active navigation or script.
// Evaluations share a JS global environment until SetContent or Navigate.
var pageIDs atomic.Uint64

type Page struct {
	currentScript          *html.Node
	images                 map[string][]byte
	instance               uint64
	sharedNetwork          bool
	realmCancels           []context.CancelFunc
	handles                map[string]goja.Value
	generation, nextHandle int
	activeCtx              context.Context
	timers                 map[int]scheduled
	nextTimer              int
	storage                map[string]map[string]string
	mu                     *sync.Mutex
	options                Options
	life                   context.Context
	cancel                 context.CancelFunc
	closed                 bool
	root                   *html.Node
	url                    string
	vm                     *goja.Runtime
	objects                map[*html.Node]*nodeObject
	nodes                  *int
	logs                   []string
	scriptErrors           []string
	client                 *network
}

func New(options Options) *Page {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Page{instance: pageIDs.Add(1), mu: &sync.Mutex{}, options: options.defaults(), life: ctx, cancel: cancel, url: "about:blank"}
	p.client = newNetwork(p.options)
	return p
}
func (p *Page) Close() error {
	p.cancel()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	p.images = nil
	p.handles = nil
	p.timers = nil
	p.storage = nil
	p.realmCancels = nil
	p.root = nil
	p.vm = nil
	p.objects = nil
	p.logs = nil
	p.scriptErrors = nil
	if !p.sharedNetwork {
		p.client.close()
	}
	return nil
}
func (p *Page) operation(ctx context.Context, fn func(context.Context) error) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, p.options.Timeout)
	defer cancel()
	stop := context.AfterFunc(p.life, cancel)
	defer stop()
	if err := p.life.Err(); err != nil {
		return ErrClosed
	}
	return fn(ctx)
}
func parse(source string, limit int) (*html.Node, int, error) {
	root, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return nil, 0, err
	}
	count := 0
	stack := []*html.Node{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		if count > limit {
			return nil, 0, errors.New("omnibrowser: DOM node limit exceeded")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			stack = append(stack, c)
		}
	}
	return root, count, nil
}
func (p *Page) load(ctx context.Context, source, address string, external bool) error {
	if int64(len(source)) > p.options.MaxBytes {
		return errors.New("omnibrowser: document byte limit exceeded")
	}
	root, count, err := parse(source, p.options.MaxNodes)
	if err != nil {
		return err
	}
	if p.sharedNetwork {
		return errors.New("cannot navigate an isolated realm")
	}
	for _, cancel := range p.realmCancels {
		cancel()
	}
	p.realmCancels = nil
	p.images = nil
	p.root, p.url = root, address
	p.nodes = &count
	p.logs = nil
	p.scriptErrors = nil
	p.objects = make(map[*html.Node]*nodeObject)
	p.vm = goja.New()
	p.handles = map[string]goja.Value{}
	p.generation++
	p.nextHandle = 0
	p.install()
	if external {
		if err := p.loadStyles(ctx); err != nil {
			return err
		}
	}
	scripts, _ := cascadia.Compile("script")
	list := cascadia.QueryAll(root, scripts)
	if len(list) > 64 {
		return errors.New("omnibrowser: script count limit exceeded")
	}
	for _, n := range list {
		kind := attr(n, "type")
		if kind != "" && kind != "text/javascript" && kind != "application/javascript" {
			continue
		}
		script := textContent(n)
		if src := attr(n, "src"); src != "" {
			if !external {
				p.log("External script skipped by SetContent: " + src)
				continue
			}
			base, _ := url.Parse(address)
			reference, parseErr := url.Parse(src)
			if parseErr != nil {
				return parseErr
			}
			resolved := base.ResolveReference(reference)
			if resolved.Scheme != base.Scheme || resolved.Host != base.Host {
				if p.options.StrictScripts {
					return errors.New("omnibrowser: cross-origin external script unsupported")
				}
				p.scriptErrors = append(p.scriptErrors, "Cross-origin script skipped: "+resolved.String())
				continue
			}
			var fetchErr error
			script, _, fetchErr = p.client.fetch(resourceContext(ctx, "Script"), resolved.String(), false)
			if fetchErr != nil {
				return fetchErr
			}
		}
		p.currentScript = n
		_, err = p.run(ctx, func() (goja.Value, error) { return p.vm.RunString(script) })
		p.currentScript = nil
		if err != nil {
			if ctx.Err() != nil || p.options.StrictScripts {
				return fmt.Errorf("script: %w", err)
			}
			p.scriptErrors = append(p.scriptErrors, err.Error())
		}
	}
	p.currentScript = nil
	_, err = p.run(ctx, func() (goja.Value, error) {
		// Drive callbacks scheduled during document scripts without waiting forever
		// for recurring timers. Promise evaluation retains its separate deadline.
		for i := 0; i < 64 && len(p.timers) > 0; i++ {
			id := 0
			var next scheduled
			for key, timer := range p.timers {
				if id == 0 || timer.at.Before(next.at) {
					id = key
					next = timer
				}
			}
			if time.Until(next.at) > 20*time.Millisecond {
				break
			}
			if delay := time.Until(next.at); delay > 0 {
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return nil, ctx.Err()
				case <-timer.C:
				}
			}
			delete(p.timers, id)
			if _, err := next.fn(goja.Undefined(), next.args...); err != nil {
				p.scriptErrors = append(p.scriptErrors, err.Error())
			}
		}
		return goja.Undefined(), p.fire(p.root, "DOMContentLoaded")
	})
	if err != nil && ctx.Err() == nil && !p.options.StrictScripts {
		p.scriptErrors = append(p.scriptErrors, err.Error())
		return nil
	}
	if err == nil && external {
		err = p.loadImages(ctx)
	}
	return err
}

// SetContent loads inline HTML and classic inline scripts, without fetching resources.
func (p *Page) SetContent(ctx context.Context, source string) error {
	return p.operation(ctx, func(ctx context.Context) error { return p.load(ctx, source, "about:blank", false) })
}
func (p *Page) run(ctx context.Context, fn func() (goja.Value, error)) (goja.Value, error) {
	vm := p.vm
	p.activeCtx = ctx
	defer func() { p.activeCtx = nil }()
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { vm.Interrupt(ctx.Err()); close(done) })
	defer func() {
		if !stop() {
			<-done
		}
		vm.ClearInterrupt()
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value, err := fn()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return value, err
}

// Eval returns a detached JSON result. Undefined becomes null; circular/non-JSON results error.
func (p *Page) Eval(ctx context.Context, script string) (json.RawMessage, error) {
	var out json.RawMessage
	err := p.operation(ctx, func(ctx context.Context) error {
		if p.vm == nil {
			return errors.New("omnibrowser: load a document first")
		}
		if int64(len(script)) > p.options.MaxBytes {
			return errors.New("omnibrowser: script byte limit exceeded")
		}
		_, err := p.run(ctx, func() (goja.Value, error) {
			v, err := p.vm.RunString(script)
			if err != nil {
				return nil, err
			}
			v, err = p.await(ctx, v)
			if err != nil {
				return nil, err
			}
			stringify, _ := goja.AssertFunction(p.vm.Get("JSON").ToObject(p.vm).Get("stringify"))
			encoded, err := stringify(goja.Undefined(), v)
			if err != nil {
				return nil, err
			}
			if goja.IsUndefined(encoded) {
				out = json.RawMessage("null")
			} else {
				out = json.RawMessage(encoded.String())
			}
			return v, nil
		})
		return err
	})
	return out, err
}

type Match struct {
	Tag        string            `json:"tag"`
	Text       string            `json:"text"`
	Attributes map[string]string `json:"attributes"`
}

func (p *Page) Query(ctx context.Context, selector string) ([]Match, error) {
	var out []Match
	err := p.operation(ctx, func(context.Context) error {
		matcher, err := cascadia.Compile(selector)
		if err != nil {
			return err
		}
		if p.root == nil {
			return errors.New("omnibrowser: no document")
		}
		for _, n := range cascadia.QueryAll(p.root, matcher) {
			m := Match{Tag: n.Data, Text: textContent(n), Attributes: map[string]string{}}
			for _, a := range n.Attr {
				m.Attributes[a.Key] = a.Val
			}
			out = append(out, m)
		}
		return nil
	})
	return out, err
}

// Snapshot is an owned copy suitable for rendering or moving to a UI goroutine.
type Snapshot struct {
	Images                 map[string][]byte
	ScriptErrors           []string
	URL, Title, HTML, Text string
	Logs                   []string
}

func (p *Page) Snapshot(ctx context.Context) (Snapshot, error) {
	var result Snapshot
	err := p.operation(ctx, func(context.Context) error {
		if p.root == nil {
			return errors.New("omnibrowser: no document")
		}
		result.Images = map[string][]byte{}
		for key, data := range p.images {
			result.Images[key] = append([]byte(nil), data...)
		}
		result.URL = p.url
		result.HTML = outerHTML(p.root)
		result.Text = visibleText(p.root)
		result.Logs = append([]string(nil), p.logs...)
		result.ScriptErrors = append([]string(nil), p.scriptErrors...)
		title := queryOne(p.root, "title")
		if title != nil {
			result.Title = textContent(title)
		}
		return nil
	})
	return result, err
}
func (p *Page) Click(ctx context.Context, selector string) error {
	return p.operation(ctx, func(ctx context.Context) error {
		n, err := p.selectOne(selector)
		if err != nil {
			return err
		}
		if hasAttr(n, "disabled") {
			return errors.New("omnibrowser: element disabled")
		}
		_, err = p.run(ctx, func() (goja.Value, error) { return goja.Undefined(), p.fire(n, "click") })
		return err
	})
}
func (p *Page) Fill(ctx context.Context, selector, value string) error {
	return p.operation(ctx, func(ctx context.Context) error {
		n, err := p.selectOne(selector)
		if err != nil {
			return err
		}
		if n.Data != "input" && n.Data != "textarea" && n.Data != "select" {
			return errors.New("omnibrowser: element is not an input")
		}
		if hasAttr(n, "disabled") || hasAttr(n, "readonly") {
			return errors.New("omnibrowser: input disabled or read-only")
		}
		if int64(len(value)) > p.options.MaxBytes {
			return errors.New("omnibrowser: input byte limit exceeded")
		}
		setAttr(n, "value", value)
		_, err = p.run(ctx, func() (goja.Value, error) { return goja.Undefined(), p.fire(n, "input") })
		return err
	})
}
func (p *Page) selectOne(selector string) (*html.Node, error) {
	if p.root == nil {
		return nil, errors.New("omnibrowser: no document")
	}
	s, err := cascadia.Compile(selector)
	if err != nil {
		return nil, err
	}
	n := cascadia.Query(p.root, s)
	if n == nil {
		return nil, fmt.Errorf("omnibrowser: selector %q did not match", selector)
	}
	return n, nil
}
func (p *Page) log(s string) {
	if len(p.logs) < 100 {
		if len(s) > 2048 {
			s = s[:2048]
		}
		p.logs = append(p.logs, s)
	}
}
