# Go icons and Lucide

The renderer and the catalog are optional Go packages. Applications importing
`baseui` alone do not link the SVG parser or Lucide assets. No browser, WebView,
CGO, runtime download or external resource loading is involved.

```go
import (
    ui "omnigo.dev/omnigo/omniui/baseui"
    "omnigo.dev/omnigo/omniui/icon/lucide"
)

search := ui.Icon("search-icon", "", lucide.Search(), ui.IconOptions{Size: 20})
settings := ui.Icon("settings-icon", "Settings", lucide.Must("settings"),
    ui.IconOptions{Size: 32, StrokeWidth: 1.5})
copyButton := ui.IconButton("copy", "Copy", lucide.Copy(), func() {
    // Application action, on the UI goroutine.
}).Variant(ui.Outline)
```

`lucide.Get(name)` returns an error for an unknown kebab-case name. `lucide.Must`
is convenient for known embedded names. `lucide.Names()` returns the sorted
catalog; `lucide.SVG(name)` returns owned source bytes. The pinned official
`lucide-static@1.71.0` archive supplies **2,350 SVG files including aliases**.
Only requested icons are parsed. Importing the catalog embeds the entire asset
set; use `icon.ParseSVG` with individually embedded SVGs to minimize binary size.

```go
import "omnigo.dev/omnigo/omniui/icon"

//go:embed search.svg
var data []byte
var search = icon.MustParseSVG(data)
```

Custom SVGs support paths (including compact arc flags), circles, ellipses,
rectangles, lines, polylines, polygons and simple groups. Non-`none` fill/stroke
becomes the requested icon color. This is a monochrome icon renderer, not a
complete SVG image engine. Transforms, CSS, gradients, masks, filters, external
references and text are rejected. The local reference provides ParseSVG/Icon/
Painter.Icon and a broader SVG engine; these missing capabilities remain audited.

`IconOptions.Color` defaults to the current theme text color. `Size` is height
in DIP and preserves aspect ratio. `StrokeWidth` is in SVG viewBox units (zero
preserves the SVG value); `AbsoluteStrokeWidth` interprets it in logical pixels.
Baseui rasterizes at actual output DPI. A blank label is decorative; meaningful
icons need labels, and icon-only buttons require a nonempty accessible label.
Button variants, focus, cancellation, disabled and read-only behavior reuse the
existing Go button. System accessibility adapter acceptance remains separate.

A parsed SVG is immutable and safe for concurrent rendering. `Rasterize` returns
owned images and retains at most eight small cached rasters per SVG. Inputs are
limited to 256 KiB, 4,096 elements and 32 levels; raster output is capped at
2,048 pixels per edge and 1,048,576 pixels total. No transient Painter is retained.
Parsing and resource work should be done before constructing frequently rebuilt
views. Lucide's lookup cache preserves parsed icons for process lifetime.

Tests parse and render every bundled asset and cover tint, alpha, compact arcs,
stroke scaling, image ownership, concurrent rendering, malformed inputs, DPI,
button state and builder snapshots. Studio's **Lucide / SVG icons** demo supports
name search, pagination, size/stroke changes, copy-name and icon buttons.

## Sources and licenses

Official assets: https://lucide.dev/ and https://github.com/lucide-icons/lucide.
`lucide/source.json` records the pinned archive URL, SHA-512 integrity and file
count. The archive was verified before extraction. Upstream assets are unchanged;
compact SVG syntax is normalized only in memory during parsing.

`lucide/LICENSE` includes ISC terms and Feather-derived MIT attribution. The
pure-Go SVG/raster dependencies are `github.com/srwiley/oksvg` and
`github.com/srwiley/rasterx`; their BSD licenses are preserved in `licenses/`.
Studio packaging includes these dependency licenses and the Lucide asset license.
