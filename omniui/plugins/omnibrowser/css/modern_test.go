package css

import (
	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"strings"
	"testing"
)

func TestLayerCascadeRangeAndLogicalSpacing(t *testing.T) {
	source := `<style>@layer reset,theme;@layer reset{:root,:host{--Size:clamp(20px, 5vw, 80px)}p{color:red!important;background-color:red;padding:0}}@layer theme{p{color:blue!important;background-color:blue}}p{color:green!important;margin-inline:12px;margin:3px;padding-inline:20px;font-size:var(--Size)}@media(width<=600px){p{padding-inline:9px}}</style><p>x</p>`
	root, _ := html.Parse(strings.NewReader(source))
	n := cascadia.Query(root, cascadia.MustCompile("p"))
	for _, width := range []int{500, 800} {
		s := DocumentViewport(root, width, 600).Styles(root)[n]
		want := "20px"
		if width == 500 {
			want = "9px"
		}
		if s["color"] != "red" || s["background-color"] != "blue" || s["padding-left"] != want || s["margin-left"] != "3px" || s["font-size"] != "clamp(20px, 5vw, 80px)" {
			t.Fatal(width, s)
		}
	}
}
func TestRangeMediaBoundaries(t *testing.T) {
	for query, want := range map[string]bool{"(width <= 64rem)": true, "(width < 1024px)": false, "(height >= 768px)": true, "(width > 1024px)": false, "(width>=800px) and (width<=1200px)": true} {
		if got := MediaMatches(query, 1024, 768); got != want {
			t.Fatal(query, got)
		}
	}
}
