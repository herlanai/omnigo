package baseui

import (
	"embed"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"sync"
)

//go:embed fonts/inter/*.ttf
var interData embed.FS

var interFonts = func() map[int]func() *opentype.Font {
	result := map[int]func() *opentype.Font{}
	for weight, name := range map[int]string{400: "Regular", 500: "Medium", 600: "SemiBold", 700: "Bold", 401: "Italic", 701: "BoldItalic"} {
		result[weight] = sync.OnceValue(func() *opentype.Font {
			data, err := interData.ReadFile("fonts/inter/Inter-" + name + ".ttf")
			if err != nil {
				panic(err)
			}
			parsed, err := opentype.Parse(data)
			if err != nil {
				panic(err)
			}
			return parsed
		})
	}
	return result
}()

func weightedFace(weight int, mono bool, size float64) font.Face {
	if mono {
		style := 4
		if weight >= 600 {
			style = 5
		}
		return richFace(style, size)
	}
	if weight < 400 {
		weight = 400
	}
	f, err := opentype.NewFace(interFonts[weight](), &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err)
	}
	return f
}
func textWeight(e *Element) int {
	if e.fontWeight <= 400 {
		return 0
	}
	return e.fontWeight
}
