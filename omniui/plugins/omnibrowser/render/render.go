// Package render provides a semantic HTML preview using OmniUI's Go rasterizer.
// It deliberately does not claim CSS layout or Chromium screenshot compatibility.
package render

import (
	"fmt"
	"image"
	"strings"

	"golang.org/x/net/html"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser"
)

// View builds a detached view. Callers provide actions, so no browser engine is
// retained in a paint context. Native controls execute on the UI goroutine.
func View(id string, snapshot omnibrowser.Snapshot, onClick func(string), onFill func(string, string)) *ui.Element {
	root, err := html.Parse(strings.NewReader(snapshot.HTML))
	if err != nil {
		return ui.Text(id, err.Error())
	}
	count := 0
	var walk func(*html.Node) []*ui.Element
	walk = func(n *html.Node) []*ui.Element {
		attribute := func(key string) string {
			for _, a := range n.Attr {
				if a.Key == key {
					return a.Val
				}
			}
			return ""
		}
		has := func(key string) bool {
			for _, a := range n.Attr {
				if a.Key == key {
					return true
				}
			}
			return false
		}
		if count >= 500 || has("hidden") || strings.Contains(strings.ReplaceAll(attribute("style"), " ", ""), "display:none") {
			return nil
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "head", "style", "script", "noscript", "template":
				return nil
			}
		}
		if n.Type == html.TextNode {
			text := strings.Join(strings.Fields(n.Data), " ")
			if text == "" {
				return nil
			}
			count++
			return []*ui.Element{ui.Text(fmt.Sprintf("%s/text/%d", id, count), text).Wrap(true)}
		}
		count++
		key := fmt.Sprintf("%s/node/%d", id, count)
		var text strings.Builder
		var gather func(*html.Node)
		gather = func(c *html.Node) {
			if c.Type == html.TextNode {
				text.WriteString(c.Data)
			}
			for child := c.FirstChild; child != nil; child = child.NextSibling {
				gather(child)
			}
		}
		gather(n)
		selector := ""
		if attribute("id") != "" {
			selector = `[id="` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(attribute("id")) + `"]`
		}
		switch n.Data {
		case "h1", "h2", "h3", "h4", "h5", "h6":
			size := 18.
			if n.Data == "h1" {
				size = 28
			} else if n.Data == "h2" {
				size = 22
			}
			return []*ui.Element{ui.Text(key, text.String()).FontSize(size).Wrap(true)}
		case "button":
			return []*ui.Element{ui.Button(key, text.String(), func() {
				if onClick != nil {
					onClick(selector)
				}
			}).Variant(ui.Outline).Disabled(has("disabled") || selector == "" || onClick == nil)}
		case "input", "textarea":
			value := attribute("value")
			if n.Data == "textarea" && value == "" {
				value = text.String()
			}
			buffer := ui.NewTextBuffer(value)
			input := ui.Input(key, attribute("name"), buffer).Placeholder(attribute("placeholder")).Disabled(has("disabled")).ReadOnly(has("readonly") || onFill == nil || selector == "")
			buffer.OnChange = func(value string) {
				if onFill != nil {
					onFill(selector, value)
				}
			}
			return []*ui.Element{input}
		case "a":
			return []*ui.Element{ui.Text(key, text.String()+"  → "+attribute("href")).Wrap(true)}
		case "hr":
			return []*ui.Element{ui.Separator(key, false)}
		case "img":
			return []*ui.Element{ui.Text(key, "[Image: "+attribute("alt")+"]").Muted(true)}
		}
		children := []*ui.Element{}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			children = append(children, walk(child)...)
		}
		if len(children) == 0 {
			return nil
		}
		return []*ui.Element{ui.Column(key, children...).Gap(8)}
	}
	return ui.Column(id, walk(root)...).Gap(12).Padding(20)
}

// Screenshot returns a viewport PNG-ready image of the semantic preview.
func SemanticScreenshot(snapshot omnibrowser.Snapshot, width, height int, dark bool) (*image.RGBA, error) {
	if width < 64 || height < 64 || width > 4096 || height > 4096 || width*height > 8_000_000 {
		return nil, fmt.Errorf("omnibrowser: invalid screenshot dimensions")
	}
	surface, err := ui.NewSurface(width, height, ui.NewTheme(dark))
	if err != nil {
		return nil, err
	}
	defer surface.Close()
	if err = surface.Mount(View("document", snapshot, nil, nil)); err != nil {
		return nil, err
	}
	return surface.Render()
}
