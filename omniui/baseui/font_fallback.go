package baseui

import (
	"image"
	"os"
	"runtime"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// System fonts remain on the user's machine; no OS font is redistributed.
// Parse lazily, so Latin-only applications do not load large CJK font files.
var systemFallbackFont = sync.OnceValue(func() *opentype.Font {
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		paths = []string{"/System/Library/Fonts/Supplemental/Arial Unicode.ttf", "/System/Library/Fonts/Hiragino Sans GB.ttc"}
	case "windows":
		root := os.Getenv("WINDIR")
		if root == "" {
			root = `C:\Windows`
		}
		paths = []string{root + `\Fonts\msyh.ttc`, root + `\Fonts\mingliu.ttc`}
	case "linux":
		paths = []string{"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", "/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc", "/usr/share/fonts/google-noto-cjk/NotoSansCJK-Regular.ttc"}
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if f, err := opentype.Parse(data); err == nil {
			return f
		}
		if c, err := opentype.ParseCollection(data); err == nil {
			if f, err := c.Font(0); err == nil {
				return f
			}
		}
	}
	return nil
})

type fallbackFace struct {
	primary     font.Face
	fallback    font.Face
	size        float64
	initialized bool
}

func withSystemFallback(primary font.Face, size float64) font.Face {
	return &fallbackFace{primary: primary, size: size}
}
func (f *fallbackFace) face(r rune) font.Face {
	if _, ok := f.primary.GlyphAdvance(r); ok {
		return f.primary
	}
	if !f.initialized {
		f.initialized = true
		if parsed := systemFallbackFont(); parsed != nil {
			f.fallback, _ = opentype.NewFace(parsed, &opentype.FaceOptions{Size: f.size, DPI: 72, Hinting: font.HintingNone})
		}
	}
	if f.fallback != nil {
		if _, ok := f.fallback.GlyphAdvance(r); ok {
			return f.fallback
		}
	}
	return f.primary
}
func (f *fallbackFace) Glyph(dot fixed.Point26_6, r rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	return f.face(r).Glyph(dot, r)
}
func (f *fallbackFace) GlyphBounds(r rune) (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	return f.face(r).GlyphBounds(r)
}
func (f *fallbackFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) { return f.face(r).GlyphAdvance(r) }
func (f *fallbackFace) Kern(a, b rune) fixed.Int26_6 {
	first, second := f.face(a), f.face(b)
	if first != second {
		return 0
	}
	return first.Kern(a, b)
}
func (f *fallbackFace) Metrics() font.Metrics { return f.primary.Metrics() }
func (f *fallbackFace) Close() error {
	err := f.primary.Close()
	if f.fallback != nil {
		if e := f.fallback.Close(); err == nil {
			err = e
		}
	}
	return err
}
