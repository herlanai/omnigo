# Native Markdown

Import `github.com/herlanai/omnigo/omniui/plugins/markdown`.

`State.View(id, source, Options)` parses CommonMark/GFM with optional Goldmark and composes Go controls. Importing baseui alone does not link the parser. Source is limited to 1 MiB; AST depth and node count are bounded. State belongs to the UI goroutine.

Supported: headings, bold/italic/strike/inline code, wrapped paragraphs, lists/tasks, quotes, fenced/indented code, tables, reference/autolinks and literal HTML. Link activation invokes the application's callback after scheme validation. Code copy callbacks receive original source. Images show alt text without fetching resources. Source changes currently reparse the whole document.

Not complete: text selection and multi-format clipboard, images, math, alerts, mentions, anchors, streaming repair/incremental parsing, table alignment, complex shaping/font fallback and per-link accessibility. This is a reader, not the reference rich text editor. Raw HTML is displayed, never executed.

Goldmark's MIT license is preserved in LICENSE.goldmark. Tests cover source changes, limits, links, copy and styled light/dark layout. Studio provides an editable source preview under Markdown.
