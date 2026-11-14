# Signed updates and executable installation

`Verify` authenticates a manifest with Ed25519. `Download` stages a bounded,
size/hash-checked artifact. `InstallVerified(ctx, target, source, manifest,
signature, publicKey)` reauthenticates the manifest and verifies a private copy
before replacing an existing regular-file executable.

The target parent must be application-owned and not writable by untrusted users.
Run installation from an external helper after the old application has stopped.
The installer does not wait for processes or bypass platform executable locks.
It does not unpack archives or install macOS app bundles.

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

Process-exit waiting, restart/health confirmation, update UI,
archive/app-bundle installation, OS signing checks and rollback after power loss
are still pending. This is an installation primitive, not a complete auto-updater.
