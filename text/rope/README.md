# Immutable text storage

`Text` is a Go persistent UTF-8 chunk tree with byte/rune/UTF-16/newline summaries. `New`, `Replace`, `Slice`, `Chunks`, line queries, forward/reverse line iteration, and UTF-16/rune conversions operate on immutable versions. `Same` tests version identity; versions may be read from concurrent goroutines. Chunks are at most 2 KiB; unchanged tree branches are shared. Invalid UTF-8 is normalized to U+FFFD; replacements round byte offsets down to rune boundaries, and UTF-16 offsets inside a surrogate pair round down.

`baseui.TextBuffer` now keeps these versions in undo/redo history and exposes `Snapshot()`. IME composition remains separate until commit. Its current rendering string is still materialized after an edit; this is not a claim that the entire editor is incremental or that shaping/grapheme navigation is complete.

Studio **Text snapshots** exercises editing, save/restore, undo/redo, a 1,000-line document and byte/rune/UTF-16/line coordinates. Tests compare 1,200 deterministic Unicode edits with strings, retain and concurrently read versions, validate tree summaries, and cover bounds, line endings and chunk boundaries. The reference uses a different tree structure; no implementation bodies were copied.
