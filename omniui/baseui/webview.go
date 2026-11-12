package baseui

// NativeWebView is supplied by an optional platform host on the UI goroutine.
// Place receives logical top-left coordinates and the visible clipping rect.
// The host must hide an empty/noninteractive view and must not retain Painter.
type NativeWebView interface {
	Place(bounds, clip Rect, visible, interactive bool)
	Focus()
}
type WebViewState struct{ view NativeWebView }

func NewWebViewState(view NativeWebView) *WebViewState {
	if view == nil {
		panic("baseui: nil native web view")
	}
	return &WebViewState{view: view}
}

// WebView embeds a real native view. Surface controls placement, clipping,
// focus and hiding on unmount; the creator owns navigation and Close lifetime.
// Platform adapters are optional; the base package has no WebKit/CEF dependency.
func WebView(id string, state *WebViewState) *Element {
	if state == nil || state.view == nil {
		panic("baseui: nil web view state")
	}
	e := Canvas(id, "Web content", func(p *Painter, r Rect) { p.Fill(r, p.theme.Surface, 0) }).Height(280).Role("document")
	e.nativeView = state
	e.onKey = func(ev KeyEvent) bool {
		if ev.Key == "Enter" {
			state.view.Focus()
			return true
		}
		return false
	}
	return e
}

// SyncNativeViews applies the current geometry after layout/scroll. Hosts call
// it before presenting each frame. Hidden controls preserve their loaded page.
func (s *Surface) SyncNativeViews() {
	next := map[*WebViewState]bool{}
	for _, item := range s.entries {
		v := item.element.nativeView
		if v == nil {
			continue
		}
		next[v] = true
		visible := !s.closed && item.clip.W > 0 && item.clip.H > 0 && s.inScope(item)
		v.view.Place(item.bounds, intersect(item.bounds, item.clip), visible, !item.disabled && !item.element.readOnly)
	}
	for view := range s.nativeViews {
		if !next[view] {
			view.view.Place(Rect{}, Rect{}, false, false)
		}
	}
	s.nativeViews = next
}
