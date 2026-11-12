package render

import (
	"bytes"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser"
)

func decodeImages(snapshot omnibrowser.Snapshot) map[string]image.Image {
	images := map[string]image.Image{}
	pixels := 0
	for key, data := range snapshot.Images {
		if len(data) > 2<<20 {
			continue
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 4096 || config.Height > 4096 || config.Width*config.Height > 4_000_000 {
			continue
		}
		pixels += config.Width * config.Height
		if pixels > 8_000_000 {
			break
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err == nil {
			images[key] = img
		}
	}
	return images
}
func imageAddress(base, address string) string {
	u, err := url.Parse(address)
	if err != nil {
		return ""
	}
	b, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return b.ResolveReference(u).String()
}
