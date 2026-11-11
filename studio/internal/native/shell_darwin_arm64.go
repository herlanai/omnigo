//go:build darwin && arm64

package native

import (
	"fmt"
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"strings"
	"unsafe"
)

// openShell embeds the bundled TanStack application in WKWebView. The loopback
// transport lives in this same process; no browser or frontend server is launched.
func openShell(app objc.ID, origin string, quit func()) (func(), error) {
	if _, err := purego.Dlopen("/System/Library/Frameworks/WebKit.framework/WebKit", purego.RTLD_GLOBAL|purego.RTLD_LAZY); err != nil {
		return nil, err
	}
	cls, err := objc.RegisterClass("OmniStudioShellDelegate", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
		{Cmd: sel("windowWillClose:"), Fn: func(objc.ID, objc.SEL, objc.ID) { quit() }},
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
	// Studio stores no browsing session, credentials or third-party website data.
	config.Send(sel("setWebsiteDataStore:"), objc.ID(objc.GetClass("WKWebsiteDataStore")).Send(sel("nonPersistentDataStore")))
	web := objc.ID(objc.GetClass("WKWebView")).Send(sel("alloc")).Send(sel("initWithFrame:configuration:"), rect{S: size{W: 1280, H: 820}}, config)
	config.Send(sel("release"))
	if web == 0 {
		delegate.Send(sel("release"))
		return nil, fmt.Errorf("Studio: could not create WKWebView")
	}
	web.Send(sel("setNavigationDelegate:"), delegate)
	web.Send(sel("setAutoresizingMask:"), uint64(18))
	window := objc.ID(objc.GetClass("NSWindow")).Send(sel("alloc")).Send(sel("initWithContentRect:styleMask:backing:defer:"), rect{S: size{W: 1280, H: 820}}, uint64(1|2|4|8), uint64(2), false)
	window.Send(sel("setReleasedWhenClosed:"), false)
	window.Send(sel("setTitle:"), stringID("Omni Studio"))
	window.Send(sel("setMinSize:"), size{W: 860, H: 640})
	window.Send(sel("setDelegate:"), delegate)
	window.Send(sel("setContentView:"), web)
	app.Send(sel("setDelegate:"), delegate)
	menu := objc.ID(objc.GetClass("NSMenu")).Send(sel("new"))
	appItem := objc.ID(objc.GetClass("NSMenuItem")).Send(sel("new"))
	menu.Send(sel("addItem:"), appItem)
	appMenu := objc.ID(objc.GetClass("NSMenu")).Send(sel("new"))
	appItem.Send(sel("setSubmenu:"), appMenu)
	quitItem := objc.ID(objc.GetClass("NSMenuItem")).Send(sel("alloc")).Send(sel("initWithTitle:action:keyEquivalent:"), stringID("Quit Omni Studio"), sel("quit:"), stringID("q"))
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
	url := objc.ID(objc.GetClass("NSURL")).Send(sel("URLWithString:"), stringID(origin+"/"))
	request := objc.ID(objc.GetClass("NSURLRequest")).Send(sel("requestWithURL:"), url)
	web.Send(sel("loadRequest:"), request)
	window.Send(sel("center"))
	window.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
	app.Send(sel("activateIgnoringOtherApps:"), true)
	return func() {
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
