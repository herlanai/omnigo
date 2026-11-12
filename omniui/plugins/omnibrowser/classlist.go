package omnibrowser

import (
	"github.com/dop251/goja"
	"strings"
)

func (o *nodeObject) classList() *goja.Object {
	vm := o.page.vm
	obj := vm.NewObject()
	tokens := func() []string { return strings.Fields(attr(o.node, "class")) }
	valid := func(v goja.Value) string {
		token := v.String()
		if token == "" || strings.ContainsAny(token, " \t\r\n\f") {
			panic(vm.NewTypeError("invalid class token"))
		}
		return token
	}
	contains := func(token string) bool {
		for _, s := range tokens() {
			if s == token {
				return true
			}
		}
		return false
	}
	set := func(token string, add bool) {
		items := tokens()
		out := items[:0]
		for _, s := range items {
			if s != token {
				out = append(out, s)
			}
		}
		if add {
			out = append(out, token)
		}
		setAttr(o.node, "class", strings.Join(out, " "))
	}
	_ = obj.Set("contains", func(token string) bool { return contains(token) })
	for _, method := range []string{"add", "remove"} {
		_ = obj.Set(method, func(c goja.FunctionCall) goja.Value {
			for _, v := range c.Arguments {
				valid(v)
			}
			for _, v := range c.Arguments {
				token := v.String()
				if method == "remove" {
					set(token, false)
				} else if !contains(token) {
					set(token, true)
				}
			}
			return goja.Undefined()
		})
	}
	_ = obj.Set("toggle", func(c goja.FunctionCall) goja.Value {
		token := valid(c.Argument(0))
		add := !contains(token)
		if len(c.Arguments) > 1 {
			add = c.Argument(1).ToBoolean()
		}
		set(token, add)
		return vm.ToValue(add)
	})
	_ = obj.Set("replace", func(c goja.FunctionCall) goja.Value {
		a, b := valid(c.Argument(0)), valid(c.Argument(1))
		if !contains(a) {
			return vm.ToValue(false)
		}
		items := tokens()
		for i, v := range items {
			if v == a {
				items[i] = b
			}
		}
		setAttr(o.node, "class", strings.Join(items, " "))
		return vm.ToValue(true)
	})
	_ = obj.DefineAccessorProperty("value", vm.ToValue(func() string { return attr(o.node, "class") }), vm.ToValue(func(value string) { setAttr(o.node, "class", o.bounded(value)) }), goja.FLAG_TRUE, goja.FLAG_TRUE)
	_ = obj.DefineAccessorProperty("length", vm.ToValue(func() int { return len(tokens()) }), nil, goja.FLAG_TRUE, goja.FLAG_TRUE)
	return obj
}
