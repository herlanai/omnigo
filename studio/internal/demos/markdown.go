package demos

import (
	ui "omnigo.dev/omnigo/omniui/baseui"
	"omnigo.dev/omnigo/omniui/plugins/markdown"
)

type markdownDemo struct {
	state   markdown.State
	source  *ui.TextBuffer
	editing bool
}

const markdownSample = "# Native Markdown\n\nA **Go-rendered** document with *emphasis*, `inline code`, ~~removed text~~ and an [interactive link](https://example.com/docs).\n\n> Markdown belongs in the native control catalog.\n\n## Checklist\n\n- [x] Parse CommonMark and GFM\n- [x] Render real Go controls\n- [ ] Finish selection, images and advanced extensions\n\n| Feature | Status |\n| --- | --- |\n| Headings and links | Available |\n| Code blocks | Copy original bytes |\n\n```go\nui.Button(\"save\", \"Save\", save)\n```\n\n1. Edit the source.\n2. Return to preview.\n"

func (d *Demo) markdownContent() *ui.Element {
	if d.markdown == nil {
		d.markdown = &markdownDemo{source: ui.NewTextBuffer(markdownSample)}
	}
	m := d.markdown
	controls := ui.Row("md/controls", ui.Button("md/edit", "Edit / preview", func() { m.editing = !m.editing }).Variant(ui.Outline), ui.Button("md/append", "Append text", func() { m.source.Set(m.source.String() + "\nNew **streamed** paragraph.\n") }).Variant(ui.Outline), ui.Button("md/reset", "Reset", func() { m.source.Set(markdownSample); m.state.Reset() }).Variant(ui.Outline))
	if m.editing {
		return ui.Column("demo", controls, ui.Textarea("md/source", "Markdown source", m.source).Height(380))
	}
	view, e := m.state.View("md/document", m.source.String(), markdown.Options{OnLink: func(v string) { d.Message = "Link callback: " + v }, OnCopyCode: d.copyText})
	if e != nil {
		return ui.Column("demo", controls, ui.Text("md/error", e.Error()).Role("alert"))
	}
	return ui.Column("demo", controls, ui.ScrollArea("md/scroll", d.Scroll, view).Height(380))
}
