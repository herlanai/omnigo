# OmniGo Application Development Guide

Use this guide when developing the framework, examples, or applications built with OmniGo. Prefer existing public APIs so application code describes UI and business behavior. Explicit user requirements for platforms, interface technologies, and deliverables take precedence over the defaults below.

## 1. Default Deliverables and Workflow

- Default target: **macOS Apple Silicon (darwin/arm64)**. Claim Windows or Intel support only when requested and validated on the corresponding platform.
- Default deliverables: an `.app` with its own name, bundle identifier, and icon, plus a `.dmg` with an Applications shortcut for drag-and-drop installation. A `.zip` archive may also be included.
- Choose the interface approach first, implement windows and application behavior, then finish icons, builds, packaging, and native runtime validation. Source code or a bare Go executable alone does not complete an application delivery.
- Keep the root `omnigo` entry point simple. Prefer `omnigo.Run(Options)`; use `omnigo.New` and `app.RunMain()` when configuration or window operations require an application instance.
- Keep business logic in application-owned packages. Framework code must not contain demo IDs, demo pages, product copy, or application-specific icons.
- Requires Go 1.26+. Ordinary Go control applications do not need Node.js. Building the Studio frontend requires Node.js 24+ and pnpm 10.33.

## 2. Choose the Interface Approach

| Requirement | Approach | Current limits |
| --- | --- | --- |
| Write the entire interface in Go | `Options.GoWindow` + `omniui/baseui` | Public host primarily validated on macOS arm64; CPU rendering |
| Use a React / TypeScript frontend in a desktop app | `Options.Assets` + `Options.Window` + OmniWeb | macOS main window uses WKWebView; the framework hosts resources |
| Embed a page inside a Go layout | `ContentWindowOptions.WebView` + `baseui.WebView` | Real WKWebView; the current per-window configuration is not a multi-tab API |
| Open the frontend in a system browser | OmniWeb project template | Standalone HTTP application; does not create a desktop window automatically |
| Use a standalone Chromium browser | `omniui/plugins/chromium` | Separate CEF host with additional runtime and native toolchain requirements |

A Web frontend does not implement Go controls. WKWebView and Chromium are distinct runtimes. Choose according to the task; do not include the larger CEF runtime by default.

## 3. Window Design Defaults

Prefer a clean window with a custom top area. For a Web main window, start with:

```go
Window: &omnigo.WindowOptions{
    Title:         "My App",
    Width:         1200,
    Height:        800,
    MinWidth:      800,
    MinHeight:     560,
    TitleBarStyle: omnigo.TitleBarHidden,
},
```

- Use the product name as the title. Adjust dimensions to the content and maintain clear spacing, hierarchy, and contrast in both themes.
- `TitleBarHidden` hides the system title, background, and window buttons, extending content to the top. Provide discoverable close/quit behavior, preserve system menus and keyboard actions, and test them natively.
- To retain macOS traffic lights, use `TitleBarHiddenInset`, optionally with `TrafficLightPosition: &omnigo.Point{X: 12, Y: 11}`. Reserve space using `TopInset` returned by `app.TitleBarState(ctx)`.
- CSS drag declarations do not establish native WKWebView window dragging. Do not claim dragging works solely because `-webkit-app-region: drag` is present. Check host support and test it; use a style that retains native window interaction areas when necessary.
- **Go content windows do not have a `TitleBarStyle` field.** Use `ContentWindowOptions.TitleBarHidden: func() bool { return true }`, and set `DragRegion` to an actual Go element ID.
- `app.TitleBarState` reads state; `app.SetTitleBar` changes it and requires an explicit style. These methods and `OpenContentWindow` are worker-facing calls. Never synchronously wait on the UI's own dispatch queue from a UI callback.
- Web main-window minimum dimensions do not imply dynamic resizing support for fixed Go canvases. Do not invent unsupported window properties or cross-platform behavior.

## 4. Native Go Controls

Import `github.com/herlanai/omnigo/omniui/baseui`, usually as `ui`. Keep element IDs stable and unique within a window. Initialize mutable state in the window factory; let `View` build the interface from that state. Do not recreate text buffers or reset state on every View call.

