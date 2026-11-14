//go:build darwin && (amd64 || arm64)

package host

import (
	"fmt"
	"net/url"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/herlanai/omnigo/omniwindow"
)

// openShell embeds the bundled TanStack application in WKWebView. The loopback
// transport lives in this same process; no browser or frontend server is launched.
func openShell(app objc.ID, options omniwindow.WindowOptions, quit func(), host *Controller) (func(), error) {
	var chrome *windowChrome
	address, _ := url.Parse(options.URL)
	origin := address.Scheme + "://" + address.Host
	if _, err := purego.Dlopen("/System/Library/Frameworks/WebKit.framework/WebKit", purego.RTLD_GLOBAL|purego.RTLD_LAZY); err != nil {
		return nil, err
	}
	cls, err := objc.RegisterClass("OmniGoUIAppShellDelegate", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
		{Cmd: sel("windowWillClose:"), Fn: func(_ objc.ID, _ objc.SEL, note objc.ID) {
			if cancel := nativeDialogOwners[note.Send(sel("object"))]; cancel != nil {
				cancel()
			}
			quit()
		}},
		{Cmd: sel("windowDidResize:"), Fn: func(objc.ID, objc.SEL, objc.ID) {
			if chrome != nil {
				chrome.layout()
			}
		}},
		{Cmd: sel("windowDidExitFullScreen:"), Fn: func(objc.ID, objc.SEL, objc.ID) {
			if chrome != nil {
				chrome.layout()
			}
		}},
		{Cmd: sel("quit:"), Fn: func(objc.ID, objc.SEL, objc.ID) { quit() }},
		{Cmd: sel("applicationShouldTerminate:"), Fn: func(objc.ID, objc.SEL, objc.ID) uint64 { quit(); return 0 }},
		{Cmd: sel("webView:decidePolicyForNavigationAction:decisionHandler:"), Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID, action objc.ID, decision unsafe.Pointer) {
			policy := uint64(0)
			guard(func() {
				url := objc.Send[string](action.Send(sel("request")).Send(sel("URL")).Send(sel("absoluteString")), sel("UTF8String"))
				if url == origin || strings.HasPrefix(url, origin+"/") {
					policy = 1
				}
			})
			// Native WebKit blocks are not Go-created objc.Block values.
			// Apple Block ABI: isa, flags, reserved, invoke function pointer.
			header := (*struct {
				Isa             uintptr
				Flags, Reserved uint32
				Invoke          uintptr
			})(decision)
			var resolve func(unsafe.Pointer, uint64)
			purego.RegisterFunc(&resolve, header.Invoke)
			resolve(decision, policy)
		}},
	})
	if err != nil {
		return nil, err
	}
	delegate := objc.ID(cls).Send(sel("new"))
	config := objc.ID(objc.GetClass("WKWebViewConfiguration")).Send(sel("new"))
	detachBridge, err := attachNativeBridge(config, host, origin)
	if err != nil {
		config.Send(sel("release"))
		delegate.Send(sel("release"))
		return nil, err
	}
	// The host stores no persistent browsing session, credentials or third-party website data.
	config.Send(sel("setWebsiteDataStore:"), objc.ID(objc.GetClass("WKWebsiteDataStore")).Send(sel("nonPersistentDataStore")))
	web := objc.ID(objc.GetClass("WKWebView")).Send(sel("alloc")).Send(sel("initWithFrame:configuration:"), rect{S: size{W: float64(options.Width), H: float64(options.Height)}}, config)
	config.Send(sel("release"))
	if web == 0 {
		detachBridge()
		delegate.Send(sel("release"))
		return nil, fmt.Errorf("host: could not create WKWebView")
	}
	detachUpload, err := attachUploadDelegate(web)
	if err != nil {
		detachBridge()
		web.Send(sel("release"))
		delegate.Send(sel("release"))
		return nil, err
	}
	web.Send(sel("setNavigationDelegate:"), delegate)
	web.Send(sel("setAutoresizingMask:"), uint64(18))
	window := objc.ID(objc.GetClass("NSWindow")).Send(sel("alloc")).Send(sel("initWithContentRect:styleMask:backing:defer:"), rect{S: size{W: float64(options.Width), H: float64(options.Height)}}, uint64(1|2|4|8), uint64(2), false)
	window.Send(sel("setReleasedWhenClosed:"), false)
	window.Send(sel("setTitle:"), stringID(options.Title))
	window.Send(sel("setMinSize:"), size{W: float64(options.MinWidth), H: float64(options.MinHeight)})
	window.Send(sel("setDelegate:"), delegate)
	window.Send(sel("setContentView:"), web)
	chrome = newWindowChrome(window)
	if _, err := chrome.apply(TitleBarRequest{Style: options.TitleBarStyle, TrafficLightPosition: options.TrafficLightPosition}); err != nil {
		detachBridge()
		detachUpload()
		web.Send(sel("setNavigationDelegate:"), objc.ID(0))
		window.Send(sel("setDelegate:"), objc.ID(0))
		window.Send(sel("close"))
		window.Send(sel("release"))
		web.Send(sel("release"))
		delegate.Send(sel("release"))
		return nil, err
	}
	host.titlebar = chrome.apply
	app.Send(sel("setDelegate:"), delegate)
	menu := objc.ID(objc.GetClass("NSMenu")).Send(sel("new"))
	appItem := objc.ID(objc.GetClass("NSMenuItem")).Send(sel("new"))
	menu.Send(sel("addItem:"), appItem)
	appMenu := objc.ID(objc.GetClass("NSMenu")).Send(sel("new"))
	appItem.Send(sel("setSubmenu:"), appMenu)
	quitItem := objc.ID(objc.GetClass("NSMenuItem")).Send(sel("alloc")).Send(sel("initWithTitle:action:keyEquivalent:"), stringID("Quit "+options.Title), sel("quit:"), stringID("q"))
	quitItem.Send(sel("setTarget:"), delegate)
	appMenu.Send(sel("addItem:"), quitItem)
	// Standard editing menu gives the embedded directory native copy/paste shortcuts.
	editItem := objc.ID(objc.GetClass("NSMenuItem")).Send(sel("new"))
	menu.Send(sel("addItem:"), editItem)
	edit := objc.ID(objc.GetClass("NSMenu")).Send(sel("alloc")).Send(sel("initWithTitle:"), stringID("Edit"))
	editItem.Send(sel("setSubmenu:"), edit)
	for _, entry := range [][3]string{{"Cut", "cut:", "x"}, {"Copy", "copy:", "c"}, {"Paste", "paste:", "v"}, {"Select All", "selectAll:", "a"}} {
		item := objc.ID(objc.GetClass("NSMenuItem")).Send(sel("alloc")).Send(sel("initWithTitle:action:keyEquivalent:"), stringID(entry[0]), sel(entry[1]), stringID(entry[2]))
		edit.Send(sel("addItem:"), item)
		item.Send(sel("release"))
	}
	app.Send(sel("setMainMenu:"), menu)
	for _, obj := range []objc.ID{quitItem, appMenu, appItem, edit, editItem, menu} {
		obj.Send(sel("release"))
	}
	url := objc.ID(objc.GetClass("NSURL")).Send(sel("URLWithString:"), stringID(options.URL))
	request := objc.ID(objc.GetClass("NSURLRequest")).Send(sel("requestWithURL:"), url)
	web.Send(sel("loadRequest:"), request)
	window.Send(sel("center"))
	window.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
	app.Send(sel("activateIgnoringOtherApps:"), true)
	return func() {
		detachBridge()
		detachUpload()
		host.titlebar = nil
		app.Send(sel("setDelegate:"), objc.ID(0))
		window.Send(sel("setDelegate:"), objc.ID(0))
		web.Send(sel("setNavigationDelegate:"), objc.ID(0))
		web.Send(sel("stopLoading"))
		window.Send(sel("close"))
		web.Send(sel("release"))
		window.Send(sel("release"))
		delegate.Send(sel("release"))
	}, nil
}
