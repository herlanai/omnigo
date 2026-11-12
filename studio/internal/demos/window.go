package demos

import (
	"fmt"
	"github.com/herlanai/omnigo/apps/host"
	ui "github.com/herlanai/omnigo/omniui/baseui"
)

// Window configures demonstration content; native resources belong to OmniGo.
func Window(key string, dark bool, services host.WindowServices) (host.ContentWindowOptions, error) {
	if !Supported(key) && key != "web-view" {
		return host.ContentWindowOptions{}, fmt.Errorf("no executable Go demo: %s", key)
	}
	d := New(key)
	d.TitleBarAvailable = services.SupportsTitleBar
	d.RunTask = services.RunTask
	d.WriteClipboard = services.WriteClipboard
	content := host.ContentWindowOptions{Title: "Omni Studio — " + key, Width: 1040, Height: 840, View: d.View, Close: d.Close, DragRegion: "heading", TitleBarHidden: func() bool { return d.TitleBarHidden }}
	if key == "plugin/borderbeam" {
		content.Width, content.Height = 960, 820
	}
	content.BeforeRender = func(surface *ui.Surface) error {
		if !d.AppearanceDirty {
			return nil
		}
		if err := surface.SetTheme(ui.ThemePreset(dark, d.Style, d.Palette, d.BoldMenus)); err != nil {
			return err
		}
		d.AppearanceDirty = false
		return nil
	}
	if key == "web-view" {
		d.webDemo = &webViewDemo{zoom: 1, dark: dark}
		content.WebView = &host.WebViewOptions{HTML: webDocument(0, dark)}
		content.WebView.OnReady = func(page host.WebPage) { d.webDemo.page = page }
		content.WebView.OnStateReady = func(state *ui.WebViewState) { d.NativeWeb = state }
	}
	return content, nil
}
