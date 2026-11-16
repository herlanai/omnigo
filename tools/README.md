# Repository tools

`cmd/` contains Go commands for repository maintainers. The root `cmd/` directory
contains distributable application commands; `scripts/` contains shell and
platform-specific CI adapters. Run repository commands from the checkout root.

| Command | Responsibility |
| --- | --- |
| `dev` | Studio development, packaging, catalog generation, and Chromium setup |
| `controlaudit` | Control declaration inventory and coverage checks |
| `featureaudit` | Feature and framework-internal reference inventories |
| `platformaudit` | Platform interface inventory |
| `serviceaudit` | Service contract inventory and evidence checks |
| `parity` | Read-only reference declaration snapshot |
| `nativeprobe` | iOS, Android, and desktop CI verification |
| `idle` | CPU/RSS sampling for an existing process |

Repository maintenance and Studio orchestration use the Go standard library:

```sh
go run ./tools/cmd/dev catalog
go run ./tools/cmd/dev audit-ui --source /absolute/path/to/golet
go run ./tools/cmd/dev dev
go run ./tools/cmd/dev build
go run ./tools/cmd/dev start
go run ./tools/cmd/dev package
go run ./tools/cmd/dev chromium
```

Run inside this checkout. `audit-ui` reads the reference repository without modifying it; constructor mappings do not establish control parity. `chromium` forwards setup flags, uses `CEF_ROOT` when `--cef-dir` is absent, and defaults to `build/chromium-runtime`. The existing pnpm shortcuts call these Go commands.

Studio still has a Web frontend: building it requires Node and pnpm, and the OmniWeb client tests use Node. Native CEF packaging still requires its platform C++/CMake toolchain. CI retains `scripts/ci/browser_project.mjs` and `scripts/ci/visible_window.swift` for browser-project and native-window validation.

Studio packages retain dependency licenses and native bundle symlinks. Packaging uses the platform's `ditto` or `tar`; macOS builds use ad-hoc code signing, without distribution signing or notarization.
