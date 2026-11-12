package omnibrowser

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dop251/goja"
	"golang.org/x/net/html"
	"math"
	"math/big"
)

type Remote struct {
	Type                string `json:"type"`
	Subtype             string `json:"subtype,omitempty"`
	ClassName           string `json:"className,omitempty"`
	Description         string `json:"description,omitempty"`
	ObjectID            string `json:"objectId,omitempty"`
	Value               any    `json:"value,omitempty"`
	UnserializableValue string `json:"unserializableValue,omitempty"`
}
type Argument struct {
	ObjectID            string          `json:"objectId"`
	Value               json.RawMessage `json:"value"`
	UnserializableValue string          `json:"unserializableValue"`
}

func (p *Page) NewSibling() *Page { return New(p.options) }
func (p *Page) remote(v goja.Value, byValue bool) (Remote, error) {
	if goja.IsUndefined(v) {
		return Remote{Type: "undefined"}, nil
	}
	if goja.IsNull(v) {
		return Remote{Type: "object", Subtype: "null", Value: json.RawMessage("null")}, nil
	}
	r := Remote{}
	if obj, ok := v.(*goja.Object); ok {
		r.Type = "object"
		r.ClassName = obj.ClassName()
		r.Description = r.ClassName
		if _, ok := goja.AssertFunction(obj); ok {
			r.Type = "function"
		}
		if r.ClassName == "Array" {
			r.Subtype = "array"
		}
		if _, ok := obj.Export().(*nodeObject); ok {
			r.Subtype = "node"
			r.ClassName = "HTMLElement"
		}
		if !byValue {
			if len(p.handles) >= 4096 {
				return r, fmt.Errorf("remote object quota exceeded")
			}
			p.nextHandle++
			r.ObjectID = fmt.Sprintf("%d:%d:%d", p.instance, p.generation, p.nextHandle)
			p.handles[r.ObjectID] = v
			return r, nil
		}
	} else {
		switch value := v.Export().(type) {
		case *big.Int:
			return Remote{Type: "bigint", UnserializableValue: value.String() + "n", Description: value.String() + "n"}, nil
		case float64:
			if math.IsNaN(value) {
				return Remote{Type: "number", UnserializableValue: "NaN"}, nil
			}
			if math.IsInf(value, 1) {
				return Remote{Type: "number", UnserializableValue: "Infinity"}, nil
			}
			if math.IsInf(value, -1) {
				return Remote{Type: "number", UnserializableValue: "-Infinity"}, nil
			}
			if value == 0 && math.Signbit(value) {
				return Remote{Type: "number", UnserializableValue: "-0"}, nil
			}
			r.Type = "number"
		case bool:
			r.Type = "boolean"
		case string:
			r.Type = "string"
		default:
			r.Type = "number"
		}
	}
	stringify, _ := goja.AssertFunction(p.vm.Get("JSON").ToObject(p.vm).Get("stringify"))
	encoded, err := stringify(goja.Undefined(), v)
	if err != nil {
		return r, err
	}
	if !goja.IsUndefined(encoded) {
		r.Value = json.RawMessage(encoded.String())
	}
	return r, nil
}
func (p *Page) EvaluateRemote(ctx context.Context, expression string, byValue, await bool) (Remote, error) {
	return p.remoteOperation(ctx, byValue, await, func() (goja.Value, error) {
		if int64(len(expression)) > p.options.MaxBytes {
			return nil, fmt.Errorf("expression byte limit")
		}
		return p.vm.RunString(expression)
	})
}
func (p *Page) CallRemote(ctx context.Context, declaration, objectID string, args []Argument, byValue, await bool) (Remote, error) {
	return p.remoteOperation(ctx, byValue, await, func() (goja.Value, error) {
		if int64(len(declaration)) > p.options.MaxBytes {
			return nil, fmt.Errorf("function byte limit")
		}
		value, err := p.vm.RunString("(" + declaration + "\n)")
		if err != nil {
			return nil, err
		}
		fn, ok := goja.AssertFunction(value)
		if !ok {
			return nil, fmt.Errorf("function required")
		}
		this := goja.Undefined()
		if objectID != "" {
			this = p.handles[objectID]
			if this == nil {
				return nil, fmt.Errorf("stale object handle")
			}
		}
		values := make([]goja.Value, len(args))
		for i, a := range args {
			if a.ObjectID != "" {
				values[i] = p.handles[a.ObjectID]
				if values[i] == nil {
					return nil, fmt.Errorf("stale argument handle")
				}
			} else if a.UnserializableValue != "" {
				switch a.UnserializableValue {
				case "NaN", "Infinity", "-Infinity", "-0":
					values[i], err = p.vm.RunString(a.UnserializableValue)
				default:
					return nil, fmt.Errorf("unsupported unserializable value")
				}
			} else if len(a.Value) > 0 {
				var decoded any
				if err = json.Unmarshal(a.Value, &decoded); err != nil {
					return nil, err
				}
				values[i] = p.vm.ToValue(decoded)
			} else {
				values[i] = goja.Undefined()
			}
		}
		return fn(this, values...)
	})
}
func (p *Page) remoteOperation(ctx context.Context, byValue, await bool, fn func() (goja.Value, error)) (Remote, error) {
	var result Remote
	err := p.operation(ctx, func(ctx context.Context) error {
		if p.vm == nil {
			return fmt.Errorf("no execution context")
		}
		_, err := p.run(ctx, func() (goja.Value, error) {
			v, e := fn()
			if e != nil {
				return nil, e
			}
			if await {
				v, e = p.await(ctx, v)
				if e != nil {
					return nil, e
				}
			}
			result, e = p.remote(v, byValue)
			return v, e
		})
		return err
	})
	return result, err
}
func (p *Page) Properties(ctx context.Context, id string) ([]map[string]any, error) {
	out := []map[string]any{}
	err := p.operation(ctx, func(ctx context.Context) error {
		v := p.handles[id]
		if v == nil {
			return fmt.Errorf("stale object handle")
		}
		_, err := p.run(ctx, func() (goja.Value, error) {
			obj := v.ToObject(p.vm)
			for _, key := range obj.Keys() {
				r, e := p.remote(obj.Get(key), false)
				if e != nil {
					return nil, e
				}
				out = append(out, map[string]any{"name": key, "value": r, "enumerable": true, "configurable": true, "writable": true, "isOwn": true})
			}
			return goja.Undefined(), nil
		})
		return err
	})
	return out, err
}
func (p *Page) Release(ctx context.Context, id string) error {
	return p.operation(ctx, func(context.Context) error { delete(p.handles, id); return nil })
}

// NewRealm shares this document and its serialized DOM ownership, but allocates
// an independent JavaScript global environment. Navigation invalidates callers'
// realms; CDP recreates them on each document load.
func (p *Page) NewRealm(ctx context.Context) (*Page, error) {
	var realm *Page
	err := p.operation(ctx, func(context.Context) error {
		if p.root == nil {
			return fmt.Errorf("no document")
		}
		realm = New(p.options)
		realm.client.close()
		realm.client = p.client
		realm.sharedNetwork = true
		realm.mu = p.mu
		realm.root = p.root
		realm.url = p.url
		realm.nodes = p.nodes
		realm.storage = p.storage
		realm.cancel()
		realm.life, realm.cancel = context.WithCancel(p.life)
		p.realmCancels = append(p.realmCancels, realm.cancel)
		realm.vm = goja.New()
		realm.objects = map[*html.Node]*nodeObject{}
		realm.handles = map[string]goja.Value{}
		realm.generation = 1
		realm.install()
		return nil
	})
	return realm, err
}
