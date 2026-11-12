package css

import (
	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"strings"
	"testing"
)

func TestViewportVariablesAndQuotedComments(t *testing.T) {
	root, _ := html.Parse(strings.NewReader(`<style>:root{--ink:#123456}p{color:var(--ink);content:"/* kept */"}@media screen and (max-width:600px){p{color:red}}@media print{p{color:blue}}</style><p>中文</p>`))
	n := cascadia.Query(root, cascadia.MustCompile("p"))
	s := DocumentViewport(root, 800, 600).Styles(root)[n]
	if s["color"] != "#123456" || s["content"] != `"/* kept */"` {
		t.Fatal(s)
	}
	if got := DocumentViewport(root, 500, 600).Styles(root)[n]["color"]; got != "red" {
		t.Fatal(got)
	}
}
