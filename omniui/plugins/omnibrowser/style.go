package omnibrowser

import (
	"context"
	"github.com/dop251/goja"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser/css"
	"strings"
)

func (p *Page) ComputedStyle(ctx context.Context, selector string) (map[string]string, error) {
	var out map[string]string
	err := p.operation(ctx, func(context.Context) error {
		n, err := p.selectOne(selector)
		if err != nil {
			return err
		}
		out = css.Document(p.root).Styles(p.root)[n]
		return nil
	})
	return out, err
}

type styleObject struct {
	o        *nodeObject
	computed bool
}

func cssName(k string) string {
	var b strings.Builder
	for _, c := range k {
		if c >= 'A' && c <= 'Z' {
			b.WriteByte('-')
			c += 32
		}
		b.WriteRune(c)
	}
	return b.String()
}
func (s *styleObject) values() map[string]string {
	if s.computed {
		return css.Document(s.o.page.root).Styles(s.o.page.root)[s.o.node]
	}
	m := map[string]string{}
	for _, d := range css.Declarations(attr(s.o.node, "style")) {
		m[d.Name] = d.Value
	}
	return m
}
func (s *styleObject) Get(k string) goja.Value {
	vm := s.o.page.vm
	switch k {
	case "getPropertyValue":
		return vm.ToValue(func(name string) string { return s.values()[name] })
	case "setProperty":
		return vm.ToValue(func(call goja.FunctionCall) goja.Value {
			name, value := call.Argument(0).String(), call.Argument(1).String()
			priority := ""
			if !goja.IsUndefined(call.Argument(2)) {
				priority = strings.ToLower(call.Argument(2).String())
			}
			if priority != "" && priority != "important" {
				return goja.Undefined()
			}
			if priority == "important" && value != "" {
				value += " !important"
			}
			s.Set(name, vm.ToValue(value))
			return goja.Undefined()
		})
	case "removeProperty":
		return vm.ToValue(func(name string) string { old := s.values()[name]; s.Set(name, vm.ToValue("")); return old })
	case "cssText":
		return vm.ToValue(attr(s.o.node, "style"))
	}
	return vm.ToValue(s.values()[cssName(k)])
}
func (s *styleObject) Set(k string, v goja.Value) bool {
	if s.computed {
		return false
	}
	if k == "cssText" {
		setAttr(s.o.node, "style", s.o.bounded(v.String()))
		return true
	}
	k = cssName(k)
	decls := css.Declarations(attr(s.o.node, "style"))
	var b strings.Builder
	for _, d := range decls {
		if d.Name == k {
			continue
		}
		b.WriteString(d.Name + ":" + d.Value)
		if d.Important {
			b.WriteString(" !important")
		}
		b.WriteByte(';')
	}
	if v.String() != "" {
		b.WriteString(k + ":" + v.String() + ";")
	}
	setAttr(s.o.node, "style", s.o.bounded(b.String()))
	return true
}
func (s *styleObject) Has(k string) bool    { return s.Get(k) != nil }
func (s *styleObject) Delete(k string) bool { return s.Set(k, s.o.page.vm.ToValue("")) }
func (s *styleObject) Keys() []string {
	var keys []string
	for k := range s.values() {
		keys = append(keys, k)
	}
	return keys
}
