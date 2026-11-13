//go:build darwin && (amd64 || arm64)

package host

import (
	"fmt"
	"github.com/ebitengine/purego/objc"
	"github.com/herlanai/omnigo/omniwindow"
	"github.com/herlanai/omnigo/omniwindow/accelerator"
	"strconv"
	"strings"
	"unicode/utf8"
	"unsafe"
)

func menuKey(chord string) (string, uint64, error) {
	if chord == "" {
		return "", 0, nil
	}
	key, err := accelerator.Parse(chord, "darwin")
	if err != nil {
		return "", 0, err
	}
	var mask uint64
	for _, m := range []struct {
		flag accelerator.Modifiers
		bit  uint64
	}{{accelerator.Shift, 1 << 17}, {accelerator.Ctrl, 1 << 18}, {accelerator.Alt, 1 << 19}, {accelerator.Super, 1 << 20}} {
		if key.Has(m.flag) {
			mask |= m.bit
		}
	}
	text := key.Key
	special := map[string]string{"Enter": "\r", "Escape": "\x1b", "Space": " ", "Tab": "\t", "Backspace": "\x08", "Delete": "\uf728", "Up": "\uf700", "Down": "\uf701", "Left": "\uf702", "Right": "\uf703", "Home": "\uf729", "End": "\uf72b", "PageUp": "\uf72c", "PageDown": "\uf72d"}
	if v, ok := special[text]; ok {
		return v, mask, nil
	}
	if strings.HasPrefix(text, "F") {
		n, e := strconv.Atoi(text[1:])
		if e == nil && n >= 1 && n <= 24 {
			return string(rune(0xf704 + n - 1)), mask, nil
		}
	}
	if utf8.RuneCountInString(text) != 1 {
		return "", 0, fmt.Errorf("menu: key %q has no AppKit menu equivalent", text)
	}
	return text, mask, nil
}
func menuImage(data []byte, template bool, pixels float64) (objc.ID, error) {
	if len(data) == 0 {
		return 0, nil
	}
	native := objc.ID(objc.GetClass("NSData")).Send(sel("dataWithBytes:length:"), unsafe.Pointer(&data[0]), uint64(len(data)))
	icon := objc.ID(objc.GetClass("NSImage")).Send(sel("alloc")).Send(sel("initWithData:"), native)
	if icon == 0 {
		return 0, fmt.Errorf("menu: native PNG decode failed")
	}
	icon.Send(sel("setTemplate:"), template)
	icon.Send(sel("setSize:"), size{W: pixels, H: pixels})
	return icon, nil
}
