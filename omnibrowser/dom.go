package omnibrowser

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/andybalholm/cascadia"
	"github.com/dop251/goja"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}
func setAttr(n *html.Node, key, value string) {
	for i := range n.Attr {
		if n.Attr[i].Key == key {
			n.Attr[i].Val = value
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: value})
}
func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}
func visibleText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "head" || hasAttr(n, "hidden")) {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
func outerHTML(n *html.Node) string { var b strings.Builder; _ = html.Render(&b, n); return b.String() }
func queryOne(n *html.Node, s string) *html.Node {
	m, err := cascadia.Compile(s)
	if err != nil {
		return nil
	}
	return cascadia.Query(n, m)
}

type nodeObject struct {
	page       *Page
	node       *html.Node
	object     *goja.Object
	properties map[string]goja.Value
	listeners  map[string][]goja.Callable
}

func (p *Page) wrap(n *html.Node) goja.Value {
	if n == nil {
		return goja.Null()
	}
	if old := p.objects[n]; old != nil {
		return old.object
	}
	o := &nodeObject{page: p, node: n, properties: map[string]goja.Value{}, listeners: map[string][]goja.Callable{}}
	o.object = p.vm.NewDynamicObject(o)
	p.objects[n] = o
	return o.object
}
func (p *Page) allocate(n *html.Node) *html.Node {
	p.nodes++
	if p.nodes > p.options.MaxNodes {
		panic(p.vm.NewTypeError("DOM node limit exceeded"))
	}
	return n
}
func (o *nodeObject) fail(message string) { panic(o.page.vm.NewTypeError(message)) }
func (o *nodeObject) bounded(s string) string {
	if int64(len(s)) > o.page.options.MaxBytes {
		o.fail("DOM string byte limit exceeded")
	}
	return s
}
func (o *nodeObject) Get(key string) goja.Value {
	p, n := o.page, o.node
	vm := p.vm
	if value, ok := o.properties[key]; ok {
		return value
	}
	fn := func(f func(goja.FunctionCall) goja.Value) goja.Value { return vm.ToValue(f) }
	switch key {
	case "nodeType":
		return vm.ToValue(map[html.NodeType]int{html.DocumentNode: 9, html.ElementNode: 1, html.TextNode: 3, html.CommentNode: 8}[n.Type])
	case "tagName":
		if n.Type == html.ElementNode {
			return vm.ToValue(strings.ToUpper(n.Data))
		}
		return goja.Undefined()
	case "textContent":
		return vm.ToValue(textContent(n))
	case "innerText":
		return vm.ToValue(visibleText(n))
	case "innerHTML":
		var b strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			_ = html.Render(&b, c)
		}
		return vm.ToValue(b.String())
	case "outerHTML":
		return vm.ToValue(outerHTML(n))
	case "id", "value", "name", "type":
		return vm.ToValue(attr(n, key))
	case "className":
		return vm.ToValue(attr(n, "class"))
	case "disabled", "checked":
		return vm.ToValue(hasAttr(n, key))
	case "href", "src":
		u, err := url.Parse(attr(n, key))
		base, _ := url.Parse(p.url)
		if err != nil || base == nil {
			return vm.ToValue("")
		}
		return vm.ToValue(base.ResolveReference(u).String())
	case "parentNode", "parentElement":
		return p.wrap(n.Parent)
	case "firstChild":
		return p.wrap(n.FirstChild)
	case "children", "childNodes":
		a := []interface{}{}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if key == "childNodes" || c.Type == html.ElementNode {
				a = append(a, p.wrap(c))
			}
		}
		return vm.NewArray(a...)
	case "querySelector", "querySelectorAll":
		return fn(func(call goja.FunctionCall) goja.Value {
			selector, err := cascadia.Compile(call.Argument(0).String())
			if err != nil {
				o.fail(err.Error())
			}
			if key == "querySelector" {
				return p.wrap(cascadia.Query(n, selector))
			}
			a := []interface{}{}
			for _, c := range cascadia.QueryAll(n, selector) {
				a = append(a, p.wrap(c))
			}
			return vm.NewArray(a...)
		})
	case "getAttribute", "hasAttribute", "setAttribute", "removeAttribute":
		return fn(func(call goja.FunctionCall) goja.Value {
			name := strings.ToLower(call.Argument(0).String())
			switch key {
			case "getAttribute":
				if !hasAttr(n, name) {
					return goja.Null()
				}
				return vm.ToValue(attr(n, name))
			case "hasAttribute":
				return vm.ToValue(hasAttr(n, name))
			case "setAttribute":
				setAttr(n, name, o.bounded(call.Argument(1).String()))
			case "removeAttribute":
				for i, a := range n.Attr {
					if a.Key == name {
						n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
						break
					}
				}
			}
			return goja.Undefined()
		})
	case "appendChild", "removeChild":
		return fn(func(call goja.FunctionCall) goja.Value {
			obj, ok := call.Argument(0).Export().(*nodeObject)
			if !ok || obj.page != p {
				o.fail("expected a node from this document")
			}
			child := obj.node
			if key == "removeChild" {
				if child.Parent != n {
					o.fail("not a child")
				}
				n.RemoveChild(child)
			} else {
				for parent := n; parent != nil; parent = parent.Parent {
					if parent == child {
						o.fail("cyclic DOM insertion")
					}
				}
				if n.Type != html.ElementNode && n.Type != html.DocumentNode {
					o.fail("node cannot have children")
				}
				if child.Parent != nil {
					child.Parent.RemoveChild(child)
				}
				n.AppendChild(child)
			}
			return obj.object
		})
	case "insertAdjacentHTML":
		return fn(func(call goja.FunctionCall) goja.Value {
			position := strings.ToLower(call.Argument(0).String())
			source := o.bounded(call.Argument(1).String())
			parent, before := n, (*html.Node)(nil)
			switch position {
			case "beforebegin":
				parent, before = n.Parent, n
			case "afterbegin":
				before = n.FirstChild
			case "beforeend":
			case "afterend":
				parent, before = n.Parent, n.NextSibling
			default:
				o.fail("invalid insertion position")
			}
			if parent == nil || parent.Type != html.ElementNode {
				o.fail("insertion requires an element parent")
			}
			nodes, err := html.ParseFragment(strings.NewReader(source), parent)
			if err != nil {
				o.fail(err.Error())
			}
			count := 0
			stack := append([]*html.Node(nil), nodes...)
			for len(stack) > 0 {
				child := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				count++
				for c := child.FirstChild; c != nil; c = c.NextSibling {
					stack = append(stack, c)
				}
			}
			if p.nodes+count > p.options.MaxNodes {
				o.fail("DOM node limit exceeded")
			}
			p.nodes += count
			for _, child := range nodes {
				parent.InsertBefore(child, before)
			}
			return goja.Undefined()
		})
	case "remove":
		return fn(func(goja.FunctionCall) goja.Value {
			if n.Parent != nil {
				n.Parent.RemoveChild(n)
			}
			return goja.Undefined()
		})
	case "addEventListener":
		return fn(func(call goja.FunctionCall) goja.Value {
			event := call.Argument(0).String()
			callback, ok := goja.AssertFunction(call.Argument(1))
			if !ok {
				o.fail("listener must be callable")
			}
			if len(o.listeners[event]) >= 100 {
				o.fail("listener limit exceeded")
			}
			o.listeners[event] = append(o.listeners[event], callback)
			return goja.Undefined()
		})
	case "click":
		return fn(func(goja.FunctionCall) goja.Value {
			if hasAttr(n, "disabled") {
				return goja.Undefined()
			}
			if err := p.fire(n, "click"); err != nil {
				panic(vm.NewGoError(err))
			}
			return goja.Undefined()
		})
	}
	if n.Type == html.DocumentNode {
		switch key {
		case "body", "head":
			return p.wrap(queryOne(n, key))
		case "documentElement":
			return p.wrap(queryOne(n, "html"))
		case "title":
			title := queryOne(n, "title")
			if title == nil {
				return vm.ToValue("")
			}
			return vm.ToValue(textContent(title))
		case "URL":
			return vm.ToValue(p.url)
		case "getElementById":
			return fn(func(call goja.FunctionCall) goja.Value {
				var found *html.Node
				var walk func(*html.Node)
				id := call.Argument(0).String()
				walk = func(node *html.Node) {
					if found != nil {
						return
					}
					if node.Type == html.ElementNode && attr(node, "id") == id {
						found = node
						return
					}
					for c := node.FirstChild; c != nil; c = c.NextSibling {
						walk(c)
					}
				}
				walk(n)
				return p.wrap(found)
			})
		case "createElement", "createTextNode":
			return fn(func(call goja.FunctionCall) goja.Value {
				value := o.bounded(call.Argument(0).String())
				if key == "createTextNode" {
					return p.wrap(p.allocate(&html.Node{Type: html.TextNode, Data: value}))
				}
				value = strings.ToLower(value)
				if strings.ContainsAny(value, "<> /\t\r\n") || value == "" {
					o.fail("invalid tag")
				}
				return p.wrap(p.allocate(&html.Node{Type: html.ElementNode, Data: value, DataAtom: atom.Lookup([]byte(value))}))
			})
		}
	}
	return nil
}
func (o *nodeObject) Set(key string, value goja.Value) bool {
	n, p := o.node, o.page
	switch key {
	case "textContent", "innerText":
		text := o.bounded(value.String())
		if n.Type == html.TextNode || n.Type == html.CommentNode {
			n.Data = text
			return true
		}
		for n.FirstChild != nil {
			n.RemoveChild(n.FirstChild)
		}
		if text != "" {
			n.AppendChild(p.allocate(&html.Node{Type: html.TextNode, Data: text}))
		}
		return true
	case "innerHTML":
		source := o.bounded(value.String())
		if n.Type != html.ElementNode {
			o.fail("innerHTML requires element")
		}
		nodes, err := html.ParseFragment(strings.NewReader(source), n)
		if err != nil {
			o.fail(err.Error())
		}
		count := 0
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			count++
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
		for _, c := range nodes {
			walk(c)
		}
		if p.nodes+count > p.options.MaxNodes {
			o.fail("DOM node limit exceeded")
		}
		p.nodes += count
		for n.FirstChild != nil {
			n.RemoveChild(n.FirstChild)
		}
		for _, c := range nodes {
			n.AppendChild(c)
		}
		return true
	case "id", "value", "name", "type", "href", "src":
		setAttr(n, key, o.bounded(value.String()))
		return true
	case "className":
		setAttr(n, "class", o.bounded(value.String()))
		return true
	case "disabled", "checked":
		if value.ToBoolean() {
			setAttr(n, key, "")
		} else {
			for i, a := range n.Attr {
				if a.Key == key {
					n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
					break
				}
			}
		}
		return true
	case "title":
		if n.Type == html.DocumentNode {
			title := queryOne(n, "title")
			if title == nil {
				title = p.allocate(&html.Node{Type: html.ElementNode, Data: "title", DataAtom: atom.Title})
				queryOne(n, "head").AppendChild(title)
			}
			wrapped := p.wrap(title).Export().(*nodeObject)
			return wrapped.Set("textContent", value)
		}
	}
	o.properties[key] = value
	return true
}
func (o *nodeObject) Has(key string) bool    { return o.Get(key) != nil }
func (o *nodeObject) Delete(key string) bool { delete(o.properties, key); return true }
func (o *nodeObject) Keys() []string {
	keys := []string{"nodeType", "textContent", "innerHTML", "children"}
	for key := range o.properties {
		keys = append(keys, key)
	}
	return keys
}
func (p *Page) fire(n *html.Node, event string) error {
	target := p.wrap(n)
	obj := p.objects[n]
	ev := p.vm.NewObject()
	_ = ev.Set("type", event)
	_ = ev.Set("target", target)
	_ = ev.Set("currentTarget", target)
	_ = ev.Set("preventDefault", func() { _ = ev.Set("defaultPrevented", true) })
	for _, callback := range append([]goja.Callable(nil), obj.listeners[event]...) {
		if _, err := callback(target, ev); err != nil {
			return err
		}
	}
	if callback, ok := goja.AssertFunction(obj.properties["on"+event]); ok {
		_, err := callback(target, ev)
		return err
	}
	if code := attr(n, "on"+event); code != "" {
		v, err := p.vm.RunString("(function(event){" + code + "\n})")
		if err != nil {
			return err
		}
		fn, ok := goja.AssertFunction(v)
		if !ok {
			return errors.New("invalid event handler")
		}
		_, err = fn(target, ev)
		return err
	}
	return nil
}
func (p *Page) install() {
	vm := p.vm
	_ = vm.Set("document", p.wrap(p.root))
	_ = vm.Set("window", vm.GlobalObject())
	_ = vm.Set("self", vm.GlobalObject())
	_ = vm.Set("console", map[string]interface{}{"log": func(call goja.FunctionCall) goja.Value {
		parts := []string{}
		for _, v := range call.Arguments {
			parts = append(parts, v.String())
		}
		p.log(strings.Join(parts, " "))
		return goja.Undefined()
	}})
	_ = vm.Set("location", map[string]string{"href": p.url})
	_ = vm.Set("navigator", map[string]interface{}{"userAgent": "OmniBrowser/0.1", "webdriver": true})
	_ = vm.Set("addEventListener", func(call goja.FunctionCall) goja.Value {
		fn, _ := goja.AssertFunction(p.objects[p.root].Get("addEventListener"))
		v, err := fn(p.wrap(p.root), call.Arguments...)
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("listener: %w", err)))
		}
		return v
	})
}
