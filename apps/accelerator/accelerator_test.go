package accelerator

import "testing"

func TestPortableChordsAndMatching(t *testing.T) {
	for _, os := range []string{"darwin", "windows", "linux"} {
		a, err := Parse(" CommandOrControl + Shift + k ", os)
		if err != nil {
			t.Fatal(err)
		}
		mods := Ctrl | Shift
		if os == "darwin" {
			mods = Super | Shift
		}
		if !a.Matches("K", mods) || a.Matches("k", mods|Alt) {
			t.Fatal("modifiers", os, a)
		}
		b, err := Parse(a.String(), os)
		if err != nil || a != b {
			t.Fatal("roundtrip", a, b, err)
		}
	}
	for _, input := range []string{"", "Ctrl+", "Ctrl+A+B", "Ctrl+F25", "F01", "Ctrl++a", "Hyper+q", "Ctrl+\x00"} {
		if _, err := Parse(input, "linux"); err == nil {
			t.Fatal("accepted", input)
		}
	}
}
func TestNativeKeyRepresentations(t *testing.T) {
	cases := map[string]string{"CmdOrCtrl++": "CTRL+plus", "Alt+Return": "ALT+Return", "Super+Space": "LOGO+space", "Ctrl+é": "CTRL+U00E9", "Ctrl+Num7": "CTRL+KP_7", "MediaPlayPause": "XF86AudioPlay", "Ctrl+PageDown": "CTRL+Page_Down", "Ctrl+\\": "CTRL+backslash", "Shift+F24": "SHIFT+F24"}
	for chord, want := range cases {
		a, err := Parse(chord, "linux")
		if err != nil || a.XDGTrigger() != want {
			t.Fatal(chord, a.XDGTrigger(), want, err)
		}
	}
	for _, d := range descriptions {
		a, err := Parse(d.canonical, "linux")
		if err != nil || a.Keysym() != d.symbol {
			t.Fatal(d, err)
		}
	}
}
func FuzzChord(f *testing.F) {
	f.Add("Ctrl+Shift+A")
	f.Add("CmdOrCtrl++")
	f.Fuzz(func(t *testing.T, s string) {
		a, err := Parse(s, "linux")
		if err != nil {
			return
		}
		b, err := Parse(a.String(), "linux")
		if err != nil || a != b {
			t.Fatal("roundtrip", a, b, err)
		}
		_ = a.XDGTrigger()
	})
}
