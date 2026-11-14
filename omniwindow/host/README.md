# Public UI application host

`omniwindow/host` owns Cocoa application/windows, input forwarding, frame submission, same-origin WebKit shells, embedded WebViews, titlebar changes, dispatch and teardown. It does not import Studio, know demo IDs, contain demo HTML, or supply application icons. Applications provide `Options`, a `WindowFactory` and `Content` callbacks.

Most applications use root `omnigo.New`, `omnigo.Bind`, `app.ConfigureMainWindow` and `app.Run`. That entry point also composes the existing desktop extension lifecycle, OmniWeb assets/RPC and loopback resource server. `host.Run` is available for hosts that already own their resource server or need a native-only application.

- Call Run from the process main thread. The current Cocoa backend is single-use per process.
- Window factories, View, BeforeRender, Close and titlebar getters run on the UI goroutine. They must not block on Host.Open/SetTitleBar, which queue work to that same goroutine.
- Services.RunTask runs work in a worker and dispatches the returned callback to the live content window. Workers must cooperate with application cancellation; arbitrary Go work is not forcibly terminated.
- Each content window owns a Surface and application callbacks; the framework owns native handles, timers and a shared reusable RGBA framebuffer. Application Close runs before Surface teardown.
- Content windows currently have fixed canvas dimensions. The shell supports initial/minimum size and titlebar options. This is not the full nativewindow API or complete golet parity.
- HTML fixtures and WebView state consumers are supplied by the application. Embedded positioning uses the actual parent view height.
- macOS amd64/arm64 is implemented; only ARM64 native evidence is available. Windows/Linux return ErrUnsupported here; their separate software driver remains in omniwindow/nativewindow.

Public-API examples: `go run ./examples/desktop-counter`; `go run ./examples/desktop-smoke` briefly opens a window and verifies rendering/closure without sleeps. Titlebar and WebView probes remain available through the public package and Studio diagnostic commands. Unit tests guard the absence of framework-to-Studio dependencies.

Root application conveniences: `omnigo.Options.GoWindow` accepts a
`func(omnigo.WindowEnvironment) (omnigo.ContentWindowOptions, error)` for a single native main
window, without a string-keyed factory. The context exposes window-scoped task
and clipboard services. `RunMain()` handles SIGINT/SIGTERM; `Run(ctx)` remains
available to hosts that own cancellation. Both must run on the process main thread.

`Options.Registrations` registers typed RPC methods/routes during construction.
Modules should only configure the application, not start resources.
`Options.Services` uses the existing desktop extension lifecycle: namespaced IDs,
deterministic ID order, cancellation before reverse-order cleanup, and a bounded
shutdown context. `omnigo.Background(id, work)` starts cooperative background
work and joins it on exit; failures are returned at shutdown, not automatically
used to terminate the application. Work must honor cancellation. Each background
adapter is single-use. Services with ordering dependencies should continue to use
the extension registry's Requires mechanism.

These conveniences do not unify native and Web window handles or add unsupported
platform hosts. `GoWindow` conflicts with InitialWindow/CreateWindow; applications
with multiple content types can continue using the existing factory.

For a small Web application, declare `Options{Assets: files, Window:
&omnigo.WindowOptions{Title: "Example"}, Registrations: []omnigo.Registration{
omnigo.Query("app.version", func() string { return "1.0" }),
}}` and call `omnigo.Run(options)` once. Empty window URL defaults to `/`.
For Go content, use GoWindow instead of Window. Method/Query registration stays
statically typed and shares Bind's permissions, strict JSON validation and limits.
New/Run(ctx) remain available when explicit construction or cancellation is needed.

Native dialogs (`OpenDialog`, `SaveDialog`, `MessageDialog`), application-owned
tray items (`NewTray`) and bounded system subscriptions (`WatchSystemEvents`)
are exposed through root `omnigo.Application`. See
[desktop service usage and limits](../DESKTOP.md). Run the opt-in native probe
with `CGO_ENABLED=0 go run ./examples/studio/cmd/desktopcheck`.

