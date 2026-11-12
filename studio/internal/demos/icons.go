package demos

import (
	"fmt"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/omniui/icon/lucide"
	"strings"
)

type iconDemo struct {
	query        *ui.TextBuffer
	size, stroke float64
	page         int
	selected     string
}

func (d *Demo) iconContent() *ui.Element {
	if d.icons == nil {
		d.icons = &iconDemo{query: ui.NewTextBuffer(""), size: 24, stroke: 2, selected: "activity"}
	}
	state := d.icons
	names := []string{}
	for _, name := range lucide.Names() {
		if strings.Contains(name, strings.ToLower(state.query.String())) {
			names = append(names, name)
		}
	}
	pages := max(1, (len(names)+23)/24)
	state.page = max(0, min(state.page, pages-1))
	cells := []*ui.Element{}
	for _, name := range names[min(len(names), state.page*24):min(len(names), (state.page+1)*24)] {
		name := name
		cells = append(cells, ui.Column("icons/cell/"+name, ui.Icon("icons/image/"+name, name, lucide.Must(name), ui.IconOptions{Size: state.size, StrokeWidth: state.stroke}), ui.Text("icons/name/"+name, name).Wrap(true), ui.Button("icons/select/"+name, "Select", func() { state.selected = name; d.Message = "Selected: " + name }).Variant(ui.Ghost).Size(ui.ControlXS)))
	}
	source := fmt.Sprintf("baseui.Icon(\n    \"icon\", \"%s\", lucide.Must(\"%s\"),\n    baseui.IconOptions{Size: %g, StrokeWidth: %g},\n)", state.selected, state.selected, state.size, state.stroke)
	return ui.Column("demo",
		ui.Text("icons/version", fmt.Sprintf("Lucide %s · %d matches · page %d / %d", lucide.Version, len(names), state.page+1, pages)),
		ui.SearchField("icons/search", "Search Lucide icons", state.query, func(string) { state.page = 0 }),
		ui.Row("icons/options", ui.Text("icons/size-label", "Size"), ui.Slider("icons/size", "Icon size", state.size, 16, 48, 4, func(v float64) { state.size = v }).Grow(1), ui.Text("icons/stroke-label", "Stroke"), ui.Slider("icons/stroke", "Stroke width", state.stroke, .5, 4, .5, func(v float64) { state.stroke = v }).Grow(1)),
		ui.Row("icons/actions", ui.IconButton("icons/previous", "Previous page", lucide.Must("chevron-left"), func() { state.page-- }).Variant(ui.Outline).Disabled(state.page == 0), ui.IconButton("icons/next", "Next page", lucide.ChevronRight(), func() { state.page++ }).Variant(ui.Outline).Disabled(state.page == pages-1), ui.IconButton("icons/example", "Example action", lucide.Check(), func() { d.Message = "Icon button activated" }), ui.IconButton("icons/disabled", "Disabled action", lucide.Trash2(), func() {}).Disabled(true)),
		ui.Row("icons/selected", ui.Icon("icons/selected/image", state.selected, lucide.Must(state.selected), ui.IconOptions{Size: state.size, StrokeWidth: state.stroke}), ui.Text("icons/selected/name", state.selected).Grow(1), ui.Button("icons/copy-name", "Copy name", func() { d.copyText(state.selected) }).Variant(ui.Outline).Size(ui.ControlSmall)),
		ui.ScrollArea("icons/scroll", d.Scroll, ui.Grid("icons/grid", 4, 12, cells...)).Height(240),
		ui.CodeBlock("icons/code", "Go · baseui + lucide", source, d.Code, 0, d.copyText))
}
