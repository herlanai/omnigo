package omnibrowser

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/dop251/goja"
	"io"
	"net/http"
	"net/url"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser/css"
	"strings"
	"time"
)

type scheduled struct {
	at   time.Time
	fn   goja.Callable
	args []goja.Value
}

func (p *Page) installWebAPI() {
	vm := p.vm
	_ = vm.Set("__mediaMatches", func(query string) bool { return css.MediaMatches(query, 1024, 768) })
	p.timers = map[int]scheduled{}
	p.nextTimer = 0
	_ = vm.Set("getComputedStyle", func(v goja.Value) goja.Value {
		o, ok := v.Export().(*nodeObject)
		if !ok {
			panic(vm.NewTypeError("expected element"))
		}
		return vm.NewDynamicObject(&styleObject{o: o, computed: true})
	})
	_ = vm.Set("setTimeout", func(c goja.FunctionCall) goja.Value {
		fn, ok := goja.AssertFunction(c.Argument(0))
		if !ok {
			panic(vm.NewTypeError("timer callback must be a function"))
		}
		if len(p.timers) >= 1024 {
			panic(vm.NewTypeError("timer limit"))
		}
		ms := c.Argument(1).ToInteger()
		if ms < 0 {
			ms = 0
		}
		if ms > 2147483647 {
			ms = 2147483647
		}
		p.nextTimer++
		p.timers[p.nextTimer] = scheduled{time.Now().Add(time.Duration(ms) * time.Millisecond), fn, append([]goja.Value(nil), c.Arguments[min(2, len(c.Arguments)):]...)}
		return vm.ToValue(p.nextTimer)
	})
	_ = vm.Set("clearTimeout", func(id int) { delete(p.timers, id) })
	_ = vm.Set("atob", func(s string) string {
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			panic(vm.NewTypeError("invalid base64"))
		}
		return string(b)
	})
	_ = vm.Set("btoa", func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) })
	_ = vm.Set("__parseURL", func(input, base string) map[string]string {
		u, err := url.Parse(input)
		if err != nil {
			panic(vm.NewTypeError("invalid URL"))
		}
		if base != "" {
			b, e := url.Parse(base)
			if e != nil {
				panic(vm.NewTypeError("invalid base URL"))
			}
			u = b.ResolveReference(u)
		}
		if u.Scheme == "" {
			panic(vm.NewTypeError("absolute URL required"))
		}
		q, f := "", ""
		if u.RawQuery != "" {
			q = "?" + u.RawQuery
		}
		if u.Fragment != "" {
			f = "#" + u.Fragment
		}
		return map[string]string{"href": u.String(), "protocol": u.Scheme + ":", "host": u.Host, "hostname": u.Hostname(), "port": u.Port(), "pathname": u.Path, "search": q, "hash": f, "origin": u.Scheme + "://" + u.Host}
	})
	_ = vm.Set("__fetch", func(address, method, body string, headers map[string]string) map[string]any {
		base, _ := url.Parse(p.url)
		u, err := url.Parse(address)
		if err != nil {
			panic(vm.NewTypeError("invalid URL"))
		}
		u = base.ResolveReference(u)
		if err := validateURL(u); err != nil {
			panic(vm.NewGoError(err))
		}
		if u.Scheme != base.Scheme || u.Host != base.Host {
			panic(vm.NewTypeError("cross-origin fetch is unsupported"))
		}
		if int64(len(body)) > p.options.MaxBytes {
			panic(vm.NewTypeError("request byte limit"))
		}
		if method == "" {
			method = "GET"
		}
		req, err := http.NewRequestWithContext(p.activeCtx, strings.ToUpper(method), u.String(), strings.NewReader(body))
		if err != nil {
			panic(vm.NewGoError(err))
		}
		for k, v := range headers {
			switch strings.ToLower(k) {
			case "cookie", "host", "origin", "connection", "content-length":
				panic(vm.NewTypeError("forbidden fetch header"))
			}
			req.Header.Set(k, v)
		}
		client := *p.client.client
		client.CheckRedirect = func(r *http.Request, via []*http.Request) error {
			if r.URL.Scheme != base.Scheme || r.URL.Host != base.Host {
				return errors.New("cross-origin redirect unsupported")
			}
			return p.client.client.CheckRedirect(r, via)
		}
		response, err := client.Do(req)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		defer response.Body.Close()
		data, err := io.ReadAll(io.LimitReader(response.Body, p.options.MaxBytes+1))
		if err != nil {
			panic(vm.NewGoError(err))
		}
		if int64(len(data)) > p.options.MaxBytes {
			panic(vm.NewTypeError("response byte limit"))
		}
		h := map[string]string{}
		for k, v := range response.Header {
			if strings.EqualFold(k, "Set-Cookie") {
				continue
			}
			h[strings.ToLower(k)] = strings.Join(v, ", ")
		}
		return map[string]any{"body": string(data), "status": response.StatusCode, "statusText": http.StatusText(response.StatusCode), "url": response.Request.URL.String(), "headers": h}
	})
	if p.storage == nil {
		p.storage = map[string]map[string]string{}
	}
	for _, name := range []string{"localStorage", "sessionStorage"} {
		u, _ := url.Parse(p.url)
		key := name + ":" + u.Scheme + "://" + u.Host
		if p.storage[key] == nil {
			p.storage[key] = map[string]string{}
		}
		m := p.storage[key]
		obj := vm.NewObject()
		_ = obj.Set("getItem", func(k string) any {
			v, ok := m[k]
			if !ok {
				return nil
			}
			return v
		})
		_ = obj.Set("setItem", func(k, v string) {
			size := len(k) + len(v)
			for key, val := range m {
				if key != k {
					size += len(key) + len(val)
				}
			}
			if int64(size) > p.options.MaxBytes {
				panic(vm.NewTypeError("storage quota exceeded"))
			}
			m[k] = v
		})
		_ = obj.Set("removeItem", func(k string) { delete(m, k) })
		_ = obj.Set("clear", func() { clear(m) })
		_ = obj.DefineAccessorProperty("length", vm.ToValue(func() int { return len(m) }), nil, goja.FLAG_TRUE, goja.FLAG_TRUE)
		_ = vm.Set(name, obj)
	}
	_, err := vm.RunString(webBootstrap)
	if err != nil {
		panic(err)
	}
}

