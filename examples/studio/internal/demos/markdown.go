package demos

import (
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/omniui/plugins/markdown"
)

type markdownDemo struct {
	state   markdown.State
	source  *ui.TextBuffer
	editing bool
	wrap    bool
}

const markdownSample = "# Native Markdown\n\n阅读体验 · Native Go rendering\n\nReadable **strong text**, *emphasis*, `inline code` and an [interactive link](https://example.com/docs).\n\n> [!NOTE]\n> A native document with aligned tables, task indicators and wrapped code.\n\n## Checklist\n\n- [x] Read CommonMark and GFM\n- [x] Render 简体中文、繁體中文、日本語\n- [ ] Complete cross-block text selection\n\n| Feature | Alignment | Count |\n| :--- | :---: | ---: |\n| **Native controls** | Center | 128 |\n| Wrapped paragraphs with longer text | Center | 2,048 |\n\n```go\n// Toggle Wrap code to compare long lines.\nfunc greeting(name string) string {\n\treturn fmt.Sprintf(\"Hello, %s — welcome to a document rendered entirely with native Go controls.\", name)\n}\n```\n\n### Nested lists\n\n1. Edit the source\n   - Keep **inline formatting**\n   - Preserve the original code when copying\n2. Return to preview\n\n> A quotation keeps its own visual hierarchy.\n\n---\n\nA footnote reference[^1].\n\n[^1]: The source remains available for editing and copying.\n"

func (d *Demo) markdownContent() *ui.Element {
	if d.markdown == nil {
		d.markdown = &markdownDemo{source: ui.NewTextBuffer(markdownSample)}
	}
	m := d.markdown
	controls := ui.Row("md/controls", ui.Button("md/edit", "Edit / preview", func() { m.editing = !m.editing }).Variant(ui.Outline), ui.Button("md/append", "Append text", func() { m.source.Set(m.source.String() + "\nNew **streamed** paragraph.\n") }).Variant(ui.Outline), ui.Checkbox("md/wrap", "Wrap code", m.wrap, func(v bool) { m.wrap = v }), ui.Button("md/copy", "Copy Markdown", func() { d.copyText(m.source.String()) }).Variant(ui.Outline), ui.Button("md/reset", "Reset", func() { m.source.Set(markdownSample); m.state.Reset() }).Variant(ui.Outline))
	if m.editing {
		return ui.Column("demo", controls, ui.Textarea("md/source", "Markdown source", m.source).Height(460))
	}
	view, e := m.state.View("md/document", m.source.String(), markdown.Options{WrapCode: m.wrap, OnLink: func(v string) { d.Message = "Link callback: " + v }, OnCopyCode: d.copyText})
	if e != nil {
		return ui.Column("demo", controls, ui.Text("md/error", e.Error()).Role("alert"))
	}
	return ui.Column("demo", controls, ui.ScrollArea("md/scroll", d.Scroll, view).Height(460))
}
