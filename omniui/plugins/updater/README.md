# Signed updates and executable installation

`Verify` authenticates a manifest with Ed25519. `Download` stages a bounded,
size/hash-checked artifact. `InstallVerified(ctx, target, source, manifest,
signature, publicKey)` reauthenticates the manifest and verifies a private copy
before replacing an existing regular-file executable.

The target parent must be application-owned and not writable by untrusted users.
Run installation from an external helper after the old application has stopped.
The installer does not wait for processes or bypass platform executable locks.
For macOS app bundles use the separate bundle transaction described below.

Installation creates an exclusive `target.omnigo-update` directory on the same
filesystem. `previous` stores the old executable, `manifest.json` records the
signed metadata. Replacement uses the host filesystem's rename semantics; no
cross-platform atomicity or power-loss durability guarantee is made. Preparation
errors preserve the old target and clean this transaction's staging; an existing
transaction directory is never deleted automatically.

The returned `Installation` must remain owned by the host:

- After independently confirming new-app startup, call `Confirm` to remove backup.
- After stopping a failed new app, call `Rollback` to restore the previous file.
- `BackupPath` identifies retained recovery data for diagnostics.
- Rollback also verifies the backup digest before restoring it. A successful
  restore followed by a cleanup failure can be finalized again on the same handle.
- Finalization checks the installed digest; external modification leaves recovery
  data intact instead of overwriting another update.

Context cancellation is checked during copies and immediately before replacement.
Once replacement succeeds, the returned transaction owns cleanup even if the
context is subsequently cancelled. The source artifact remains caller-owned.

If the helper crashes after writing a complete transaction journal, call
`RecoverInstallation(ctx, target, publicKey)`. It acquires an OS process lock,
rejects live owners, verifies the saved signed manifest, and checks current and
backup digests before returning a handle for Confirm or Rollback. A crash before
replacement or after restoration is recognized from the previous digest.
Incomplete/corrupt journals retain all files and require manual inspection. The
persistent .omnigo-update.lock inode is intentionally not deleted. The local
journal and backup still require an application-owned directory; they are not
authenticated by the publisher signature. No power-loss durability is claimed.

Use `defer installation.Close()` to release ownership on error. Close leaves
recovery data in place, invalidates the old handle, and permits another helper
to recover the transaction. Confirm/Rollback also release the lock on success.

## macOS bundle installation and supervised restart

`InstallBundleVerified(ctx, target, archive, manifest, signature, publicKey)`
accepts a ZIP containing exactly the target basename (`My App.app/...`). It checks
the publisher signature, archive size/hash, entry paths/types/expanded size,
`codesign --verify --deep --strict`, matching CFBundleIdentifier and an executable
CFBundleExecutable. ZIP symlinks and special files are rejected; bundles such as
CEF frameworks containing symlinks need a separate distribution strategy.
Ad-hoc signatures work for local acceptance; they are not Developer ID or notarization.

The previous bundle remains beside the target until confirmation. Replacement is
two same-volume renames with error recovery, not an atomic directory swap.
`RecoverBundleInstallation` reopens abandoned transactions, including interruption
between the two renames. A signed manifest authenticates the release; local
journals/backups require a trusted application-owned directory. Power-loss
persistence, privileged installation into unwritable directories and notarization
are outside this implementation.

From an external helper, after the old application has stopped:

```go
tx, err := updater.InstallBundleVerified(ctx, target, zipPath, manifest, signature, key)
if err != nil { return err }
defer tx.Close()
_, err = tx.RestartAndConfirm(ctx, "my-app", args...)
return err
```

`RestartAndConfirm` launches the installed executable. A random single-use loopback
credential is passed only to this child. OmniGo's root Application reports readiness
automatically after the native OnReady callback returns; custom hosts call
`updater.ConfirmStartup(ctx)` after their own readiness checks. This confirms native
host startup, not application-specific background data loading or future stability.
The helper waits at most 30 seconds, deletes the backup after acknowledgment, and
stops a timed-out child/process group before restoring the old bundle. Early exit
or spawn failure also restores it. The old version is restored but not relaunched
automatically. On cleanup failure, preserve the returned transaction for inspection.

Build `go build -o /trusted/helper/location/omni-update ./cmd/omni-update` outside
the target bundle. The helper accepts `--target`, `--archive`, `--manifest`,
`--signature` (64 raw bytes or standard Base64), `--public-key` (32 trusted raw bytes or the Base64 `.pub` file from `omni keygen`), and
`--executable`. `--wait-stdin` waits for an inherited pipe's EOF before installation:
the old application must retain its writer until native shutdown/process exit,
and must not pass it to unrelated children. Plain shell invocation requires the
old app to be stopped first. `--recover` resumes an abandoned installation;
`--recover --rollback` restores it without launching. Public key provenance belongs
to the application's trusted configuration, never to the downloaded release.

Release discovery, update UX and production signing remain application-owned.


`ParsePublicKey` and `ParseSignature` provide the same file-format handling to
custom helpers. They copy raw input without trimming binary bytes, accept surrounding
whitespace on Base64 text, and reject invalid lengths/encoding. Manifest bytes remain
unchanged: signing and verification must use exactly the same JSON bytes.
