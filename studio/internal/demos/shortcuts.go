package demos

import (
	"fmt"
	"github.com/herlanai/omnigo/apps/accelerator"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"runtime"
)

type shortcutDemo struct {
	input       *ui.TextBuffer
	goos        string
	activations int
}

func (d *Demo) shortcutContent() *ui.Element {
	if d.shortcuts == nil {
		d.shortcuts = &shortcutDemo{input: ui.NewTextBuffer("CmdOrCtrl+Shift+K"), goos: runtime.GOOS}
	}
	state := d.shortcuts
	a, err := accelerator.Parse(state.input.String(), state.goos)
	result := ""
	if err != nil {
		result = err.Error()
	} else {
		result = fmt.Sprintf("Canonical: %s\nXKB: %s\nXDG trigger: %s", a.String(), a.Keysym(), a.XDGTrigger())
	}
	choose := func(os string) func() { return func() { state.goos = os } }
	probe := ui.Button("demo/shortcut-probe", "Focus here, then press the shortcut", nil).OnKey(func(e ui.KeyEvent) bool {
		if err != nil || e.Up || e.Repeat {
			return false
		}
		var mods accelerator.Modifiers
		if e.Ctrl {
			mods |= accelerator.Ctrl
		}
		if e.Alt {
			mods |= accelerator.Alt
		}
		if e.Super {
			mods |= accelerator.Super
		}
		if e.Shift {
			mods |= accelerator.Shift
		}
		if a.Matches(e.Key, mods) {
			state.activations++
			return true
		}
		return false
	})
	return ui.Column("demo/shortcuts", ui.Input("demo/chord", "Shortcut", state.input), ui.Row("demo/platforms", ui.Button("demo/mac", "macOS", choose("darwin")), ui.Button("demo/win", "Windows", choose("windows")), ui.Button("demo/linux", "Linux", choose("linux"))).Gap(8), ui.Text("demo/target", "Parse for: "+state.goos), ui.Text("demo/parsed", result), probe, ui.Text("demo/matches", fmt.Sprintf("Matched %d times", state.activations)), ui.Text("demo/scope", "Focus is required; shortcuts do not run in other apps.").Muted(true)).Gap(16)
}
