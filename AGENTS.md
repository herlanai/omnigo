# OmniGo repository rules

- Framework controls are implemented in Go under `omniui/baseui`. This includes behavior, layout, theme, painting, compositions and optional control plugins. Do not implement the control rewrite in React/TSX or route it through a WebView.
- OmniWeb remains the separate Web UI protocol. Its TanStack Start + shadcn template is an application frontend option, not evidence of Go control parity.
- Read `omniui/baseui/README.md` and `migration/controls/README.md` before control work. Preserve every reference capability in the audit. An API entry, static drawing, alias or passing build does not establish parity.
- The local reference repository is read-only. Rewrite behavior; do not copy implementation bodies, rename the old framework or add it as a dependency. Preserve licenses for any third-party libraries/assets actually used.
- All Go targets must build with `CGO_ENABLED=0`. Keep optional rendering/plugin dependencies out of applications that do not import them.
- Surface and control state are confined to their UI goroutine. Use `internal/runtime/dispatch` for work from other goroutines. Do not retain transient paint contexts.
- Test interaction with deterministic events and explicit clocks, not sleeps. Verify disabled/read-only, focus, cancellation, IME composition, bounds, resource lifetime and dark/light rendering as relevant.
- Run `sh scripts/check.sh` for substantive core changes. It checks tests, vet, race, SDK tests and six desktop cross-compilation targets. Cross-compilation does not establish native-window compatibility.
- Report incomplete behavior and performance evidence accurately. Never describe partial controls as full reference coverage or claim a speedup without equivalent benchmarks.
