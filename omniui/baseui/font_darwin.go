//go:build darwin

package baseui

import (
	"golang.org/x/image/font/opentype"
	"os"
)

// Read installed fonts, never redistribute Apple's assets. Unsupported or absent
// fonts retain the portable Go family. Parsed fonts are immutable shared data;
// each Surface still owns and closes its own sized faces.
func platformFonts(fallback []*opentype.Font) []*opentype.Font {
	result := append([]*opentype.Font(nil), fallback...)
	load := func(path string, index int) *opentype.Font {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		collection, err := opentype.ParseCollection(data)
		if err != nil {
			return nil
		}
		face, err := collection.Font(index)
		if err != nil {
			return nil
		}
		return face
	}
	if f := load("/System/Library/Fonts/SFNS.ttf", 0); f != nil {
		result[0] = f
	}
	// x/image does not expose variable-font weight axes. Use the installed static
	// Helvetica styles for emphasized text instead of synthesizing a fake stroke.
	for i := 1; i <= 3; i++ {
		if f := load("/System/Library/Fonts/Helvetica.ttc", i); f != nil {
			result[i] = f
		}
	}
	return result
}
