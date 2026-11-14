# Application executable builds

From any working directory, run `omni build -config /path/to/project/omnigo.json`:

```json
{
  "package": "./cmd/myapp",
  "output": "build/myapp",
  "goos": "darwin",
  "goarch": "arm64",
  "before": [["npm", "--prefix", "ui", "run", "build"]]
}
```

Package and output paths resolve from the configuration directory. Target OS and
architecture are explicit; supported targets are darwin/windows/linux and
arm64/amd64. Commands in `before` are argument arrays, run sequentially without
shell expansion. They are trusted local project code and may modify files.
Omit them for Go-only applications. Build frontend assets into the directory
embedded by the application before compilation.

Go compilation always uses CGO_ENABLED=0 and trimpath. The executable is staged
next to its destination, checked for a nonempty regular file, then published by
rename. Compiler failure or cancellation preserves the previous executable and
removes staging. Rename failure is reported without deleting the old output;
replacement atomicity follows the host filesystem/OS guarantees. Preparation
commands are not transactional. Concurrent builds are not serialized.

This command produces an executable, not a complete app bundle or installer.
Generic dev/package/sign/notarize commands remain pending. Cross-building does
not validate native platform support or supply WebView runtimes.

## macOS app and DMG

Add `bundle` to the same configuration, then run `omni package` on macOS:

```json
"bundle": {
  "name": "My App",
  "id": "com.example.myapp",
  "version": "1.0.0",
  "icon": "assets/AppIcon.icns",
  "licenses": ["LICENSE", "THIRD_PARTY_NOTICES.md"],
  "destination": "releases/1.0.0"
}
```

The destination must not exist. Packaging builds the executable, checks its
Mach-O architecture, creates the app bundle, signs it ad-hoc, creates a DMG
with an Applications shortcut, verifies it, and publishes the release directory.
Previous release directories are never intentionally overwritten. Use a unique
release destination; concurrent publishers to the same path are unsupported.

Supply a valid product ICNS icon and licenses for all components actually used.
The runtime PNG icon must still be embedded and set by application code. CEF
runtime embedding, Developer ID signing and notarization are not implemented.
The executable output and preparation commands from build are not rolled back
if a later packaging step fails. A packaged app still requires native launch,
window interaction and installation testing.

## Development rebuild/restart (macOS and Linux)

Add an explicit watch list and run `omni dev -config omnigo.json`:

```json
"dev": {
  "watch": ["cmd", "internal", "ui/src", "go.mod", "go.sum"],
  "args": []
}
```

The config itself is always watched. Source contents are streamed through a
32 KiB buffer and hashed every 300 ms; scanning cooperates with cancellation;
additions/deletions and same-size edits trigger rebuild. Hidden directories and
node_modules are skipped. Do not include generated output paths, build/release
directories or the whole repository in the watch list. Rebuild runs the configured
before commands, so type generation can be added as an argument array, e.g.
`["omni", "generate", "-package", "./cmd/app", "-out", "ui/services.ts"]`.
Exclude that generated file from watch roots. Generation runs trusted project code.

A failed build leaves the old running application alive. A successful build stops
its process group (TERM, then KILL after at most two seconds), waits for exit and
starts the new binary. SIGINT performs the same cleanup. Natural child exit ends
the dev session and nonzero exits are reported. Watch paths and application args
are read at session start; restart dev after changing those settings. Build fields
are re-read on every build. This is rebuild/restart, not Vite HMR or browser-state
preservation. Detached child sessions and preparation-command descendants are not
managed by this process-group implementation. Windows dev is explicitly rejected
until Job Object ownership and cleanup are implemented. Dev requires a host-native
target; build may still cross-compile.
