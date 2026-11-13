# Native desktop services

Use root `omnigo.Application` methods after readiness. These services currently
use the macOS host; Windows/Linux host implementations remain outstanding.
Call blocking methods from workers, never from a UI callback. `OnReady` can start
a worker. Contexts own operations/subscriptions, and application shutdown cleans
up outstanding native resources.

## Open, save and message dialogs

```go
selection, err := app.OpenDialog(ctx, omnigo.FileDialogOptions{
    Title: "Choose images",
    Multiple: true,
    Filters: []omnigo.FileFilter{{Name: "Images", MIMETypes: []string{"image/*"}}},
})
if err != nil {
    return err
}
if !selection.Canceled {
    // Read selection.Paths in application-owned code.
}

save, err := app.SaveDialog(ctx, omnigo.FileDialogOptions{
    Title: "Export report", DefaultPath: "/absolute/path/report.txt",
    Filters: []omnigo.FileFilter{{Extensions: []string{"txt"}}},
})
if err != nil {
    return err
}
if !save.Canceled {
    // save.Paths contains one destination. The dialog does not write the file.
}

answer, err := app.MessageDialog(ctx, omnigo.MessageDialogOptions{
    Kind: "warning", Message: "Remove this item?",
    Detail: "This action cannot be undone.",
    Buttons: []string{"Remove", "Cancel"}, DefaultButton: 1, CancelButton: 1,
    DestructiveButtons: []int{0},
})
if err != nil {
    return err
}
if answer.Response == 0 {
    // Perform the action in application-owned code.
}
```

- Empty `Parent` chooses the key/main native window. For Go content windows,
  specify their factory ID and `ParentDark` to select its light/dark variant.
  Unknown or closed parents fail; they do not silently attach elsewhere.
- `Directory` selects directories in an open dialog; files are the default.
  Save rejects `Directory` and `Multiple`. `ShowHiddenFiles`,
  `CreateDirectories`, `TreatPackagesAsDirectories`, `ButtonLabel`, `Message`
  and `NameFieldLabel` configure native panels.
- Filters combine extensions and MIME types into one native allowed-type list.
  A leading dot is optional; `*` / `*/*` allow every type. Common MIME wildcards
  (`image/*`, `audio/*`, `video/*`, `text/*`, `font/*`) map to native UTTypes.
  Unresolvable types return an error. Filter names do not create a dropdown.
  UTType filters and destructive buttons require macOS 11+.
- User cancellation returns `Canceled:true`; context cancellation returns its
  error and schedules UI cleanup even when the ordinary work queue is full.
  The operation may return before the native dismissal animation completes.
- One application dialog is allowed at a time. Concurrent presentation returns
  `host.ErrDialogBusy`. Parent closure dismisses its sheet; shutdown releases
  outstanding callers and native references.
- Message buttons use zero-based indices. Empty buttons become `OK`.
  `CheckboxLabel` and `CheckboxChecked` configure a native suppression checkbox;
  the result includes its final state. The cancel button gets an Escape shortcut
  when distinct from the default Return button. Escape when both indices are
  identical remains native behavior and has not been interactively validated.
- File chooser confirmation, overwrite prompts, sandbox bookmarks and all
  keyboard/accessibility interactions are not covered by the automatic probe.

## Tray / menu bar

```go
tray, err := app.NewTray(ctx, omnigo.TrayOptions{
    ID: "com.example.myapp.status", Title: "My App",
    ToolTip: "My App is running",
    // Icon: embeddedPNG, Template: true,
    Menu: []omnigo.MenuItem{
        {ID: "open", Label: "Open"},
        {Separator: true},
        {ID: "settings", Label: "Settings", Items: []omnigo.MenuItem{
            {ID: "notifications", Label: "Notifications", Checked: true},
        }},
    },
    OnEvent: func(event omnigo.TrayEvent) {
        // UI callback: enqueue work, or start a worker for blocking APIs.
        if event.Kind == "menu" && event.ItemID == "open" {
            _, _ = app.Activate(ctx) // Enqueues without waiting.
        }
    },
})
if err != nil {
    return err
}
state, err := tray.State(ctx) // Bounds, native dark appearance, visibility.
_ = state
if err != nil {
    return err
}
// tray.Update(ctx, fullReplacementOptions) updates title/icon/menu/callback.
// tray.PopUpMenu(ctx) tracks the current menu until it closes.
// tray.Close(ctx) removes the status item; application shutdown also removes it.
```

IDs are stable and unique among live trays. Updates replace the entire snapshot
and cannot change the ID. PNG icons are copied, limited to 4 MiB / 4096 pixels,
and displayed at 18 logical pixels. Template icons follow the native bar theme.
Menus support nested items, separators, disabled/hidden items, checkmarks and
tooltips. Item IDs must be unique across the tree. Checked state is application
controlled; selecting an item does not silently mutate it.

With a menu, AppKit handles opening it and `OnEvent` reports selections. Without
a menu, it reports clicks, native button number and click count. Callbacks run
on the UI thread. Menu roles, accelerators, radio groups, per-item icons and
RTL configuration are not implemented by this API yet. A tray does not change
the application's policy of exiting when its last native window closes.

## System event subscriptions

```go
subscription, err := app.WatchSystemEvents(ctx)
if err != nil {
    return err
}
defer subscription.Close() // Nonblocking and safe from a UI callback.
for event := range subscription.Events {
    if event.Err != nil {
        // Report native snapshot errors.
        continue
    }
    switch event.Kind {
    case omnigo.SystemDisplaysChanged:
        // event.Screen is a fresh detached snapshot.
    case omnigo.SystemAppearanceChanged:
        // event.Appearance includes effective theme and accessibility settings.
    case omnigo.SystemWillSleep, omnigo.SystemDidWake:
    case omnigo.SystemScreensDidSleep, omnigo.SystemScreensDidWake:
    case omnigo.SystemSessionActive, omnigo.SystemSessionInactive:
    }
}
```

Query `Screen` and `Appearance` for initial state. Native observers are installed
on the first subscription and removed after the last one closes. Context
cancellation and shutdown close the channel. App appearance uses effective-
appearance KVO; display and workspace events use native notification centers.
These are observations, not sleep inhibitors or guaranteed pre-sleep work time.

Pending display/appearance notifications are coalesced. Other notification order
is retained within a bounded 64-signal native queue. Each subscription buffers
16 events and drops its oldest event on overflow. `Sequence` and cumulative
`Dropped` expose loss; this is not a durable log. Slow consumers never block the
UI and do not retain unlimited snapshots. A consumer that needs current state
should re-query it after overflow. Go themes and Web frontend CSS remain owned
by the application; subscriptions do not rewrite either.

## Verification

```sh
CGO_ENABLED=0 go run ./examples/studio/cmd/desktopcheck
sh scripts/check.sh
```

The diagnostic opens its own window/sheets and status item, then removes them.
It verifies native cancellation, alert response/checkbox, parent closure,
context cleanup, tray state/update/menu action/removal, real app appearance KVO,
and synthetic display/wake notifications. It does not change system preferences,
put the machine to sleep, or write files selected by a dialog.

See `migration/noncontrols/desktop-services.md` for platform limits and evidence.
