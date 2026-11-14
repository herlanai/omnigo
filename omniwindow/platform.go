package omniwindow

import "runtime"

type PlatformSupport struct {
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	CoreBuild    bool   `json:"coreBuild"`
	NativeWindow string `json:"nativeWindow"`
	WebView      string `json:"webView"`
	Evidence     string `json:"evidence"`
}

// Platform describes implemented drivers, not just accepted GOOS/GOARCH values.
func Platform(goos, goarch string) PlatformSupport {
	p := PlatformSupport{OS: goos, Arch: goarch, NativeWindow: "unavailable", WebView: "unavailable", Evidence: "No native window host"}
	if goarch != "amd64" && goarch != "arm64" {
		return p
	}
	switch goos {
	case "darwin":
		p.CoreBuild = true
		p.NativeWindow = "experimental-cocoa"
		p.WebView = "experimental-wkwebview"
		p.Evidence = "Cross-build only; native Intel machine not yet verified"
		if goarch == "arm64" {
			p.Evidence = "AppKit titlebar and WKWebView native probes; full IME/accessibility still incomplete"
		}
	case "windows", "linux":
		p.CoreBuild = true
		p.NativeWindow = "experimental-win32-software"
		if goos == "linux" {
			p.NativeWindow = "experimental-x11-software"
		}
		p.Evidence = "Optional omniwindow/nativewindow driver; cross-build only, no native runtime verification. Studio WebView host unavailable"
		if goos == "windows" && experimentalWebView2 {
			p.WebView = "experimental-webview2"
			p.Evidence = "Opt-in Win32/WebView2 main window and native RPC; matching loader/runtime required; native Windows verification pending"
		}
		if goos == "linux" && experimentalWebKitGTK {
			p.WebView = "experimental-webkitgtk6"
			p.Evidence = "Opt-in GTK4/WebKitGTK 6 Web main window; runtime libraries/display required; native validation pending; Go content windows and desktop services unsupported"
		}
	}
	return p
}
func CurrentPlatform() PlatformSupport { return Platform(runtime.GOOS, runtime.GOARCH) }
func Platforms() []PlatformSupport {
	var out []PlatformSupport
	for _, os := range []string{"darwin", "windows", "linux"} {
		for _, arch := range []string{"amd64", "arm64"} {
			out = append(out, Platform(os, arch))
		}
	}
	return out
}
