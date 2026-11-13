# Non-control parity work

Scope: align the local reference's desktop system services, application lifecycle,
OmniWeb, tooling and non-control plugin behavior. Controls, drawing and panel
work are deferred. The local reference is read-only; this audit contains file
fingerprints and exported declarations, not implementation bodies.

`reference.json` partitions the current local scan: **2,618 source/document files**,
**1,832 retained for non-control or mixed review**, **786 explicitly deferred**,
and **1,574 exported declarations in retained files**. These counts are inventory,
not completed behaviors. Mixed platform contracts, templates, root API files and
non-control plugin directories remain included until individually reviewed; they
may contain control-related declarations that must be deferred at behavior level.
No file disappears merely because it is internal, a test, TypeScript or a CLI file.

`status.json` is the work queue. Its broad entries retain unresolved method-level
mapping explicitly; they do not replace the detailed platform, Web or plugin
records. `pending` means this inventory has no accepted implementation evidence;
`partial` records an implementation without claiming reference parity. `verified`
requires evidence and no remaining differences. No entry currently claims full
reference parity. Performance parity requires equivalent benchmarks separately.

Priority order:

1. System directories and shell, then dialogs, display/appearance, clipboard,
   menus/tray, notifications, power/network and secrets.
2. Readiness/shutdown, activation/reopen, open-file/URL, single-instance, relaunch.
3. RPC/events/streams, typed client generation, WebView services and bridge APIs.
4. Project/dev/build/package/release tools, then each non-control plugin contract.

## 2026-11-13 implementation increment

- `system.Paths` and root `Application.Path`/`SetPath`: 14 named locations,
  stable application name, thread-safe overrides, lazy private data/cache/log
  directories, bundle/dev resource resolution, XDG user-directory parsing without
  shell execution and Windows KnownFolder APIs. Unit tests cover these contracts;
  Windows redirected-folder and Linux native acceptance remain outstanding.
- `Options.OnReady` and `Application.WhenReady(ctx)`: once-only UI callback,
  cancelable worker wait, startup-failure/shutdown release, and explicit stopped
  error. These do not implement cancelable quit/reopen lifecycle events.
- `omniweb.GenerateTypeScript` / `Application.GenerateTypeScript` and
  `WriteTypeScript`: deterministic unary RPC contracts from registered types,
  recursive structs, nullability, JSON names/omission/string tags, bytes/time/raw
  JSON mapping. Ambiguous/custom codecs fail without replacing previous output.
- `omni generate -package ./cmd/app -out ui/services.ts`: runs trusted application
  code in generation mode; `omnigo.Run`/`RunMain` generate before native windows
  or services start. Constructor/init code still runs. Staged output prevents
  stale files from masquerading as successful generation. Identical files retain
  their timestamp. Events/streams, embedded fields and full convenience-client
  generation are still pending.

See `omniwindow/system/README.md` and `migration/webui.md` for boundaries and usage.
Existing reveal/trash, single-instance, events/streams and watch work was preserved.
No control files or control behavior are changed by this increment.

## Refresh and validate

```sh
go run ./tools/parity -source /absolute/path/to/golet -out /tmp/reference-current.json
go run ./tools/serviceaudit -inventory /tmp/reference-current.json
go test ./tools/serviceaudit
sh scripts/check.sh
```

Review fingerprint changes before accepting a refresh. The partition tool never
rewrites `status.json`. Coverage tests reject untracked groups, orphaned API
records, duplicate source files and nonexistent evidence paths. They do not
establish behavioral equivalence. Native tests and external module acceptance
must be recorded separately from six-target cross-compilation.

## Verification recorded for this increment

- Full `sh scripts/check.sh` passed on this macOS arm64 workspace: Go tests,
  vet, race, 37 SDK tests, and six CGO-disabled desktop build targets.
- A separate temporary Go module imported only public APIs, compiled and ran
  the real `omni generate` command; its service deliberately panicked if started
  and did not start. The emitted contract compiled with the canonical SDK using
  the repository's TypeScript 7.0.2 compiler under strict mode.
- Generator tests also compile recursive DTOs and nullability against the SDK.
  They explicitly skip compiler acceptance when `tsc` is unavailable; this run
  added the installed compiler to PATH and passed that check.
- No new native-window/Windows/Linux device acceptance or equivalent performance
  benchmark was performed. Broad non-control parity is still incomplete.
