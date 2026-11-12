# Keyboard accelerators

`Parse(chord, goos)` resolves portable modifier aliases, named keys, F1–F24, keypad/media keys, Unicode characters and literal plus. `String`, `Keysym`, `XDGTrigger`, and `Matches` provide canonical display, XKB names, portal trigger syntax and exact physical-modifier matching. Parsing does not register a global shortcut.

Studio **Keyboard shortcuts** lets users change the chord and target platform, inspect its representation, focus a test button and press the chord. Repeat/key-up events do not activate it. Native event structures retain Ctrl/Alt/Super separately from the existing primary `Command` flag. macOS forwards additional Command/Control characters; full keyboard layout, Option/dead-key, global registration and six-platform native validation remain outstanding. X11 currently assumes the conventional Mod1/Mod4 Alt/Super mapping.
