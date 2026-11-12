package demos

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"time"

	"omnigo.dev/omnigo/omnibrowser"
	browserrender "omnigo.dev/omnigo/omnibrowser/render"
	ui "omnigo.dev/omnigo/omniui/baseui"
)

const browserExample = `<!doctype html>
<html><head><title>OmniBrowser playground</title></head><body>
<h1>Browse with Go.</h1>
<p>A live DOM, JavaScript and native Go rendering.</p>
<input id="name" name="Name" value="OmniGo" placeholder="Your name">
<button id="greet">Run page action</button>
<p id="result">Ready to interact.</p>
<h2>Explore the document</h2>
<ul><li class="feature">DOM selectors</li><li class="feature">JavaScript evaluation</li><li class="feature">PNG snapshots</li></ul>
<script>
document.querySelector('#greet').addEventListener('click', function () {
 document.querySelector('#result').textContent = 'Hello, ' + document.querySelector('#name').value + '!';
});
console.log('Page initialized');
</script></body></html>`

type browserDemo struct {
	page                              *omnibrowser.Page
	source, script, selector, address *ui.TextBuffer
	snapshot                          omnibrowser.Snapshot
	tab, output                       string
	busy                              bool
	scroll                            ui.ScrollState
	shot                              image.Image
}

// Close releases demo-owned services; native windows call this before disposal.
func (d *Demo) Close() {
	if d.browser != nil && d.browser.page != nil {
		_ = d.browser.page.Close()
	}
}
func (d *Demo) browserContent() *ui.Element {
	if d.browser == nil {
		s := &browserDemo{page: omnibrowser.New(omnibrowser.Options{Timeout: 2 * time.Second}), source: ui.NewTextBuffer(browserExample), script: ui.NewTextBuffer(`Array.from(document.querySelectorAll('.feature')).map(el => el.textContent)`), selector: ui.NewTextBuffer(".feature"), address: ui.NewTextBuffer("https://example.com"), tab: "preview"}
		if err := s.page.SetContent(context.Background(), browserExample); err != nil {
			d.Message = err.Error()
		}
		s.snapshot, _ = s.page.Snapshot(context.Background())
		d.browser = s
	}
	s := d.browser
	run := func(action func() (string, image.Image, error)) {
		if s.busy {
			return
		}
		s.busy = true
		work := func() func() {
			output, shot, err := action()
			snapshot, snapshotErr := s.page.Snapshot(context.Background())
			if err == nil {
				err = snapshotErr
			}
			return func() {
				s.busy = false
				if err != nil {
					d.Message = err.Error()
					s.output = err.Error()
					return
				}
				s.snapshot = snapshot
				s.output = output
				if shot != nil {
					s.shot = shot
				}
				d.Message = "Page operation completed"
				if len(snapshot.ScriptErrors) > 0 {
					d.Message = fmt.Sprintf("Loaded with %d script errors: %s", len(snapshot.ScriptErrors), snapshot.ScriptErrors[0])
				}
			}
		}
		if d.RunTask != nil {
			d.RunTask(work)
		} else {
			work()()
		}
	}
	ctx := context.Background()
	var content *ui.Element
	switch s.tab {
	case "html":
		content = ui.Column("browser/html", ui.Textarea("browser/source", "HTML source", s.source).Height(220), ui.Button("browser/load", "Load HTML", func() {
			source := s.source.String()
			run(func() (string, image.Image, error) { return "Document loaded", nil, s.page.SetContent(ctx, source) })
		}))
	case "script":
		content = ui.Column("browser/javascript", ui.Textarea("browser/script", "JavaScript", s.script).Height(100), ui.Button("browser/eval", "Evaluate JavaScript", func() {
			script := s.script.String()
			run(func() (string, image.Image, error) {
				result, err := s.page.Eval(ctx, script)
				return string(result), nil, err
			})
		}), ui.Text("browser/value", s.output).Wrap(true))
	case "extract":
		content = ui.Column("browser/extract", ui.Input("browser/selector", "CSS selector", s.selector), ui.Button("browser/query", "Query DOM", func() {
			selector := s.selector.String()
			run(func() (string, image.Image, error) {
				matches, err := s.page.Query(ctx, selector)
				if err != nil {
					return "", nil, err
				}
				result, err := json.MarshalIndent(matches, "", "  ")
				return string(result), nil, err
			})
		}), ui.ScrollArea("browser/results", &s.scroll, ui.Text("browser/output", s.output).Wrap(true)).Height(220))
	case "screenshot":
		preview := ui.Text("browser/empty-shot", "Capture the current document as a Go-rendered PNG preview.")
		if s.shot != nil {
			preview = ui.Image("browser/image", "Document screenshot", s.shot, ui.ImageOptions{}).Height(260)
		}
		content = ui.Column("browser/screenshot", ui.Button("browser/capture", "Capture screenshot", func() {
			run(func() (string, image.Image, error) {
				snapshot, err := s.page.Snapshot(ctx)
				if err != nil {
					return "", nil, err
				}
				img, err := browserrender.Screenshot(snapshot, 800, 500, false)
				return "Captured 800 × 500", img, err
			})
		}), preview)
	default:
		content = ui.ScrollArea("browser/preview", &s.scroll, browserrender.View("browser/document", s.snapshot, func(selector string) {
			run(func() (string, image.Image, error) { return "Clicked " + selector, nil, s.page.Click(ctx, selector) })
		}, func(selector, value string) {
			run(func() (string, image.Image, error) { return "Input updated", nil, s.page.Fill(ctx, selector, value) })
		})).Height(340)
	}
	return ui.Column("browser", ui.Text("browser/info", "Pure Go document engine · no Chromium · semantic HTML preview").Muted(true).Wrap(true), ui.Row("browser/navigation", ui.Input("browser/url", "Page URL", s.address).Grow(1), ui.Button("browser/navigate", "Navigate", func() {
		address := s.address.String()
		run(func() (string, image.Image, error) { return "Loaded " + address, nil, s.page.Navigate(ctx, address) })
	}).Variant(ui.Outline).Disabled(d.RunTask == nil)), ui.Text("browser/title", fmt.Sprintf("%s · %s", s.snapshot.Title, s.snapshot.URL)).Wrap(true), ui.Tabs("browser/tabs", "Browser tools", s.tab, []ui.Option{{ID: "preview", Label: "Preview"}, {ID: "html", Label: "HTML"}, {ID: "script", Label: "JavaScript"}, {ID: "extract", Label: "Extract"}, {ID: "screenshot", Label: "Screenshot"}}, func(tab string) { s.tab = tab; s.scroll = ui.ScrollState{} }), content).ReadOnly(s.busy)
}
