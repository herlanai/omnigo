// Package accelerator parses portable keyboard chords independently of native
// shortcut registration. Hosts forward key events to Matches on their UI thread.
package accelerator

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Modifiers uint8

const (
	Ctrl Modifiers = 1 << iota
	Alt
	Shift
	Super
)

type Accelerator struct {
	Modifiers Modifiers
	Key       string
}
type keyDescription struct {
	canonical, symbol string
	aliases           []string
}

var descriptions = []keyDescription{
	{"Enter", "Return", []string{"return"}}, {"Escape", "Escape", []string{"esc"}}, {"Space", "space", []string{"spacebar"}},
	{"Tab", "Tab", nil}, {"Backspace", "BackSpace", nil}, {"Delete", "Delete", []string{"del"}}, {"Insert", "Insert", []string{"ins"}},
	{"Up", "Up", []string{"arrowup"}}, {"Down", "Down", []string{"arrowdown"}}, {"Left", "Left", []string{"arrowleft"}}, {"Right", "Right", []string{"arrowright"}},
	{"Home", "Home", nil}, {"End", "End", nil}, {"PageUp", "Page_Up", []string{"pgup"}}, {"PageDown", "Page_Down", []string{"pgdown", "pgdn"}},
	{"PrintScreen", "Print", nil}, {"CapsLock", "Caps_Lock", nil}, {"NumLock", "Num_Lock", nil}, {"ScrollLock", "Scroll_Lock", nil},
	{"VolumeUp", "XF86AudioRaiseVolume", nil}, {"VolumeDown", "XF86AudioLowerVolume", nil}, {"VolumeMute", "XF86AudioMute", nil},
	{"MediaNextTrack", "XF86AudioNext", nil}, {"MediaPreviousTrack", "XF86AudioPrev", nil}, {"MediaStop", "XF86AudioStop", nil}, {"MediaPlayPause", "XF86AudioPlay", nil},
	{"NumDec", "KP_Decimal", nil}, {"NumAdd", "KP_Add", nil}, {"NumSub", "KP_Subtract", nil}, {"NumMult", "KP_Multiply", nil}, {"NumDiv", "KP_Divide", nil},
	{"+", "plus", []string{"plus"}},
}

func modifier(s, goos string) Modifiers {
	switch strings.ToLower(s) {
	case "ctrl", "control":
		return Ctrl
	case "alt", "option":
		return Alt
	case "shift":
		return Shift
	case "super", "cmd", "command", "meta", "win", "windows":
		return Super
	case "cmdorctrl", "commandorcontrol":
		if goos == "darwin" {
			return Super
		}
		return Ctrl
	}
	return 0
}
func canonicalKey(key string) (string, error) {
	if !utf8.ValidString(key) {
		return "", fmt.Errorf("accelerator: invalid UTF-8 key")
	}
	if utf8.RuneCountInString(key) == 1 {
		r, _ := utf8.DecodeRuneInString(key)
		if !unicode.IsControl(r) && !unicode.IsSpace(r) {
			return strings.ToLower(key), nil
		}
	}
	for _, d := range descriptions {
		if strings.EqualFold(key, d.canonical) {
			return d.canonical, nil
		}
		for _, alias := range d.aliases {
			if strings.EqualFold(key, alias) {
				return d.canonical, nil
			}
		}
	}
	lower := strings.ToLower(key)
	if strings.HasPrefix(lower, "f") {
		n, err := strconv.Atoi(lower[1:])
		if err == nil && n >= 1 && n <= 24 && strconv.Itoa(n) == lower[1:] {
			return fmt.Sprintf("F%d", n), nil
		}
	}
	if len(lower) == 4 && strings.HasPrefix(lower, "num") && lower[3] >= '0' && lower[3] <= '9' {
		return "Num" + lower[3:], nil
	}
	return "", fmt.Errorf("accelerator: unknown key %q", key)
}
func Parse(chord, goos string) (Accelerator, error) {
	var out Accelerator
	chord = strings.TrimSpace(chord)
	if len(chord) > 256 {
		return out, fmt.Errorf("accelerator: chord too long")
	}
	var key string
	var parts []string
	if chord == "+" {
		key = "+"
	} else if strings.HasSuffix(chord, "++") {
		key = "+"
		parts = strings.Split(chord[:len(chord)-2], "+")
	} else {
		all := strings.Split(chord, "+")
		key = strings.TrimSpace(all[len(all)-1])
		parts = all[:len(all)-1]
	}
	for _, part := range parts {
		m := modifier(strings.TrimSpace(part), goos)
		if m == 0 {
			return Accelerator{}, fmt.Errorf("accelerator: unknown modifier %q", part)
		}
		out.Modifiers |= m
	}
	var err error
	out.Key, err = canonicalKey(key)
	if err != nil {
		return Accelerator{}, err
	}
	return out, nil
}
func (a Accelerator) Has(m Modifiers) bool { return a.Modifiers&m == m }
func (a Accelerator) String() string {
	var b strings.Builder
	for _, m := range []struct {
		flag  Modifiers
		label string
	}{{Ctrl, "Ctrl"}, {Alt, "Alt"}, {Shift, "Shift"}, {Super, "Super"}} {
		if a.Has(m.flag) {
			b.WriteString(m.label)
			b.WriteByte('+')
		}
	}
	key := a.Key
	if len(key) == 1 && key[0] >= 'a' && key[0] <= 'z' {
		key = strings.ToUpper(key)
	}
	b.WriteString(key)
	return b.String()
}
func (a Accelerator) Matches(key string, mods Modifiers) bool {
	k, err := canonicalKey(key)
	return err == nil && mods == a.Modifiers && k == a.Key
}
func (a Accelerator) Keysym() string {
	for _, d := range descriptions {
		if a.Key == d.canonical {
			return d.symbol
		}
	}
	if len(a.Key) == 4 && strings.HasPrefix(a.Key, "Num") {
		return "KP_" + a.Key[3:]
	}
	punctuation := "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
	names := strings.Fields("exclam quotedbl numbersign dollar percent ampersand apostrophe parenleft parenright asterisk plus comma minus period slash colon semicolon less equal greater question at bracketleft backslash bracketright asciicircum underscore grave braceleft bar braceright asciitilde")
	for i, r := range punctuation {
		if a.Key == string(r) {
			return names[i]
		}
	}
	r, n := utf8.DecodeRuneInString(a.Key)
	if n == len(a.Key) && r > '~' {
		return fmt.Sprintf("U%04X", r)
	}
	return a.Key
}
func (a Accelerator) XDGTrigger() string {
	var pieces []string
	for _, m := range []struct {
		flag  Modifiers
		label string
	}{{Ctrl, "CTRL"}, {Alt, "ALT"}, {Shift, "SHIFT"}, {Super, "LOGO"}} {
		if a.Has(m.flag) {
			pieces = append(pieces, m.label)
		}
	}
	return strings.Join(append(pieces, a.Keysym()), "+")
}
