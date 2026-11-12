package render

import (
	_ "embed"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"image"
	"os"
	"path/filepath"
	"sync"
)

// Bundled OFL fonts make Chinese rendering independent of host installations.
//
//go:embed fonts/NotoSansSC-Regular.otf
var cjkRegular []byte

//go:embed fonts/NotoSansSC-Bold.otf
var cjkBold []byte

//go:embed fonts/Inter-Regular.ttf
var latinRegular []byte

//go:embed fonts/Inter-Bold.ttf
var latinBold []byte
var bundledFonts = sync.OnceValue(func() [4]*opentype.Font {
	var fonts [4]*opentype.Font
	for i, data := range [][]byte{latinRegular, cjkRegular, latinBold, cjkBold} {
		f, err := opentype.Parse(data)
		if err != nil {
			panic(err)
		}
		fonts[i] = f
	}
	return fonts
})
var parsedFonts = sync.OnceValue(func() []*opentype.Font {
	var fonts []*opentype.Font
	for _, path := range filepath.SplitList(os.Getenv("OMNIBROWSER_FONT_PATH")) {
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() > 64<<20 {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		collection, err := opentype.ParseCollection(data)
		if err != nil {
			continue
		}
		for i := 0; i < min(8, collection.NumFonts()); i++ {
			f, err := collection.Font(i)
			if err == nil {
				fonts = append(fonts, f)
			}
		}
	}
	return fonts
})

type fallbackFace struct{ faces []font.Face }

func newTextFace(size float64) font.Face { return newStyledFace(size, false) }
func newStyledFace(size float64, bold bool) font.Face {
	f := &fallbackFace{}
	fonts := bundledFonts()
	offset := 0
	if bold {
		offset = 2
	}
	sources := append([]*opentype.Font{}, parsedFonts()...)
	sources = append(sources, fonts[offset], fonts[offset+1])
	for _, parsed := range sources {
		face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		if err == nil {
			f.faces = append(f.faces, face)
		}
	}
	return f
}
func (f *fallbackFace) pick(r rune) font.Face {
	for _, face := range f.faces {
		if _, ok := face.GlyphAdvance(r); ok {
			return face
		}
	}
	return f.faces[0]
}
func (f *fallbackFace) Close() error {
	for _, face := range f.faces {
		face.Close()
	}
	return nil
}
func (f *fallbackFace) Glyph(dot fixed.Point26_6, r rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	return f.pick(r).Glyph(dot, r)
}
func (f *fallbackFace) GlyphBounds(r rune) (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	return f.pick(r).GlyphBounds(r)
}
func (f *fallbackFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) { return f.pick(r).GlyphAdvance(r) }
func (f *fallbackFace) Kern(a, b rune) fixed.Int26_6 {
	fa, fb := f.pick(a), f.pick(b)
	if fa != fb {
		return 0
	}
	return fa.Kern(a, b)
}
func (f *fallbackFace) Metrics() font.Metrics {
	m := f.faces[0].Metrics()
	for _, face := range f.faces[1:] {
		other := face.Metrics()
		m.Ascent = max(m.Ascent, other.Ascent)
		m.Descent = max(m.Descent, other.Descent)
		m.Height = max(m.Height, other.Height)
	}
	return m
}