This complete entry point assumes `assets/app-icon.png` exists beside `main.go`:

```go
package main

import (
    _ "embed"
    "fmt"
    "log"

    "github.com/herlanai/omnigo"
    ui "github.com/herlanai/omnigo/omniui/baseui"
)

//go:embed assets/app-icon.png
var appIcon []byte

func main() {
    err := omnigo.Run(omnigo.Options{
        Icon: appIcon,
        GoWindow: func(env omnigo.WindowEnvironment) (omnigo.ContentWindowOptions, error) {
            count := 0
            return omnigo.ContentWindowOptions{
                Title: "My App", Width: 800, Height: 560,
                TitleBarHidden: func() bool { return true },
                DragRegion: "heading",
                View: func() *ui.Element {
                    return ui.Column("root",
                        ui.Heading("heading", "My App", 1),
                        ui.Text("count", fmt.Sprintf("Count: %d", count)),
                        ui.Button("add", "Add one", func() { count++ }),
                    ).Padding(24).Gap(16)
                },
            }, nil
        },
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

- Reuse existing controls and themes, including `Button`, `Input`, `Row`, `Column`, and `Card`. Verify properties against the actual API.
- Surface, control state, text buffers, and native host operations belong to their UI goroutine. Use `env.RunTask` for background work and return a callback that applies results on the UI. Workers must not mutate UI state directly.
- Long-running work must cooperate with cancellation. `RunTask` cannot forcibly terminate arbitrary Go code. Register application-wide work through `omnigo.Background` / `Service` in `Options.Services`.
- Applications do not need to create their own Surface, native event loop, or frame encoder. Never retain a Painter across frames.
- Examples: [counter](examples/desktop-counter/main.go), [control showcase](examples/control-showcase/main.go), and [control documentation](omniui/baseui/README.md).

## 5. Desktop Applications with a WebView Frontend

Embed built static frontend assets in Go and let the framework create the resource server and native main window. Ordinary desktop applications should not reimplement listeners, HTTP lifetimes, or AppKit event loops.

This complete entry point assumes `web/index.html` and `assets/app-icon.png` exist beside `main.go`. In a real project, copy frontend build output into `web/` before building Go:

```go
package main

import (
    "embed"
    "io/fs"
    "log"

    "github.com/herlanai/omnigo"
)

//go:embed all:web
var frontend embed.FS

//go:embed assets/app-icon.png
var appIcon []byte

