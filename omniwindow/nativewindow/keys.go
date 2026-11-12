package nativewindow

import "unicode/utf16"

// Windows delivers UTF-16 code units separately; never expose half a surrogate.
type utf16Input struct{ high uint16 }

func (d *utf16Input) feed(c uint16) string {
	if c >= 0xd800 && c <= 0xdbff {
		d.high = c
		return ""
	}
	if c >= 0xdc00 && c <= 0xdfff {
		h := d.high
		d.high = 0
		if h != 0 {
			return string(utf16.DecodeRune(rune(h), rune(c)))
		}
		return "�"
	}
	d.high = 0
	if c < 32 || c == 127 {
		return ""
	}
	return string(rune(c))
}
