# Repository tools

Repository maintenance and Studio orchestration use the Go standard library:

```sh
go run ./scripts/devtool catalog
go run ./scripts/devtool audit-ui --source /absolute/path/to/golet
go run ./scripts/devtool dev
go run ./scripts/devtool build
go run ./scripts/devtool start
go run ./scripts/devtool package
go run ./scripts/devtool chromium
```

Run inside this checkout. `audit-ui` reads the reference repository without modifying it; constructor mappings do not establish control parity. `chromium` forwards setup flags, uses `CEF_ROOT` when `--cef-dir` is absent, and defaults to `build/chromium-runtime`. The existing pnpm shortcuts call these Go commands.

Studio still has a Web frontend: building it requires Node and pnpm, and the OmniWeb client tests use Node. These tools do not replace the frontend with Go. Native CEF packaging still requires its platform C++/CMake toolchain. Python and standalone MJS maintenance scripts are no longer required.

Studio packages retain dependency licenses and native bundle symlinks. Packaging uses the platform's `ditto` or `tar`; macOS builds use ad-hoc code signing, without distribution signing or notarization.
