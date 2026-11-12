//go:build darwin && (amd64 || arm64)

package clipboard

import (
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var loadOnce sync.Once
var loadError error

func selector(s string) objc.SEL { return objc.RegisterName(s) }
func ns(s string) objc.ID {
	return objc.ID(objc.GetClass("NSString")).Send(selector("stringWithUTF8String:"), s)
}

var nativeTypes = map[string]string{Text: "public.utf8-plain-text", HTML: "public.html", PNG: "public.png", URIList: "org.omnigo.uri-list", Markdown: "net.daringfireball.markdown"}

func nativeType(f string) string {
	if t := nativeTypes[f]; t != "" {
		return t
	}
	return "org.omnigo.mime." + hex.EncodeToString([]byte(f))
}
func mimeType(t string) string {
	for f, n := range nativeTypes {
		if t == n {
			return f
		}
	}
	const prefix = "org.omnigo.mime."
	if len(t) > len(prefix) && t[:len(prefix)] == prefix {
		b, err := hex.DecodeString(t[len(prefix):])
		if err != nil {
			return ""
		}
		f, err := format(string(b))
		if err != nil {
			return ""
		}
		return f
	}
	return ""
}

type macAdapter struct{ board objc.ID }

func openAdapter(o Options) (adapter, error) {
	loadOnce.Do(func() {
		_, loadError = purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	})
	if loadError != nil {
		return nil, loadError
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(selector("new"))
	defer pool.Send(selector("drain"))
	var board objc.ID
	if o.Name == "" {
		board = objc.ID(objc.GetClass("NSPasteboard")).Send(selector("generalPasteboard"))
	} else {
		board = objc.ID(objc.GetClass("NSPasteboard")).Send(selector("pasteboardWithName:"), ns(o.Name))
	}
	if board == 0 {
		return nil, errors.New("clipboard: pasteboard unavailable")
	}
	board.Send(selector("retain"))
	return &macAdapter{board: board}, nil
}
func (m *macAdapter) close() { m.board.Send(selector("release")); m.board = 0 }
func (m *macAdapter) write(items []map[string][]byte) error {
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(selector("new"))
	defer pool.Send(selector("drain"))
	array := objc.ID(objc.GetClass("NSMutableArray")).Send(selector("array"))
	for _, values := range items {
		item := objc.ID(objc.GetClass("NSPasteboardItem")).Send(selector("new"))
		for f, b := range values {
			var p unsafe.Pointer
			if len(b) > 0 {
				p = unsafe.Pointer(&b[0])
			}
			data := objc.ID(objc.GetClass("NSData")).Send(selector("dataWithBytes:length:"), p, uintptr(len(b)))
			if item.Send(selector("setData:forType:"), data, ns(nativeType(f))) == 0 {
				item.Send(selector("release"))
				return fmt.Errorf("clipboard: cannot encode %s", f)
			}
		}
		array.Send(selector("addObject:"), item)
		item.Send(selector("release"))
	}
	m.board.Send(selector("clearContents"))
	if len(items) > 0 && m.board.Send(selector("writeObjects:"), array) == 0 {
		return errors.New("clipboard: pasteboard write failed")
	}
	return nil
}
func (m *macAdapter) read(wanted []string) ([]map[string][]byte, error) {
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(selector("new"))
	defer pool.Send(selector("drain"))
	array := m.board.Send(selector("pasteboardItems"))
	count := int(array.Send(selector("count")))
	if count > 128 {
		return nil, errors.New("clipboard: native item budget exceeded")
	}
	var out []map[string][]byte
	total := 0
	for i := 0; i < count; i++ {
		item := array.Send(selector("objectAtIndex:"), uintptr(i))
		types := item.Send(selector("types"))
		n := int(types.Send(selector("count")))
		if n > 1024 {
			return nil, errors.New("clipboard: native format budget exceeded")
		}
		values := map[string][]byte{}
		for j := 0; j < n; j++ {
			typ := types.Send(selector("objectAtIndex:"), uintptr(j))
			f := mimeType(objc.Send[string](typ, selector("UTF8String")))
			if f == "" || (len(wanted) > 0 && !slices.Contains(wanted, f)) {
				continue
			}
			data := item.Send(selector("dataForType:"), typ)
			if data == 0 {
				continue
			}
			size := int(data.Send(selector("length")))
			if size < 0 || size > MaxBytes-total {
				return nil, errors.New("clipboard: native data budget exceeded")
			}
			total += size
			p := objc.Send[unsafe.Pointer](data, selector("bytes"))
			if size > 0 && p == nil {
				return nil, errors.New("clipboard: missing native data")
			}
			values[f] = slices.Clone(unsafe.Slice((*byte)(p), size))
		}
		if len(values) > 0 {
			out = append(out, values)
		}
	}
	return out, nil
}
