package insights

import (
	"image/color"
	"image/png"
	"net/http"
	"omnigo.dev/omnigo/omniui/icon"
	"omnigo.dev/omnigo/omniui/icon/lucide"
	"strconv"
)

type Icons struct {
	Names   []string `json:"names"`
	Version string   `json:"version"`
}

func IconCatalog() Icons { return Icons{lucide.Names(), lucide.Version} }

// IconHandler uses the same Go rasterizer as baseui.Icon. No browser SVG renderer
// or unbounded server-side image cache is involved in the Studio preview.
func IconHandler() http.Handler {
	slots := make(chan struct{}, 4)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		case <-r.Context().Done():
			return
		}
		q := r.URL.Query()
		size, e := strconv.Atoi(q.Get("size"))
		if e != nil || size < 16 || size > 96 {
			http.Error(w, "invalid size", 400)
			return
		}
		stroke, e := strconv.ParseFloat(q.Get("stroke"), 64)
		if e != nil || !(stroke >= .5 && stroke <= 4) {
			http.Error(w, "invalid stroke", 400)
			return
		}
		data, e := lucide.SVG(q.Get("name"))
		if e != nil {
			http.NotFound(w, r)
			return
		}
		svg, e := icon.ParseSVG(data)
		if e != nil {
			http.Error(w, "cannot parse icon", 500)
			return
		}
		ink := color.NRGBA{R: 36, G: 45, B: 43, A: 255}
		if q.Get("dark") == "true" {
			ink = color.NRGBA{R: 228, G: 238, B: 231, A: 255}
		}
		img, e := svg.Rasterize(size*2, size*2, ink, stroke)
		if e != nil {
			http.Error(w, "cannot render icon", 500)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "private, max-age=3600")
		_ = png.Encode(w, img)
	})
}
