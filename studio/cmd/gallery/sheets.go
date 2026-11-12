package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func writeReviewSheets(directory string, ids []string) error {
	const columns, rows, cellW, cellH = 4, 4, 300, 308
	for _, theme := range []string{"light", "dark"} {
		for start := 0; start < len(ids); start += columns * rows {
			sheet := image.NewRGBA(image.Rect(0, 0, columns*cellW, rows*cellH))
			draw.Draw(sheet, sheet.Bounds(), image.NewUniform(color.NRGBA{R: 220, G: 220, B: 220, A: 255}), image.Point{}, draw.Src)
			for i := start; i < min(len(ids), start+columns*rows); i++ {
				f, err := os.Open(filepath.Join(directory, strings.ReplaceAll(ids[i], "/", "-")+"-"+theme+".png"))
				if err != nil {
					return err
				}
				img, err := png.Decode(f)
				f.Close()
				if err != nil {
					return err
				}
				local := i - start
				x, y := (local%columns)*cellW, (local/columns)*cellH
				drawer := font.Drawer{Dst: sheet, Src: image.NewUniform(color.Black), Face: basicfont.Face7x13, Dot: fixed.P(x+8, y+17)}
				drawer.DrawString(ids[i])
				draw.ApproxBiLinear.Scale(sheet, image.Rect(x, y+24, x+cellW, y+cellH), img, img.Bounds(), draw.Src, nil)
			}
			path := filepath.Join(directory, fmt.Sprintf("sheet-%s-%02d.png", theme, start/(columns*rows)+1))
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			err = png.Encode(f, sheet)
			closeErr := f.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
			fmt.Println(path)
		}
	}
	return nil
}
