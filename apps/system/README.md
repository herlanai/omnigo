# System integration

`OpenURL(ctx, url)` accepts explicit HTTP, HTTPS and mailto URLs. `OpenPath(ctx, path)` resolves and checks an existing local path. Both ask the user's configured handler to open the target; they do not embed it in a WebView.

- Windows amd64/arm64: native `ShellExecuteW`, with balanced COM initialization on a locked OS thread. Reference: https://learn.microsoft.com/windows/win32/api/shellapi/nf-shellapi-shellexecutew
- macOS amd64/arm64: `/usr/bin/open`.
- Linux amd64/arm64: `xdg-open` or `gio open`; these require a working graphical session and installed desktop utilities. Absence returns `ErrUnavailable`.

No shell command concatenation, Python, JS, CGO or browser engine is introduced. Tests validate rejected schemes, single-argument preservation and missing-handler behavior without opening applications or reading user data. Windows/Linux native execution is not yet validated on those systems. This package does not implement windows, file dialogs, tray, notifications or the clipboard.
