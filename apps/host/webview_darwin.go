//go:build darwin && (amd64 || arm64)

package host

import (
	"fmt"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	ui "github.com/herlanai/omnigo/omniui/baseui"
)

// embeddedWeb owns an NSView clipping container and a WKWebView. It is a child
// of the Go window's NSImageView; only this dedicated control renders web content.
type embeddedWeb struct {
	container, web, parent objc.ID
	closed                 bool
}

func newEmbeddedWeb(parent objc.ID) (*embeddedWeb, error) {
	if _, err := purego.Dlopen("/System/Library/Frameworks/WebKit.framework/WebKit", purego.RTLD_GLOBAL|purego.RTLD_LAZY); err != nil {
		return nil, err
	}
	container := objc.ID(objc.GetClass("NSView")).Send(sel("alloc")).Send(sel("initWithFrame:"), rect{})
	container.Send(sel("setWantsLayer:"), true)
	container.Send(sel("layer")).Send(sel("setMasksToBounds:"), true)
	config := objc.ID(objc.GetClass("WKWebViewConfiguration")).Send(sel("new"))
	config.Send(sel("setWebsiteDataStore:"), objc.ID(objc.GetClass("WKWebsiteDataStore")).Send(sel("nonPersistentDataStore")))
	web := objc.ID(objc.GetClass("WKWebView")).Send(sel("alloc")).Send(sel("initWithFrame:configuration:"), rect{}, config)
	config.Send(sel("release"))
	if web == 0 {
		container.Send(sel("release"))
		return nil, fmt.Errorf("could not create embedded WKWebView")
	}
	container.Send(sel("addSubview:"), web)
	parent.Send(sel("addSubview:"), container)
	w := &embeddedWeb{container: container, web: web, parent: parent}
	// Local, functional document: no network is needed to verify native input,
	// DOM events and navigation inside the clipped Go control.
	return w, nil
}
func (w *embeddedWeb) HTML(value string) {
	if !w.closed {
		w.web.Send(sel("loadHTMLString:baseURL:"), stringID(value), objc.ID(0))
	}
}
func (w *embeddedWeb) Place(bounds, clip ui.Rect, visible, interactive bool) {
	if w.closed {
		return
	}
	// A disabled native page is hidden so it cannot intercept native input.
	w.container.Send(sel("setHidden:"), !visible || !interactive)
	if !visible || !interactive {
		return
	}
	w.container.Send(sel("setFrame:"), rect{P: point{X: clip.X, Y: objc.Send[rect](w.parent, sel("bounds")).S.H - clip.Y - clip.H}, S: size{W: clip.W, H: clip.H}})
	w.web.Send(sel("setFrame:"), rect{P: point{X: bounds.X - clip.X, Y: clip.Y + clip.H - bounds.Y - bounds.H}, S: size{W: bounds.W, H: bounds.H}})
}
func (w *embeddedWeb) Focus() {
	if !w.closed {
		w.parent.Send(sel("window")).Send(sel("makeFirstResponder:"), w.web)
	}
}
func (w *embeddedWeb) Close() {
	if w.closed {
		return
	}
	w.closed = true
	w.web.Send(sel("stopLoading"))
	w.container.Send(sel("removeFromSuperview"))
	w.web.Send(sel("release"))
	w.container.Send(sel("release"))
}

func (w *embeddedWeb) LoadHTML(value string) { w.HTML(value) }
func (w *embeddedWeb) Back() {
	if !w.closed {
		w.web.Send(sel("goBack"))
	}
}
func (w *embeddedWeb) Forward() {
	if !w.closed {
		w.web.Send(sel("goForward"))
	}
}
func (w *embeddedWeb) SetZoom(value float64) {
	if !w.closed && value >= 0.5 && value <= 2 {
		w.web.Send(sel("setPageZoom:"), value)
	}
}
