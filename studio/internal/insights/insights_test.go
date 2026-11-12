package insights

import (
	"image/png"
	"net/http/httptest"
	"testing"
)

func TestIconPreview(t *testing.T) {
	h := IconHandler()
	for _, dark := range []string{"false", "true"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/studio/icon.png?name=activity&size=24&stroke=2&dark="+dark, nil))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		img, e := png.Decode(w.Body)
		if e != nil || img.Bounds().Dx() != 48 {
			t.Fatal(e)
		}
		visible := false
		for y := 0; y < 48; y++ {
			for x := 0; x < 48; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				visible = visible || a > 0
			}
		}
		if !visible {
			t.Fatal("empty preview")
		}
	}
	for _, q := range []string{"name=missing&size=24&stroke=2", "name=../activity&size=24&stroke=2", "name=activity&size=999&stroke=2", "name=activity&size=24&stroke=NaN"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/studio/icon.png?"+q, nil))
		if w.Code < 400 {
			t.Fatal(q)
		}
	}
}
