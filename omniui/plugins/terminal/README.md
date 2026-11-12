# Terminal

Unix PTY transport and a native Go viewport are separate APIs. Read PTY output on a worker; dispatch `Screen.Feed` and UI updates through runtime/dispatch. Close the PTY explicitly. `View` captures a screen snapshot; remount after changes. It never launches a shell implicitly.

The bounded screen handles split UTF-8/escape chunks, cursor movement, line/screen erasing, SGR colors, bold/inverse, alternate screen and resizing. Basic committed text and navigation keys are delivered through OnInput. History stores bounded rows but is not yet exposed as an interactive scrollback viewport. Resizing preserves overlapping cells without reflow.

Remaining: wide/combining text, complete VT modes/margins/replies, selection/search, IME composition, mouse reporting, advanced keys, interactive native acceptance and Windows ConPTY. Unsupported escape strings are ignored. This is a limited VT implementation, not xterm compatibility.
