package omnibrowser

import (
	"context"
	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"net/url"
	"strings"
)

// Load styles in document order, preserving cascade order and network policy.
// Resource errors are reported without discarding a usable document.
func (p *Page) loadStyles(ctx context.Context) error {
	base, _ := url.Parse(p.url)
	matches := cascadia.QueryAll(p.root, cascadia.MustCompile(`link[href]`))
	total := 0
	bytes := 0
	for _, n := range matches {
		rel := " " + strings.ToLower(attr(n, "rel")) + " "
		if !strings.Contains(rel, " stylesheet ") || hasAttr(n, "disabled") {
			continue
		}
		if total >= 64 {
			p.log("Stylesheet count limit exceeded")
			break
		}
		total++
		ref, err := url.Parse(attr(n, "href"))
		if err != nil {
			p.log(err.Error())
			continue
		}
		address := base.ResolveReference(ref).String()
		source, _, err := p.client.fetch(resourceContext(ctx, "Stylesheet"), address, false)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			p.log("Stylesheet failed: " + address + ": " + err.Error())
			continue
		}
		bytes += len(source)
		if bytes > 1<<20 || *p.nodes+2 > p.options.MaxNodes || len(source) > 512<<10 {
			p.log("Stylesheet exceeds 512 KiB: " + address)
			continue
		}
		// Retain link identity and insert its fetched sheet immediately after it.
		style := &html.Node{Type: html.ElementNode, Data: "style", DataAtom: atom.Style, Attr: []html.Attribute{{Key: "data-source", Val: address}}}
		if media := attr(n, "media"); media != "" {
			source = "@media " + media + "{" + source + "}"
		}
		*p.nodes += 2
		style.AppendChild(&html.Node{Type: html.TextNode, Data: source})
		n.Parent.InsertBefore(style, n.NextSibling)
	}
	return nil
}

func (p *Page) loadImages(ctx context.Context) error {
	p.images = map[string][]byte{}
	base, _ := url.Parse(p.url)
	total, attempts := 0, 0
	for _, n := range cascadia.QueryAll(p.root, cascadia.MustCompile("img[src]")) {
		if attempts >= 32 {
			p.log("Image count limit exceeded")
			break
		}
		ref, err := url.Parse(attr(n, "src"))
		if err != nil {
			continue
		}
		address := base.ResolveReference(ref).String()
		if _, ok := p.images[address]; ok {
			continue
		}
		attempts++
		data, _, err := p.client.fetch(resourceContext(ctx, "Image"), address, false)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			p.log("Image failed: " + address + ": " + err.Error())
			continue
		}
		total += len(data)
		if total > 8<<20 {
			p.log("Image byte quota exceeded")
			break
		}
		p.images[address] = []byte(data)
	}
	return nil
}
