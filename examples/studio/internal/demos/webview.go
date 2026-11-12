package demos

import (
	"fmt"
	"github.com/herlanai/omnigo/omniwindow/host"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"strings"
)

type webViewDemo struct {
	page         host.WebPage
	document     int
	zoom         float64
	hidden, dark bool
}

func webDocument(document int, dark bool) string {
	body := `<h1>Notes in a native WebView</h1><p>输入文字、选择内容，或点击按钮执行页面 JavaScript。</p><label>你的名字 <input id="name" placeholder="OmniGo"></label><button onclick="document.querySelector('#result').textContent='你好，'+(document.querySelector('#name').value||'OmniGo')">打个招呼</button><p id="result" aria-live="polite">等待交互</p><label>便笺<textarea placeholder="在这里编辑便笺；隐藏再显示会保留内容。"></textarea></label><a href="#details">跳到页面下方</a><div style="height:220px"></div><h2 id="details">页面导航</h2><p>使用 Go 工具栏中的 Back / Forward 测试锚点历史。</p>`
	if document == 1 {
		body = `<h1>Go layout + WebKit</h1><p>工具栏由 Go baseui 绘制，下方页面由真正的 WKWebView 显示。</p><ul><li>离线 HTML、原生输入和页面 JavaScript</li><li>页面缩放与锚点导航</li><li>隐藏保留状态；切换文档或 Reset 会重新加载</li></ul><p>本例尚未提供多标签保活、Go/JS 双向桥接和任意浮层叠加。</p>`
	}
	theme := "light"
	if dark {
		theme = "dark"
	}
	return strings.ReplaceAll(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><style>:root{color-scheme:THEME}body{font:15px -apple-system,sans-serif;margin:24px;line-height:1.6}input,textarea,button{font:inherit;padding:8px;border:1px solid #8888;border-radius:8px}textarea{display:block;width:90%;height:80px}button{margin-left:8px;cursor:pointer}a{color:#4589ee}</style>`+body+`</html>`, "THEME", theme)
}
func (d *Demo) webViewContent() *ui.Element {
	if d.webDemo == nil {
		d.webDemo = &webViewDemo{zoom: 1}
	}
	w := d.webDemo
	load := func(document int) {
		w.document = document
		if w.page != nil {
			w.page.LoadHTML(webDocument(document, w.dark))
		}
	}
	toolbar := ui.Row("web/toolbar",
		ui.Button("web/notes", "Notes", func() { load(0) }),
		ui.Button("web/about", "About", func() { load(1) }),
		ui.Button("web/back", "Back", func() {
			if w.page != nil {
				w.page.Back()
			}
		}),
		ui.Button("web/forward", "Forward", func() {
			if w.page != nil {
				w.page.Forward()
			}
		}),
		ui.Button("web/reset", "Reset", func() { load(w.document) }),
		ui.Button("web/zoom", "Zoom", func() {
			w.zoom += .25
			if w.zoom > 1.5 {
				w.zoom = .75
			}
			if w.page != nil {
				w.page.SetZoom(w.zoom)
			}
		}),
		ui.Button("web/visibility", "Hide / Show", func() { w.hidden = !w.hidden }),
	).Gap(8)
	children := []*ui.Element{toolbar, ui.Text("web/status", fmt.Sprintf("WKWebView · %.0f%% · document switches reset page state", w.zoom*100))}
	if !w.hidden {
		children = append(children, ui.WebView("native-web", d.NativeWeb).Height(340))
	} else {
		children = append(children, ui.Text("web/hidden", "Page hidden — click Hide / Show to restore it."))
	}
	return ui.Column("web/demo", children...).Gap(12)
}
