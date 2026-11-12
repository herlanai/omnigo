# Third-party notices

Studio is an original implementation. The reference project is read only and is not linked or vendored.

The native experimental host links github.com/ebitengine/purego (Apache-2.0). Go text rasterization uses golang.org/x/image, golang.org/x/text and the Go fonts distributed with x/image (BSD-style and font-specific licenses). The Go standard library is BSD-3-Clause.

The directory application uses React, TanStack Start/Router, Radix Slot, class-variance-authority, clsx, tailwind-merge, Tailwind CSS and Lucide. The package lock records exact versions. Their license files are included in the packaged Resources/licenses directory where distributed by the dependencies. Build tooling includes Vite, TypeScript and their transitive packages; the bundle preserves generated license comments.

See individual license files for copyright holders and full terms. The default shadcn-compatible Button/Input source originated in this repository's OmniWeb template, not in the reference project's controls.

Optional plugins additionally use coder/websocket (ISC), creack/pty (MIT), and modernc.org/sqlite with its transitive pure-Go runtime (see each component's license). Complete collected license texts are preserved under `omniui/plugins/licenses`; the SQLite distribution includes public-domain SQLite and additional third-party notices. The reference Golet repository is not imported or copied into these plugins.

The native Go icon demo embeds official lucide-static 1.71.0 SVGs (ISC, with Feather-derived MIT attribution). The complete asset license is preserved at `omniui/icon/lucide/LICENSE` and packaged as `lucide-static-1.71.0-LICENSE`. Native SVG rasterization uses srwiley/oksvg and srwiley/rasterx (BSD-3-Clause); module licenses are included in the packaged license directory.

## Golet Chromium port

The optional Chromium plugin, native host, assistant, setup tooling and new-tab
assets are directly ported from Golet by HyperparameterSI at the user's request.
Copyright (c) 2026 MyGo contributors. MIT license retained at
`omniui/plugins/chromium/LICENSE.golet` and copied into distributed bundles.
CEF binaries retain their own LICENSE.txt and CREDITS.html; see the optional
runtime bundle's Resources directory.
