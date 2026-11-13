# Application instance coordination

`Open(ctx, privateDirectory, applicationID)` acquires an OS lock. `ErrRunning`
means another process owns it; call `Notify` to forward `Launch` arguments,
working directory and activation token. The caller must supply a per-user private
directory (0700 on Unix; an appropriate user-only ACL on Windows). New Windows directories receive a protected inheritable DACL for the owner, SYSTEM
and administrators. Existing directories must be owned by the current user and
may not grant access to other principals, including inherit-only rules. Unknown
ACE kinds are rejected rather than treated as private. Native Windows acceptance
is still pending; cross-compilation does not validate runtime ACL behavior.

The lease owns a loopback listener and random authentication token, publishes its
endpoint atomically, and closes with its context or `Close`. The lock file is
intentionally retained: unlinking it would allow competing lock inodes. OS process
exit releases the lock; the next owner replaces stale endpoint metadata.

Messages are limited to 1 MiB and queued with capacity 32. `Notify` acknowledges
queue acceptance, not execution. Read `Messages()` and dispatch onto the UI;
never mutate UI state in a receiving worker. A full queue returns `ErrBusy`.
Connections have a bounded deadline. Cancellation after acceptance cannot retract
an already queued launch. There is no automatic duplicate-message retry.

This is the coordination layer, not complete application parity. OS foreground activation, deep-link/file
open dispatch and Windows native acceptance remain outstanding. Unix private
permissions and directory ownership are checked, but callers must also ensure trusted parent directories.
Tests cover competing leases, a real child process, launch forwarding, queue bounds,
cancellation and reacquisition after process exit without cleanup. Tests require
loopback sockets and touch only test-owned temporary directories.


The root application API integrates this module via `Options.SingleInstance`:

```go
SingleInstance: &omnigo.SingleInstanceOptions{
    ID: "com.example.myapp",
    OnLaunch: func(launch omnigo.Launch) error {
        // UI goroutine: consume arguments or update UI state.
        return nil
    },
},
```

The default directory is under the user cache. Configuration is snapshotted by
`New`. The first instance owns the lease before services start, releases it after
shutdown/rollback, and dispatches later launches through the host UI queue.
A secondary instance returns nil from Run after successful enqueue, without
starting services or windows. Transfer failure is returned; the application does
not silently start another instance. A handler error cancels the native lifetime
and is returned by Run. Automatic foreground activation and URL/file dispatch are
not yet implemented; the activation token is preserved in the launch payload.


The root application startup retries only `ErrNotReady` failures (missing endpoint
or connection failure before sending) while rechecking the OS lock on every
attempt. `StartupTimeout` defaults to five seconds and also observes caller
cancellation. An owner that dies before handoff can be replaced. Queue-full,
authentication/protocol errors and ambiguous acknowledgements after sending are
returned without retry to avoid duplicate launch side effects. Tests drive retries
with injected deterministic wait steps, not sleeps. Request parsing rejects trailing
JSON and invalid credentials before enqueueing.

The public `app.Activate(ctx)` queues activation and returns a completion channel.
It is safe to enqueue from `OnLaunch`; do not wait for completion inside the UI
callback. The macOS host unhides the application, chooses the key/main/front
eligible window, restores it if minimized, and requests foreground activation.
OS focus policy still applies; completion does not prove that focus was granted.
Windows foreground permission transfer and Wayland activation-token consumption
remain missing. The public desktop smoke example exercises the macOS request
through the real event loop and closes automatically.
