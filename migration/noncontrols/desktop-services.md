# Display and appearance services

## Public usage

Call from a worker after `app.WhenReady(ctx)`; do not synchronously call these
methods from `OnReady`, a content factory, or any other UI callback:

```go
screens, err := app.Screen(ctx)
// screens.Displays contains IDs, labels, bounds, work areas, scale, rotation,
// internal/primary flags. screens.Cursor uses the same desktop coordinates.
appearance, err := app.Appearance(ctx)
appearance, err = app.SetAppearance(ctx, omnigo.AppearanceDark)
appearance, err = app.SetAppearance(ctx, omnigo.AppearanceSystem)
```

Handle each error in application code. Queries return detached snapshots. The
primary display's top-left corner is the desktop origin, measured in logical
pixels; displays above/left have negative coordinates. Work areas exclude native
reserved areas. Display IDs are not persistent device identities. The list is
queried on demand without a polling worker or cache that can become stale.

Native support currently requires the macOS host. Missing hosts return
`ErrNotReady`; unsupported controllers return `host.ErrUnsupported`. Queries
and mutations execute on the UI queue. Canceled callers do not read a result
that the UI may still be writing. Cancellation after a mutation begins cannot
undo the native change.

Appearance overrides belong only to `NSApplication`; `AppearanceSystem` removes
the override. The effective dark/light state and desktop reduce-motion,
reduce-transparency and high-contrast preferences are read from AppKit. This
does not rewrite Go surface themes or arbitrary frontend CSS. There is no
change subscription yet.

## Reference contract mapping

| Reference non-control contract | OmniGo | Status / remaining |
| --- | --- | --- |
| `Screen.Displays()` | `Application.Screen(ctx).Displays` | macOS native snapshot; other platforms pending |
| `Screen.CursorPoint()` | `Application.Screen(ctx).Cursor` | macOS snapshot, shared desktop coordinates |
| `Theme.IsDark()` | `Application.Appearance(ctx).Dark` | effective application appearance |
| `Theme.SetSource(source)` | `Application.SetAppearance(ctx, source)` | system/light/dark; rejects invalid sources |
| `Theme.Preferences()` | accessibility fields in `AppearanceState` | partial; accent and input/font preferences remain |
| `Theme.UIFont()` / `FontRendering()` | none | pending |
| Display / appearance change notifications | none | pending |

## Native acceptance, 2026-11-13

An independent temporary module required `github.com/herlanai/omnigo` with a
local replacement and used only public APIs. It created a short-lived WKWebView
window with `CGO_ENABLED=0`, queried from an `OnReady`-started worker, cycled
application dark/light/system appearance, and canceled the application context.
The process exited successfully after cleanup. Before readiness and after Run
returned, `Screen` returned `ErrNotReady`.

Observed on macOS arm64:

- Built-in Retina Display: ID 1, bounds `(0,0,1512,982)`, work area
  `(0,33,1512,897)`, scale 2, rotation 0, internal and primary.
- HP Zhan D9: ID 3, bounds `(1512,0,1920,1080)`, work area
  `(1512,30,1920,1050)`, scale 1, rotation 0, external and secondary.
- Dark override returned `Dark:true`; light returned `Dark:false`; system
  cleared the override and returned the then-current light appearance.
- Accessibility preferences were all false on this machine. Toggling the user's
  system settings, hotplug, rotated displays and Windows/Linux were not tested.

The first probe exposed an HTTP shutdown timeout from unused browser
preconnections. The server now closes connections still in `StateNew` at
shutdown, including connections accepted during shutdown; already-used
connections retain normal graceful shutdown. Deterministic tests cover state
transitions and concurrent acceptance/shutdown. The same native probe then
exited without the timeout.

Readiness now follows successful creation of the initial native window. It does
not imply that the WebView document has finished loading.

## Automated evidence

- `omniwindow/host/screen_test.go`: top-left transforms including above/left
  displays and menu/Dock insets; dispatch, snapshot isolation, cancellation
  during execution, shutdown and unsupported hosts.
- `omniwindow/host/appearance_test.go`: invalid source rejection, canceled
  mutation, UI dispatch, read/override/reset and shutdown.
- `screen_test.go`: public pre-readiness errors.
- `http_connections_test.go`: unused/active/idle/hijacked connections and
  concurrent shutdown. No sleeps or timing guesses are used.

These checks do not establish complete non-control parity or performance gains.
