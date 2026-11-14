//go:build darwin && (amd64 || arm64)

package host

import (
	"context"
	"encoding/json"
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"net/url"
	"time"
	"unsafe"
)

func attachNativeBridge(config objc.ID, h *Controller, origin string) (func(), error) {
	if h.nativeExchange == nil {
		return func() {}, nil
	}
	var copyBlock func(unsafe.Pointer) unsafe.Pointer
	var releaseBlock func(unsafe.Pointer)
	for name, fn := range map[string]any{"_Block_copy": &copyBlock, "_Block_release": &releaseBlock} {
		p, e := purego.Dlsym(purego.RTLD_DEFAULT, name)
		if e != nil {
			return nil, e
		}
		purego.RegisterFunc(fn, p)
	}
	controller := config.Send(sel("userContentController")).Send(sel("retain"))
	pending := map[string]func(string, string){}
	closed := false
	cls, err := objc.RegisterClass("OmniGoNativeMessageBridge", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{{Cmd: sel("userContentController:didReceiveScriptMessage:replyHandler:"), Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID, message objc.ID, reply unsafe.Pointer) {
		block := copyBlock(reply)
		resolve := func(value, failure string) {
			head := (*struct {
				Isa             uintptr
				Flags, Reserved uint32
				Invoke          uintptr
			})(block)
			var invoke func(unsafe.Pointer, objc.ID, objc.ID)
			purego.RegisterFunc(&invoke, head.Invoke)
			var result, problem objc.ID
			if failure != "" {
				problem = stringID(failure)
			} else {
				result = stringID(value)
			}
			invoke(block, result, problem)
			releaseBlock(block)
		}
		frame := message.Send(sel("frameInfo"))
		address := objc.Send[string](frame.Send(sel("request")).Send(sel("URL")).Send(sel("absoluteString")), sel("UTF8String"))
		u, e := url.Parse(address)
		if closed || e != nil || u.User != nil || u.Scheme+"://"+u.Host != origin || frame.Send(sel("isMainFrame")) == 0 {
			resolve("", "Native frame rejected")
			return
		}
		body := message.Send(sel("body"))
		if body.Send(sel("isKindOfClass:"), objc.GetClass("NSString")) == 0 || body.Send(sel("length")) > 1<<20 {
			resolve("", "Invalid native envelope")
			return
		}
		raw := objc.Send[string](body, sel("UTF8String"))
		var envelope struct {
			Kind    string          `json:"kind"`
			Token   string          `json:"token"`
			Request json.RawMessage `json:"request"`
		}
		if json.Unmarshal([]byte(raw), &envelope) != nil || len(envelope.Token) == 0 || len(envelope.Token) > 128 {
			resolve("", "Invalid native envelope")
			return
		}
		if envelope.Kind == "cancel" {
			if finish := pending[envelope.Token]; finish != nil {
				finish("", "Request canceled")
			}
			resolve("null", "")
			return
		}
		if envelope.Kind != "call" || len(pending) >= 64 || pending[envelope.Token] != nil {
			resolve("", "Native request rejected")
			return
		}
		ctx, cancel := context.WithTimeout(h.ctx, 30*time.Second)
		finished := false
		finish := func(value, failure string) {
			if finished {
				return
			}
			finished = true
			delete(pending, envelope.Token)
			cancel()
			resolve(value, failure)
		}
		pending[envelope.Token] = finish
		go func() {
			result, err := h.nativeExchange(ctx, envelope.Request)
			failure := ""
			if err != nil {
				failure = "Native request failed"
			}
			h.control(func() { finish(string(result), failure) })
		}()
	}}})
	if err != nil {
		controller.Send(sel("release"))
		return nil, err
	}
	delegate := objc.ID(cls).Send(sel("new"))
	world := objc.ID(objc.GetClass("WKContentWorld")).Send(sel("pageWorld"))
	controller.Send(sel("addScriptMessageHandlerWithReply:contentWorld:name:"), delegate, world, stringID("omnigoNative"))
	return func() {
		closed = true
		controller.Send(sel("removeScriptMessageHandlerForName:contentWorld:"), stringID("omnigoNative"), world)
		for _, finish := range pending {
			finish("", "Window closed")
		}
		delegate.Send(sel("release"))
		controller.Send(sel("release"))
	}, nil
}