// await drives callbacks on the page owner goroutine; no goroutine enters Goja.
func (p *Page) await(ctx context.Context, v goja.Value) (goja.Value, error) {
	promise, ok := v.Export().(*goja.Promise)
	if !ok {
		return v, nil
	}
	for promise.State() == goja.PromiseStatePending {
		var id int
		var next scheduled
		for k, t := range p.timers {
			if id == 0 || t.at.Before(next.at) {
				id, next = k, t
			}
		}
		if id == 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		delay := time.Until(next.at)
		if delay > 0 {
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
			return nil, err
		}
	}
	if promise.State() == goja.PromiseStateRejected {
		return nil, errors.New(promise.Result().String())
	}
	return promise.Result(), nil
}

const webBootstrap = `
class Headers {constructor(init={}){this._m={};if(init instanceof Headers)init=init._m;for(const [k,v] of (Array.isArray(init)?init:Object.entries(init)))this.set(k,v)}set(k,v){this._m[String(k).toLowerCase()]=String(v)}append(k,v){const a=this.get(k);this.set(k,a===null?v:a+', '+v)}get(k){return this._m[String(k).toLowerCase()]??null}has(k){return this.get(k)!==null}delete(k){delete this._m[String(k).toLowerCase()]}entries(){return Object.entries(this._m)[Symbol.iterator]()}[Symbol.iterator](){return this.entries()}forEach(fn){for(const [k,v] of this)fn(v,k,this)}}
class Response {constructor(body='',init={}){this._body=String(body);this.status=init.status??200;this.statusText=init.statusText??'';this.headers=new Headers(init.headers);this.url=init.url??'';this.ok=this.status>=200&&this.status<300;this.bodyUsed=false}text(){if(this.bodyUsed)return Promise.reject(new TypeError('body already used'));this.bodyUsed=true;return Promise.resolve(this._body)}json(){return this.text().then(JSON.parse)}clone(){if(this.bodyUsed)throw new TypeError('body already used');return new Response(this._body,this)}}
function fetch(input,init={}){try{if(init.signal?.aborted)throw new Error('AbortError');const r=__fetch(String(input),init.method??'GET',init.body??'',new Headers(init.headers)._m);return Promise.resolve(new Response(r.body,r))}catch(e){return Promise.reject(e)}}
class URLSearchParams{constructor(init=''){this._p=[];if(typeof init==='string'){for(const s of init.replace(/^\?/,'').split('&')){if(!s)continue;const i=s.indexOf('=');this.append(decodeURIComponent((i<0?s:s.slice(0,i)).replace(/\+/g,' ')),decodeURIComponent((i<0?'':s.slice(i+1)).replace(/\+/g,' ')))}}else for(const [k,v] of (Array.isArray(init)?init:Object.entries(init)))this.append(k,v)}append(k,v){this._p.push([String(k),String(v)])}get(k){return this._p.find(p=>p[0]===String(k))?.[1]??null}getAll(k){return this._p.filter(p=>p[0]===String(k)).map(p=>p[1])}has(k){return this.get(k)!==null}delete(k){this._p=this._p.filter(p=>p[0]!==String(k))}set(k,v){const i=this._p.findIndex(p=>p[0]===String(k));this.delete(k);this._p.splice(i<0?this._p.length:i,0,[String(k),String(v)])}toString(){return this._p.map(p=>p.map(v=>encodeURIComponent(v).replace(/%20/g,'+')).join('=')).join('&')}entries(){return this._p[Symbol.iterator]()}[Symbol.iterator](){return this.entries()}}
class URL{constructor(input,base){Object.assign(this,__parseURL(String(input),base===undefined?'':String(base)));this.searchParams=new URLSearchParams(this.search)}toString(){return this.href}toJSON(){return this.href}}
function matchMedia(query){const listeners=new Set();return {media:String(query),matches:__mediaMatches(String(query)),onchange:null,addListener(fn){listeners.add(fn)},removeListener(fn){listeners.delete(fn)},addEventListener(type,fn){if(type==="change")listeners.add(fn)},removeEventListener(type,fn){if(type==="change")listeners.delete(fn)}}}
function requestAnimationFrame(fn){return setTimeout(()=>fn(performance.now()),0)}
function cancelAnimationFrame(id){clearTimeout(id)}
function queueMicrotask(fn){Promise.resolve().then(fn)}
const performance={now:()=>Date.now()};
class Event{constructor(type,options={}){this.type=String(type);this.bubbles=!!options.bubbles;this.cancelable=!!options.cancelable;this.defaultPrevented=false}preventDefault(){if(this.cancelable)this.defaultPrevented=true}stopPropagation(){this._stopped=true}}
Object.assign(globalThis,{Headers,Response,URLSearchParams,URL,Event,fetch,queueMicrotask,performance,requestAnimationFrame,cancelAnimationFrame,matchMedia});
`
