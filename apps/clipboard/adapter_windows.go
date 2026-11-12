//go:build windows && (amd64 || arm64)

package clipboard

import (
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var user = windows.NewLazySystemDLL("user32.dll")
var kernel = windows.NewLazySystemDLL("kernel32.dll")

//go:uintptrescapes
func ucall(name string, args ...uintptr) uintptr {
	v, _, _ := user.NewProc(name).Call(args...)
	return v
}

//go:uintptrescapes
func kcall(name string, args ...uintptr) uintptr {
	v, _, _ := kernel.NewProc(name).Call(args...)
	return v
}

type winAdapter struct {
	owner uintptr
	owned bool
}

func openAdapter(o Options) (adapter, error) {
	if o.Name != "" {
		return nil, ErrUnsupported
	}
	w := &winAdapter{owner: o.Owner}
	if w.owner == 0 {
		class := windows.StringToUTF16Ptr("STATIC")
		w.owner = ucall("CreateWindowExW", 0, uintptr(unsafe.Pointer(class)), 0, 0, 0, 0, 0, 0, ^uintptr(2), 0, 0, 0)
		w.owned = true
	}
	if w.owner == 0 || ucall("IsWindow", w.owner) == 0 {
		return nil, errors.New("clipboard: owner window unavailable")
	}
	return w, nil
}
func (w *winAdapter) close() {
	if w.owned {
		ucall("DestroyWindow", w.owner)
	}
}
func winFormat(f string) uintptr {
	if f == Text {
		return 13
	}
	name := "OmniGo:" + f
	switch f {
	case PNG:
		name = "PNG"
	case HTML:
		name = "HTML Format"
	}
	p := windows.StringToUTF16Ptr(name)
	return ucall("RegisterClipboardFormatW", uintptr(unsafe.Pointer(p)))
}
func winMIME(id uintptr) string {
	if id == 13 {
		return Text
	}
	if id < 0xc000 {
		return ""
	}
	var buf [256]uint16
	n := ucall("GetClipboardFormatNameW", id, uintptr(unsafe.Pointer(&buf[0])), 256)
	if n == 0 {
		return ""
	}
	name := windows.UTF16ToString(buf[:min(n, 256)])
	switch name {
	case "PNG":
		return PNG
	case "HTML Format":
		return HTML
	}
	if !strings.HasPrefix(name, "OmniGo:") {
		return ""
	}
	f, err := format(strings.TrimPrefix(name, "OmniGo:"))
	if err != nil {
		return ""
	}
	return f
}
func (w *winAdapter) write(items []map[string][]byte) error {
	if len(items) > 1 {
		return fmt.Errorf("%w: Windows adapter accepts one item with multiple representations", ErrUnsupported)
	}
	type prepared struct{ id, handle uintptr }
	var allocated []prepared
	defer func() {
		for _, p := range allocated {
			if p.handle != 0 {
				kcall("GlobalFree", p.handle)
			}
		}
	}()
	if len(items) == 1 {
		for f, data := range items[0] {
			encoded, err := encodeWindows(f, data)
			if err != nil {
				return err
			}
			id := winFormat(f)
			if id == 0 {
				return errors.New("clipboard: format registration failed")
			}
			h := kcall("GlobalAlloc", 0x42, uintptr(max(1, len(encoded))))
			if h == 0 {
				return errors.New("clipboard: allocation failed")
			}
			allocated = append(allocated, prepared{id, h})
			p := kcall("GlobalLock", h)
			if p == 0 {
				return errors.New("clipboard: GlobalLock failed")
			}
			if len(encoded) > 0 {
				err = windows.WriteProcessMemory(windows.CurrentProcess(), p, &encoded[0], uintptr(len(encoded)), nil)
			}
			kcall("GlobalUnlock", h)
			runtime.KeepAlive(encoded)
			if err != nil {
				return err
			}
		}
	}
	if ucall("OpenClipboard", w.owner) == 0 {
		return errors.New("clipboard: system clipboard busy")
	}
	defer ucall("CloseClipboard")
	if ucall("EmptyClipboard") == 0 {
		return errors.New("clipboard: cannot clear clipboard")
	}
	for i, p := range allocated {
		if ucall("SetClipboardData", p.id, p.handle) == 0 {
			return errors.New("clipboard: native write failed")
		}
		allocated[i].handle = 0
	}
	return nil
}
func (w *winAdapter) read(wanted []string) ([]map[string][]byte, error) {
	if ucall("OpenClipboard", w.owner) == 0 {
		return nil, errors.New("clipboard: system clipboard busy")
	}
	defer ucall("CloseClipboard")
	values := map[string][]byte{}
	total := 0
	id := uintptr(0)
	for count := 0; count < 1024; count++ {
		id = ucall("EnumClipboardFormats", id)
		if id == 0 {
			break
		}
		f := winMIME(id)
		if f == "" || (len(wanted) > 0 && !slices.Contains(wanted, f)) {
			continue
		}
		h := ucall("GetClipboardData", id)
		if h == 0 {
			continue
		}
		size := kcall("GlobalSize", h)
		if size > uintptr(MaxBytes-total) {
			return nil, errors.New("clipboard: native data budget exceeded")
		}
		p := kcall("GlobalLock", h)
		if p == 0 {
			return nil, errors.New("clipboard: GlobalLock failed")
		}
		b := make([]byte, int(size))
		var err error
		if len(b) > 0 {
			err = windows.ReadProcessMemory(windows.CurrentProcess(), p, &b[0], size, nil)
		}
		kcall("GlobalUnlock", h)
		if err != nil {
			return nil, err
		}
		b, err = decodeWindows(f, b)
		if err != nil {
			return nil, err
		}
		total += len(b)
		values[f] = b
	}
	if len(values) == 0 {
		return nil, nil
	}
	return []map[string][]byte{values}, nil
}
func encodeWindows(f string, b []byte) ([]byte, error) {
	switch f {
	case Text:
		if strings.ContainsRune(string(b), 0) {
			return nil, errors.New("clipboard: text contains NUL")
		}
		units := utf16.Encode([]rune(string(b)))
		out := make([]byte, (len(units)+1)*2)
		for i, u := range units {
			binary.LittleEndian.PutUint16(out[i*2:], u)
		}
		return out, nil
	case HTML:
		return htmlClipboard(b), nil
	case PNG:
		return b, nil
	default:
		out := make([]byte, 8+len(b))
		binary.LittleEndian.PutUint64(out, uint64(len(b)))
		copy(out[8:], b)
		return out, nil
	}
}
func decodeWindows(f string, b []byte) ([]byte, error) {
	switch f {
	case Text:
		var units []uint16
		for i := 0; i+1 < len(b); i += 2 {
			u := binary.LittleEndian.Uint16(b[i:])
			if u == 0 {
				break
			}
			units = append(units, u)
		}
		return []byte(string(utf16.Decode(units))), nil
	case HTML:
		return htmlFragment(b)
	case PNG:
		return pngBytes(b)
	default:
		if len(b) < 8 {
			return nil, errors.New("clipboard: invalid private representation")
		}
		n := binary.LittleEndian.Uint64(b)
		if n > uint64(len(b)-8) {
			return nil, errors.New("clipboard: truncated representation")
		}
		return slices.Clone(b[8 : 8+int(n)]), nil
	}
}