func main() {
    assets, err := fs.Sub(frontend, "web")
    if err != nil {
        log.Fatal(err)
    }
    err = omnigo.Run(omnigo.Options{
        Assets: assets,
        Icon: appIcon,
        Window: &omnigo.WindowOptions{
            Title: "My App", URL: "/",
            Width: 1200, Height: 800, MinWidth: 800, MinHeight: 560,
            TitleBarStyle: omnigo.TitleBarHidden,
        },
        Registrations: []omnigo.Registration{
            omnigo.Query("app.version", func() string { return "0.1.0" }),
        },
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

- `Window.URL` is an application-relative path such as `/`; do not supply an external website URL here.
- Use `OmniWebClient` / `HTTPTransport` from the canonical SDK in `omniweb/client/index.ts`. Do not maintain another protocol implementation. Keep frontend type mappings aligned with Go request/response types.
- Use `Query` for no-argument queries and generic `Method` for request handlers. Specify permission names explicitly when needed and configure trusted grants through `omnigo.LifecycleOptions.Permissions`.
- RPC handlers process requests off the UI goroutine. Dispatch any UI state access; do not substitute unsynchronized cross-goroutine reads or writes.
- Production applications embed frontend output and do not require Vite, a Node server, or the source working directory. Separate development tools from distribution assets.
- See the complete [Studio entry point](examples/studio/cmd/studio/main.go) and [service registrations](examples/studio/cmd/studio/services.go).

### A WebView Inside a Go Layout

Set `ContentWindowOptions.WebView` to `&host.WebViewOptions{HTML: ..., OnStateReady: ..., OnReady: ...}`, importing `host` from `omniwindow/host`. Save the `*ui.WebViewState` supplied to `OnStateReady`, then place it with `ui.WebView("page", state)`. Show placeholder content before state is available rather than passing nil.

`OnReady` supplies a `host.WebPage` controller with `LoadHTML`, `Back`, `Forward`, and `SetZoom`. Call it only on the UI goroutine and stop using it after closure. The host owns page lifetime. Do not assume this controller already provides JavaScript evaluation, a bidirectional bridge, persistent tabs, or full overlay compositing.

See the [WebView demo](examples/studio/internal/demos/webview.go) and [window configuration](examples/studio/internal/demos/window.go). External applications may learn from these examples but cannot import Studio's `internal` packages.

## 6. Browser Applications

### System Browser + OmniWeb

From the repository root, generate a standalone Web project. The destination must not already exist:

```sh
go run ./cmd/omni init --framework "$PWD" --module example.com/myapp /tmp/my-omniweb-app
cd /tmp/my-omniweb-app
npm --prefix ui ci
npm run build:web
npm start
```

Open `http://127.0.0.1:7331`. This template creates a browser application, not an `.app` desktop host. For desktop distribution, adopt the WebView application entry point above. External Web hosts use `omniweb.NewRouter`, `omniweb.Bind`, and `omniweb.NewHTTP`; they cannot import `internal/runtime/rpc`.

### Standalone Chromium / CEF

From the repository root:

```sh
pnpm setup:chromium
pnpm dev:browser --url https://example.com
# Content-only application window:
pnpm dev:browser --app --url https://example.com
```

- Use `chromium.Launch(ctx, chromium.Options{...})` from Go. The context owns the session. On exit, call `Close` with a separate bounded cleanup context.
- Install the host first. `HostPath` or `OMNIGO_CHROMIUM_HOST` must point to its executable. Do not copy only the executable while omitting the CEF Framework, helpers, or resources.
- Building CEF requires CMake and a C++/Objective-C++ toolchain. This is separate from the Go targets' `CGO_ENABLED=0` requirement.
- Production applications use persistent profiles. Temporary profiles require an explicit development/testing choice. Never carry test-only security-storage bypasses into production.
- Report real startup failures instead of silently substituting a WebView. Chromium currently runs as a separate window, not a control embedded in Go Row/Column layouts.
- See the [Chromium entry point](examples/chromium/main.go) and [plugin documentation](omniui/plugins/chromium/README.md).

## 7. Application Icons: Runtime and Bundle

1. Provide product-specific artwork, preferably SVG and a 1024×1024 PNG. Do not default to Studio's branding.
2. Embed the PNG with `//go:embed` and assign it to `omnigo.Options.Icon` for the runtime application icon.
3. Place `AppIcon.icns` in `.app/Contents/Resources/` for Finder and distribution. Set Info.plist's `CFBundleIconFile` accordingly.
4. Keep the product name, `CFBundleExecutable`, bundle identifier, version, icon, and package filename consistent.

Inside this repository, reuse the icon converter with explicit application paths:

```sh
go run ./examples/studio/cmd/appicon \
  -source /absolute/path/to/my-app/assets/app-icon.svg \
  -out /absolute/path/to/my-app/build/AppIcon.icns \
  -preview /absolute/path/to/my-app/assets/app-icon.png
```

This is a repository tool, not a required dependency of external applications. Verify icons in the Dock, Finder, and after restarting the app.

## 8. Default Packaging: macOS arm64

### Package Studio

On a macOS arm64 development machine, from the repository root:

```sh
pnpm install --frozen-lockfile
# Only if the application needs Chromium; verify the runtime before packaging.
pnpm setup:chromium
CGO_ENABLED=0 pnpm package
```

The current script packages for the host architecture. Do not label output built on an Intel host as arm64. Confirm `go env GOHOSTOS GOHOSTARCH` reports `darwin arm64`, and inspect the architecture of both the main executable and any CEF host.

Outputs are `build/studio/Omni Studio.app` and `build/studio/omnigo-studio-darwin-arm64.zip`. **The existing script does not create a DMG and is not a generic application packager.** New applications should own their build scripts, using `scripts/devtool/studio.go` as a reference. Renaming Studio is not a substitute for product-specific packaging configuration.

### New Application Bundles and DMGs

```text
My App.app/
└── Contents/
    ├── Info.plist
    ├── MacOS/my-app
    └── Resources/
        ├── AppIcon.icns
        └── licenses/
```

- Build frontend assets, if any, and icons before compiling Go. Generate Info.plist with at least the bundle identifier, name, version, executable, icon, `CFBundlePackageType=APPL`, and high-resolution declaration.
- An example build command in the application repository is `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -o "build/My App.app/Contents/MacOS/my-app" ./cmd/my-app`. Use actual product configuration for the entry point, name, and version instead of these placeholders.
- Include the licenses for third-party components actually used. When using CEF, bundle its complete runtime, matching architecture, and licenses.
- Ad-hoc signing is suitable for local validation. Public distribution requires the user's Developer ID signing and notarization workflow. Never describe an ad-hoc package as notarized.

Once the `.app` has been built, populated with resources, and appropriately signed, create a DMG from the application project root. The following names are placeholders:

```sh
# Use temporary staging to avoid overwriting the app or deleting other artifacts.
staging_dir=$(mktemp -d "${TMPDIR:-/tmp}/omnigo-dmg.XXXXXX")
ditto "build/My App.app" "$staging_dir/My App.app"
ln -s /Applications "$staging_dir/Applications"
hdiutil create -volname "My App" -srcfolder "$staging_dir" \
  -ov -format UDZO "build/my-app-darwin-arm64.dmg"
# Inspect staging_dir before removing this temporary directory.
```

Inspect package contents and architecture, then launch the `.app` after copying it to a temporary installation location. Testing only `go run` from the source checkout is insufficient. Verify independence from Node and source files, icons, window interactions, resource resolution, and cleanup of child processes and tasks. Report artifact paths and actual validation results in the final response.

## 9. Framework Engineering Rules

- Public application entry points belong to the root package; platform/window capabilities to `omniwindow`; Go controls to `omniui/baseui`; the Web protocol to `omniweb`; Studio to `examples/studio`.
- Keep root Go files focused on the public `omnigo` API and `Application` composition. Application HTTP hosting and single-instance handoff belong to `internal/apphost`; generated-file publication to `internal/codegen`; cooperative background work to `internal/runtime/lifecycle`. Keep implementation tests with their packages and repository dependency checks in `tests/architecture`. Do not move public types or methods merely to reduce root file count.
- `internal/runtime` and `internal/text` are implementation details. External applications must use public entry points such as `omnigo.LifecycleOptions`, `omniweb.NewRouter`, and `baseui.TextSnapshot`. Framework code may use `internal/runtime/dispatch` for cross-goroutine work.
- Implement control behavior, layout, themes, painting, compositions, and control plugins in Go. Do not replace Go controls with React/TSX or WebViews. Application Web frontends may use React/TypeScript as required.
- Before control work, read `omniui/baseui/README.md` and `migration/controls/README.md`. Preserve capability audits. API names, static screenshots, aliases, and successful builds do not establish complete behavioral coverage.
- The local reference repository is read-only. Do not copy implementation bodies, rename and import its framework, or add it as a dependency. Preserve licenses for third-party libraries and assets actually used; existing explicitly authorized ports retain their directory-specific licensing requirements.
- All Go targets must build with `CGO_ENABLED=0`. Do not force optional GPU, CEF, or service-plugin dependencies into applications that do not use them.
- Test interactions with deterministic events and explicit clocks, not sleeps. Validate disabled/read-only behavior, focus, cancellation, IME composition, bounds, resource lifetime, and dark/light rendering as relevant.
- Run `sh scripts/check.sh` for substantive core changes: Go tests, vet, race detection, SDK tests, and six desktop cross-compilation targets. Changes to generated projects or public APIs must also be validated from an independent external Go module.
- Cross-compilation does not establish native compatibility. Report incomplete behavior and measurement limits accurately. Never describe partial controls as full coverage or claim speedups without equivalent benchmarks.
