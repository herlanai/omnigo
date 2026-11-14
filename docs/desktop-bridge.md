# Explicit desktop service bridge

Register `omnigo.DesktopBridge()` in `Options.Registrations`. Grant only the
permissions the trusted application frontend needs in
`Options.Lifecycle.Permissions`: `desktop.read` and/or `desktop.dialog`.
Registration alone does not grant access.

Generate the application's TypeScript contracts with `omni generate`, then use
the canonical `OmniWebClient<Services>` and HTTPTransport. Desktop handlers are
ordinary typed RPC methods; no second serialization protocol is introduced.

| Method | Permission | Request |
| --- | --- | --- |
| desktop.platform | desktop.read | {} |
| desktop.screen | desktop.read | {} |
| desktop.appearance | desktop.read | {} |
| desktop.titlebar | desktop.read | {} |
| desktop.dialog.open | desktop.dialog | FileDialogOptions |
| desktop.dialog.save | desktop.dialog | FileDialogOptions |
| desktop.dialog.message | desktop.dialog | MessageDialogOptions |

Dialog request field names follow the Go types (capitalized where no JSON tags
are present); use generated contracts rather than hand-written wire mappings.
RPC workers dispatch native work through the existing UI queue and propagate
request cancellation. Native dialogs do not read or write the selected file.
The platform query describes implementation status, not machine readiness.

This bridge exposes existing services only. Windows/Linux WebView hosts,
clipboard forwarding, window-ID management, mutation APIs and native transport
are not provided by this registration. Existing macOS host constraints remain.
Do not grant desktop permissions to untrusted web content.
