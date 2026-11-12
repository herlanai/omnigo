//go:build !darwin

package baseui

import "golang.org/x/image/font/opentype"

func platformFonts(fonts []*opentype.Font) []*opentype.Font { return fonts }