### HTML file inputs on macOS

Both the main WKWebView and embedded Go-layout WebViews install a shared
`WKUIDelegate` implementation for HTML file inputs, including custom buttons that
synchronously call `input.click()`. It opens an asynchronous `NSOpenPanel` sheet,
forwards multiple/directory selection, and passes selected NSURLs back to WebKit.
Cancel, an occupied parent sheet, or WebView closure resolves with no selection.
The host retains WebKit's completion block until exactly one resolution.

File type and size validation remains application-owned; WKOpenPanelParameters
contains no HTML `accept` filter. See [the local upload fixture](../../examples/webview-upload/README.md).
Native lifecycle regression (macOS GUI session, executed on the process main thread):

```sh
OMNIGO_NATIVE_UPLOAD_TEST=1 CGO_ENABLED=0 go test ./omniwindow/host -count=1 -v
```

The native probe covers the embedded host's installed delegate, input options,
busy-sheet cancellation, repeated cancellation, and cleanup when the view closes.
The interactive fixture verifies real FileList delivery and file reads; no file is
sent to a remote service. This does not establish Windows/Linux WebView support.


## Experimental Linux Web main window

Build with `CGO_ENABLED=0 go build -tags omnigo_webkitgtk` to select the
GTK4/WebKitGTK 6 backend on linux/amd64 or linux/arm64. It dynamically requires
libgtk-4.so.1, libwebkitgtk-6.0.so.4, GLib and GObject, plus a usable desktop display.
Missing libraries/symbols/display produce an error. The default Linux build still
uses ErrUnsupported for this host; no silent fallback is performed.

The current path creates one Web main window, loads the existing HTTP resource
server, rejects cross-origin top-level navigation and new windows, dispatches
worker requests and cleans up on context cancellation/window close. It uses the
system titlebar. Custom titlebars, Go content factories, native menus, desktop
service methods, explicit profile configuration and native message Transport
are not implemented for this backend. It does not bundle WebKit or disable its
sandbox. Native library handles remain loaded for process lifetime.

`CGO_ENABLED=0 go run -tags omnigo_webkitgtk ./examples/webview-smoke` requires
actual page JavaScript to complete a Unicode HTTP roundtrip before succeeding.
The Linux CI job runs it under Xvfb and D-Bus; this is not Wayland acceptance.
This backend has cross-build evidence only until that job or a desktop run is
observed passing. Do not infer native success from this documentation.


## Experimental Windows WebView2

`CGO_ENABLED=0 go build -tags omnigo_webview2 ./...` enables a Win32 Web main
window for Windows amd64/arm64. Deploy Microsoft's matching WebView2Loader.dll
beside the executable and install the WebView2 Evergreen Runtime. The loader is
loaded by absolute path, without silently falling back to a browser. The backend
uses STA COM callbacks, owns controller lifetime, resizes to the native client
rectangle, rejects cross-origin top-level navigation/popups, denies browser
permission prompts, and returns initialization/process failures to Go.

Native unary/batch RPC is available through `NativeExchange` and the canonical
TypeScript `NativeTransport`. Incoming message sources must match the application
origin; cancellation and window shutdown cancel pending RPC. Profiles persist in
the user's cache directory. Custom titlebars, Go content factories and native
desktop service methods are not implemented by this backend. This is opt-in and
has cross-build evidence only: no Windows test environment was available locally.
The Windows CI job deploys a pinned Microsoft loader and requires real JavaScript
→ Go → JavaScript completion; its result has not been observed yet.

## Native unary/batch Transport

WKWebView uses message handlers with reply; WebView2 uses WebMessageReceived and
PostWebMessageAsJson. Root applications automatically attach the existing router
and trusted grants. `new NativeTransport()` chooses the available native bridge;
`HTTPTransport` remains available. Both transports share the HTTP host's source
identity. Assets, events and streams still use HTTP. Linux currently has no native
Transport. Payload bounds are 1 MiB per request, 64 calls per batch and 4 MiB per
reply. No latency or memory improvement is claimed without equivalent benchmarks.
