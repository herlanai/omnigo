package omnibrowser

import (
	"context"
	"errors"
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/dop251/goja"
	"golang.org/x/net/html"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser/css"
	"strings"
	"time"
)

// Locator resolves anew for each operation, so it survives document replacement.
// Actions are strict and wait for attached, visible and enabled elements.
type Locator struct {
	page           *Page
	selector, text string
	index          int
}

func (p *Page) Locator(selector string) *Locator {
	return &Locator{page: p, selector: selector, index: -1}
}
func (l *Locator) Nth(index int) *Locator          { copy := *l; copy.index = index; return &copy }
func (l *Locator) FilterText(text string) *Locator { copy := *l; copy.text = text; return &copy }
func (l *Locator) matches() ([]*html.Node, error) {
	selector, err := cascadia.Compile(l.selector)
	if err != nil {
		return nil, err
	}
	if l.page.root == nil {
		return nil, nil
	}
	all := cascadia.QueryAll(l.page.root, selector)
	out := all[:0]
	for _, n := range all {
		if l.text == "" || strings.Contains(textContent(n), l.text) {
			out = append(out, n)
		}
	}
	if l.index >= 0 {
		if l.index >= len(out) {
			return nil, nil
		}
		return out[l.index : l.index+1], nil
	}
	return out, nil
}
func (l *Locator) Count(ctx context.Context) (int, error) {
	count := 0
	err := l.page.operation(ctx, func(context.Context) error { nodes, err := l.matches(); count = len(nodes); return err })
	return count, err
}
func visible(n *html.Node, styles map[*html.Node]map[string]string) bool {
	for ; n != nil; n = n.Parent {
		style := styles[n]
		if style["display"] == "none" || style["visibility"] == "hidden" || hasAttr(n, "hidden") {
			return false
		}
	}
	return true
}
func enabled(n *html.Node) bool {
	for current := n; current != nil; current = current.Parent {
		if hasAttr(current, "disabled") && (current == n || current.Data == "fieldset") {
			return false
		}
		if attr(current, "aria-disabled") == "true" {
			return false
		}
	}
	return true
}
func (l *Locator) wait(ctx context.Context, state string, action func(context.Context, *html.Node) error) error {
	switch state {
	case "attached", "detached", "visible", "hidden", "action", "editable":
	default:
		return fmt.Errorf("invalid locator state %q", state)
	}
	ctx, cancel := context.WithTimeout(ctx, l.page.options.Timeout)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		ready := false
		err := l.page.operation(ctx, func(ctx context.Context) error {
			nodes, err := l.matches()
			if err != nil {
				return err
			}
			if len(nodes) > 1 {
				return fmt.Errorf("locator %q is ambiguous: %d matches", l.selector, len(nodes))
			}
			if state == "detached" {
				ready = len(nodes) == 0
				return nil
			}
			if len(nodes) == 0 {
				ready = state == "hidden"
				return nil
			}
			n := nodes[0]
			shown := visible(n, css.Document(l.page.root).Styles(l.page.root))
			switch state {
			case "attached":
				ready = true
			case "visible":
				ready = shown
			case "hidden":
				ready = !shown
			case "action":
				ready = shown && enabled(n)
			case "editable":
				ready = shown && enabled(n) && !hasAttr(n, "readonly")
			default:
				return fmt.Errorf("invalid locator state %q", state)
			}
			if ready && action != nil {
				return action(ctx, n)
			}
			return nil
		})
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("locator %q waiting for %s: %w", l.selector, state, ctx.Err())
		case <-l.page.life.Done():
			return ErrClosed
		case <-ticker.C:
		}
	}
}
func (l *Locator) WaitFor(ctx context.Context, state string) error { return l.wait(ctx, state, nil) }
func (l *Locator) Click(ctx context.Context) error {
	return l.wait(ctx, "action", func(ctx context.Context, n *html.Node) error {
		_, err := l.page.run(ctx, func() (goja.Value, error) { return goja.Undefined(), l.page.fire(n, "click") })
		return err
	})
}
func (l *Locator) Fill(ctx context.Context, value string) error {
	return l.wait(ctx, "editable", func(ctx context.Context, n *html.Node) error {
		if n.Data != "input" && n.Data != "textarea" {
			return errors.New("locator is not a text input")
		}
		if int64(len(value)) > l.page.options.MaxBytes {
			return errors.New("input byte limit exceeded")
		}
		setAttr(n, "value", value)
		_, err := l.page.run(ctx, func() (goja.Value, error) { return goja.Undefined(), l.page.fire(n, "input") })
		return err
	})
}
func (l *Locator) TextContent(ctx context.Context) (string, error) {
	var text string
	err := l.wait(ctx, "attached", func(_ context.Context, n *html.Node) error { text = textContent(n); return nil })
	return text, err
}
func (l *Locator) GetAttribute(ctx context.Context, name string) (string, error) {
	var value string
	err := l.wait(ctx, "attached", func(_ context.Context, n *html.Node) error { value = attr(n, name); return nil })
	return value, err
}
