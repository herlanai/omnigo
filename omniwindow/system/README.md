# System integration

`OpenURL(ctx, url)` accepts explicit HTTP, HTTPS and mailto URLs. `OpenPath(ctx, path)` resolves and checks an existing local path. Both ask the user's configured handler to open the target; they do not embed it in a WebView.

- Windows amd64/arm64: native `ShellExecuteW`, with balanced COM initialization on a locked OS thread. Reference: https://learn.microsoft.com/windows/win32/api/shellapi/nf-shellapi-shellexecutew
- macOS amd64/arm64: `/usr/bin/open`.
- Linux amd64/arm64: `xdg-open` or `gio open`; these require a working graphical session and installed desktop utilities. Absence returns `ErrUnavailable`.

No shell command concatenation, Python, JS, CGO or browser engine is introduced. Tests validate rejected schemes, single-argument preservation and missing-handler behavior without opening applications or reading user data. Windows/Linux native execution is not yet validated on those systems. This package does not implement windows, file dialogs, tray, notifications or the clipboard.

`Reveal(ctx, path)` shows and selects an existing item: Finder `open -R`, Windows
`SHOpenFolderAndSelectItems`, or Linux `org.freedesktop.FileManager1.ShowItems`
through `gdbus`. Linux requires a session bus and a supporting file manager.
Missing handlers return errors; there is no parent-folder-only fallback. Paths
are absolute, passed without shell interpolation, and symlinks are not resolved.
Cancellation is checked before launch; it cannot undo an already issued OS request.
Tests cover invalid/missing paths, cancellation and argument/URI preservation.
Native selection acceptance on all three platforms remains outstanding.

`Trash(ctx, path)` moves one existing item to the OS trash. macOS uses
NSFileManager; Windows uses IFileOperation with explicit recycling and undo
flags (Windows 8+); Linux requires `gio trash`, which maintains desktop trash
metadata. No permanent-delete fallback is implemented. Empty paths and filesystem
roots are rejected; symlinks are not resolved. Errors/aborted shell operations are
reported. Cancellation is checked before submission and cannot reverse a completed
move; Windows shell confirmation may require interaction. macOS ARM64 native round trips have passed for files, directories, symlinks and
dangling symlinks, checking bytes and preserving link targets; restoration uses
the OS-returned destination, not Finder Put Back. Windows/Linux native acceptance,
Intel Mac, cross-volume and unsupported-volume behavior remain pending. Tests use only owned temporary files; the explicit native tests exercise real
trash/restore while default tests cover validation/cancellation without OS moves.

Windows operation flags follow the [Microsoft IFileOperation contract](https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nf-shobjidl_core-ifileoperation-setoperationflags).


Native macOS verification (only creates, trashes and restores test-owned items):

```sh
OMNIGO_NATIVE_TEST=1 CGO_ENABLED=0 go test ./omniwindow/system -run TestNativeTrash -v
```

## Standard application locations

`NewPaths("MyApp")` creates a concurrent-safe resolver without creating directories.
`Get(PathUserData)`, `Get(PathCache)` and `Get(PathLogs)` create their default
application-scoped directories with mode 0700 (subject to the OS/filesystem).
`PathAppData` follows `os.UserConfigDir`, matching the reference contract rather
than Linux's differently named XDG data directory. User folders are queried but
never created. Windows uses native KnownFolder paths, including OS redirection;
Linux reads `user-dirs.dirs` with bounded parsing and no shell evaluation.

`Set(name, path)` records an absolute override without IO; an empty string restores
the default. Overrides do not rewrite other locations. Existing directory modes
are not changed. macOS Resources points to Contents/Resources inside a bundle;
Go temporary build executables use the current project's resources directory.
Other installed executables use their containing directory. This locates files;
it does not copy/package them or guarantee they exist.

The application facade is `Options.Name`, `app.Name()`, `app.Path(omnigo.PathCache)`
and `app.SetPath(...)`. Supply a stable Name for production; otherwise the current
executable's basename is used. Names must be single portable path components.
