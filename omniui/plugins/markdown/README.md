# Native Markdown

Import `github.com/herlanai/omnigo/omniui/plugins/markdown`.

`State.View(id, source, Options)` parses CommonMark/GFM with optional Goldmark and composes Go controls. Importing baseui alone does not link the parser. Source is limited to 1 MiB; AST depth and node count are bounded. State belongs to the UI goroutine.

Supported: headings, bold/italic/strike/inline code, wrapped paragraphs, nested lists, read-only native task checkboxes, quotes, GitHub Note/Tip/Important/Warning/Caution alerts, fenced/indented code, aligned tables, reference/autolinks, footnotes and literal HTML. Table columns keep equal widths across rows, with Markdown left/center/right alignment and header styling. Link activation invokes the application's callback after scheme validation. Raw HTML is displayed, never executed.

Body defaults to 14 DIP, 1.5 line height and 0.75 em block spacing. Heading sizes follow the reference's compact reading hierarchy. `Options.FontSize`, `LineHeight` and `BlockSpacing` customize layout. `WrapCode` switches between horizontal scrolling and wrapping without changing the copy payload. Go code highlighting reuses baseui's editor lexer; other languages remain plain text. Code indentation uses four-column tab stops. The normal text and code layouts share font fallback, DPI scaling, measurements and hit testing.

```go
var state markdown.State // retain per document on the UI goroutine
view, err := state.View("document", source, markdown.Options{
    WrapCode: true,
    OnLink: openLink,
    OnCopyCode: copyText,
})
```

`Options.Image(id, destination, title, alt)` optionally supplies an application-owned element for a standalone image paragraph. Return nil to keep the alt-text placeholder. Unsafe destinations are rejected before this callback. No remote fetch is performed; load/cache images outside the UI goroutine and remount when ready. Mixed inline images still use placeholders.

Unchanged source reuses the AST; changed source reparses the entire document. This is not golet's incremental parser, syntax healing or animated streaming. Also incomplete: cross-block text selection and multi-format clipboard, automatic image loading and inline images, math rendering, mentions, anchors, link context menus, full typography theming, complex shaping and per-link platform accessibility. The capability audit remains partial.

Goldmark's MIT license is preserved in LICENSE.goldmark. Tests cover source changes, limits, links, original-byte code copy, table geometry, alerts, image callbacks, task semantics and light/dark layout. Studio provides source editing, append, code wrap, copy Markdown and reset controls.
