package baseui

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

// Fonts stay on the user's Mac. Match PostScript names, since collection
// ordering and MobileAsset directories vary across macOS releases.
var systemUIFonts = sync.OnceValue(func() map[int]*opentype.Font {
	fonts := map[int]*opentype.Font{}
	if runtime.GOOS != "darwin" {
		return fonts
	}
	paths := []string{"/System/Library/Fonts/PingFang.ttc"}
	assets, _ := filepath.Glob("/System/Library/AssetsV2/com_apple_MobileAsset_Font*/*.asset/AssetData/PingFang.ttc")
	paths = append(paths, assets...)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		collection, err := opentype.ParseCollection(data)
		if err != nil {
			continue
		}
		candidate := map[int]*opentype.Font{}
		for i := 0; i < collection.NumFonts(); i++ {
			f, err := collection.Font(i)
			if err != nil {
				continue
			}
			name, _ := f.Name(nil, sfnt.NameIDPostScript)
			switch name {
			case "PingFangSC-Regular":
				candidate[400] = f
			case "PingFangSC-Medium":
				candidate[500] = f
			case "PingFangSC-Semibold":
				candidate[600] = f
				candidate[700] = f
			}
		}
		if candidate[400] != nil && candidate[500] != nil && candidate[600] != nil {
			return candidate
		}
	}
	return fonts
})

var preferSystemUI atomic.Bool

// UseSystemUIFont opts into the installed macOS PingFang SC UI family.
// Call before creating any Surfaces, so cached measurements and painting agree.
// Returns false when unavailable; bundled fonts remain the fallback.
func UseSystemUIFont() bool {
	available := len(systemUIFonts()) != 0
	preferSystemUI.Store(available)
	return available
}

func interfaceFont(weight int) *opentype.Font {
	if !preferSystemUI.Load() {
		return interFonts[weight]()
	}
	// PingFang has no italic face; preserve its family for italic UI runs.
	systemWeight := weight
	if weight == 401 {
		systemWeight = 400
	}
	if weight == 701 {
		systemWeight = 700
	}
	if f := systemUIFonts()[systemWeight]; f != nil {
		return f
	}
	return interFonts[weight]()
}
