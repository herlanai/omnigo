# Chromium port provenance

At the user's explicit request, this plugin is directly ported from the local
Golet Chromium plugin by HyperparameterSI (MIT; see LICENSE.golet). The source
repository remains read-only. This permission applies to Chromium and its
assistant/setup/native/new-tab assets, not to unrelated framework controls.

Imported: Go browser/profile/theme/style/process APIs, assistant and user-script
support, macOS/Windows CEF hosts, setup/packaging, runtime pins, new-tab assets,
and corresponding tests. Package paths and product/runtime/cache names are
adapted for OmniGo. No Golet framework dependency is introduced.

CEF remains an optional third-party native runtime. Its license is retained in
native/CEF-LICENSE.txt and installed runtime bundles include LICENSE.txt and
CREDITS.html. macOS uses the upstream sandbox; no no-sandbox fallback is added.
Native Windows execution and Linux host support are not implied by Go builds.

OmniGo-specific adaptations include explicit CEF main-bundle, framework and
helper paths for a browser nested inside Studio.app; runtime discovery and
single-browser lifecycle ownership in Studio; a standalone CLI; and license
packaging on macOS and Windows. E2E fixtures explicitly use disposable demo
profiles so tests do not access the user's secure storage.

Validated on macOS arm64: source runtime and Studio-bundled runtime passed real
CEF tests for navigation/history, tabs, JavaScript/CDP, Chrome extensions,
Alloy/Chrome disposable profiles, app mode, branding and assistant actions.
This is independent of Go unit/race/cross-build checks. Windows native and Intel
Mac execution remain unverified; the upstream Linux/embedded-view limitations
remain unchanged. A source hash manifest records the imported reference files.
