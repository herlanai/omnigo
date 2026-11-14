package demos

import (
	"fmt"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"image"
	"image/color"
	"time"
)

var uiControlIDs = []string{"go-to-line", "completion-menu", "diff-view", "virtual-list", "grid-view", "code-editor", "richtext-editor", "timeline", "video", "decimal-input", "outline", "outline-table", "number-input", "date-input", "time-input", "search-field", "rating", "range-slider", "token-field", "toolbar", "color-picker", "color-well", "image", "grid", "checkbox-group", "editable-text", "find-bar"}

type uiControlDemo struct {
	gotoLine      ui.GoToLineState
	completion    ui.CompletionState
	completionDoc *ui.TextBuffer
	diff          *ui.DiffState
	virtual       ui.VirtualListState
	editor        ui.CodeEditorState
	rich          *ui.RichEditorState
	timeline      ui.AnimationTimeline
	video         *ui.VideoPlayer
	decimal       ui.DecimalInputState
	outline       ui.OutlineState[string]
	edit          ui.EditableTextState
	find          ui.FindBarState
	number        ui.NumberInputState
	date          ui.DateInputState
	clock         ui.TimeInputState
	color         ui.ColorPickerState
	tokens        ui.TokenFieldState
	rangeValue    ui.RangeSliderState
	rating        int
	bold          bool
}

func (d *Demo) uiControlContent() *ui.Element {
	if d.uiControls == nil {
		now := time.Date(2026, 11, 11, 14, 30, 0, 0, time.UTC)
		d.uiControls = &uiControlDemo{number: ui.NumberInputState{Value: 2.5}, date: ui.DateInputState{Value: now}, clock: ui.TimeInputState{Value: now}, color: ui.ColorPickerState{Value: color.NRGBA{R: 60, G: 110, B: 220, A: 255}}, tokens: ui.TokenFieldState{Tokens: []string{"Go"}}, rangeValue: ui.RangeSliderState{Low: 20, High: 80}, rating: 3}
	}
	s := d.uiControls
	if s.edit.Value == "" {
		s.edit.Value = "report.txt"
	}
	switch d.ID {
	case "go-to-line":
		if s.editor.Text == nil {
			s.editor.Text = ui.NewTextBuffer("package main\n\nfunc main() {\n    println(\"OmniGo\")\n}\n")
		}
		return ui.Column("goto-demo", ui.GoToLine("goto", &s.gotoLine, s.editor.Text, func(line int) {
			d.Message = fmt.Sprintf("Caret at line %d", line)
			s.editor.Scroll.Y = float64(line-1) * 21
		}), ui.CodeEditor("goto-editor", &s.editor, ui.CodeEditorOptions{Language: "go", Height: 180}))
	case "completion-menu":
		if s.completionDoc == nil {
			s.completionDoc = ui.NewTextBuffer("fmt.Pr")
			s.completion.Begin(s.completionDoc, 4, 6)
		}
		items := []ui.CompletionItem{{ID: "print", Label: "Print", Detail: "fmt · function", InsertText: "Print()"}, {ID: "printf", Label: "Printf", Detail: "fmt · format", InsertText: "Printf(\"%s\", value)"}, {ID: "println", Label: "Println", Detail: "fmt · newline", InsertText: "Println()"}, {ID: "disabled", Label: "Private helper", Disabled: true}}
		return ui.Column("completion-demo", ui.Input("completion-source", "Source", s.completionDoc), ui.Button("completion-reset", "Reset example", func() { s.completionDoc.Set("fmt.Pr"); s.completion.Begin(s.completionDoc, 4, 6) }).Variant(ui.Outline), ui.CompletionMenu("completion", &s.completion, s.completionDoc, items, func(item ui.CompletionItem) { d.Message = "Inserted " + item.Label }))
	case "diff-view":
		if s.diff == nil {
			s.diff = &ui.DiffState{Old: "package main\n\nfunc greeting() string {\n    return \"Hello, golet\"\n}\n\n// Legacy implementation\n", New: "package main\n\nfunc greeting() string {\n    return \"Hello, OmniGo\"\n}\n\n// Native Go controls\n// Shared light and dark themes\n"}
		}
		return ui.DiffView("diff", s.diff, 280, func(string) { d.Message = "Change reverted; Undo restores it" })

	case "virtual-list", "grid-view":
		options := ui.VirtualListOptions{Count: 1000000, RowHeight: 64, Height: 280, Overscan: 2, Label: func(i int) string { return fmt.Sprintf("Record %06d", i+1) }, OnSelect: func(i int) { d.Message = fmt.Sprintf("Selected record %d", i+1) }}
		item := func(id string, i int) *ui.Element {
			return ui.Text(id, fmt.Sprintf("Row data %d", i+1)).Muted(true).FontSize(12)
		}
		if d.ID == "grid-view" {
			options.RowHeight = 88
			return ui.GridView("virtual", &s.virtual, ui.GridViewOptions{VirtualListOptions: options, Width: 600, MinWidth: 180, Gap: 8}, item)
		}
		return ui.VirtualList("virtual", &s.virtual, options, item)
	case "code-editor":
		if s.editor.Text == nil {
			s.editor.Text = ui.NewTextBuffer("package main\n\nimport \"fmt\"\n\nfunc main() {\n    // Native Go editor\n    fmt.Println(\"Hello, OmniGo\")\n}\n")
		}
		return ui.CodeEditor("editor", &s.editor, ui.CodeEditorOptions{Language: "go", Height: 260, OnChange: func(string) { d.Message = "Edited Go source" }})
	case "richtext-editor":
		if s.rich == nil {
			s.rich = ui.NewRichEditor("Select some text, then apply bold or italic. Formatting and text share undo history.")
		}
		return ui.RichTextEditor("rich-editor", s.rich, func([]ui.TextSpan) { d.Message = "Document changed" })
	case "timeline":
		if s.timeline.Duration == 0 {
			s.timeline = ui.AnimationTimeline{Duration: 2 * time.Second, Repeat: -1, Alternate: true}
		}
		return ui.Column("timeline-demo", ui.Row("timeline-actions", ui.Button("timeline/pause", "Pause / resume", func() { s.timeline.Paused = !s.timeline.Paused }).Variant(ui.Outline), ui.Button("timeline/restart", "Restart", func() { s.timeline.Restart() }).Variant(ui.Ghost)), ui.Timeline("timeline", &s.timeline, func(p *ui.Painter, r ui.Rect, progress float64) {
			p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H/2 - 4, W: r.W, H: 8}, p.Theme().Muted, 4)
			p.Fill(ui.Rect{X: r.X + progress*max(0, r.W-32), Y: r.Y + r.H/2 - 16, W: 32, H: 32}, p.Theme().Accent, 16)
		}))
	case "video":
		if s.video == nil {
			frames := make([]ui.VideoFrame, 12)
			for i := range frames {
				frame := image.NewNRGBA(image.Rect(0, 0, 240, 120))
				for y := 0; y < 120; y++ {
					for x := 0; x < 240; x++ {
						c := color.NRGBA{R: 24, G: 40, B: 34, A: 255}
						if x >= i*17 && x < i*17+32 && y > 35 && y < 85 {
							c = color.NRGBA{R: 90, G: 200, B: 140, A: 255}
						}
						frame.SetNRGBA(x, y, c)
					}
				}
				frames[i] = ui.VideoFrame{Image: frame, Duration: 100 * time.Millisecond}
			}
			var err error
			s.video, err = ui.NewVideoPlayer(frames)
			if err != nil {
				panic(err)
			}
			s.video.Loop(true)
		}
		return ui.Column("video-demo", ui.Video("video", s.video), ui.Row("video-actions", ui.Button("video/pause", "Pause / resume", func() { s.video.Pause(!s.video.Paused()) }).Variant(ui.Outline), ui.Button("video/restart", "Restart", func() { s.video.Seek(0) }).Variant(ui.Ghost)), ui.Text("video/caption", "Decoded frame playback · explicit clock · no audio").Muted(true))

	case "decimal-input":
		if s.decimal.Value == "" {
			s.decimal.Value = "1234.50"
		}
		return ui.Column("decimal-demo", ui.Field("amount-field", "Amount", ui.DecimalInput("amount", "Amount", &s.decimal, ui.DecimalInputOptions{Min: "0", Step: "0.01", OnChange: func(v string) { d.Message = "Exact amount: " + v }}), "Exact decimal arithmetic · Up / Down to step", s.decimal.Error), ui.Text("decimal/value", "Committed: "+s.decimal.Value).Muted(true))
	case "outline", "outline-table":
		if s.outline.Open == nil {
			s.outline.Open = map[string]bool{"Engineering": true}
		}
		children := map[string][]string{"Engineering": {"Platform", "Applications", "Archived"}, "Platform": {"Runtime", "Rendering"}, "Design": {"Components", "Icons"}}
		options := ui.OutlineOptions[string]{Incremental: true, ID: func(k string) string { return k }, Label: func(k string) string { return k }, Children: func(k string) []string { return children[k] }, Disabled: func(k string) bool { return k == "Archived" }, OnSelect: func(k string) { d.Message = "Selected: " + k }, OnActivate: func(k string) { d.Message = "Opened: " + k }}
		roots := []string{"Engineering", "Design", "Documentation"}
		if d.ID == "outline" {
			return ui.TreeOutline("outline", &s.outline, roots, options)
		}
		return ui.OutlineTable("outline", &s.outline, roots, options, []ui.OutlineColumn[string]{{Title: "Team"}, {Title: "Items", Width: 70, Cell: func(id, k string) *ui.Element { return ui.Text(id, fmt.Sprint(len(children[k]))) }}, {Title: "Status", Width: 90, Cell: func(id, k string) *ui.Element {
			if k == "Archived" {
				return ui.Badge(id, "Archived")
			}
			return ui.Badge(id, "Active")
		}}})
	case "image":
		bitmap := image.NewNRGBA(image.Rect(0, 0, 120, 80))
		for y := 0; y < 80; y++ {
			for x := 0; x < 120; x++ {
				bitmap.SetNRGBA(x, y, color.NRGBA{uint8(x * 2), uint8(y * 3), 160, 255})
			}
		}
		return ui.Image("bitmap", "Generated color gradient", bitmap, ui.ImageOptions{Width: 300, Height: 180, Fit: ui.ImageContain})
	case "grid":
		children := []*ui.Element{}
		for i := 0; i < 7; i++ {
			i := i
			children = append(children, ui.Button(fmt.Sprintf("cell/%d", i), fmt.Sprintf("Item %d", i+1), func() { d.Message = fmt.Sprintf("Selected %d", i+1) }).Variant(ui.Outline))
		}
		return ui.Grid("grid", 3, 12, children...)
	case "checkbox-group":
		return ui.CheckboxGroup("checks", "Notifications", d.Selected, []ui.Option{{ID: "mail", Label: "Mail"}, {ID: "calendar", Label: "Calendar"}, {ID: "locked", Label: "Locked", Disabled: true}}, func(v map[string]bool) { d.Selected = v })
	case "editable-text":
		return ui.EditableText("rename", "Filename", &s.edit, func(v string) { d.Message = "Renamed: " + v })
	case "find-bar":
		return ui.Column("find-demo", ui.Button("find/open", "Open find", func() { s.find.Open = true }), ui.FindBar("find", &s.find, ui.FindBarOptions{Matches: 3, OnSelect: func(i int) { d.Message = fmt.Sprintf("Match %d", i+1) }, OnQuery: func(v string) { d.Message = "Query callback: " + v }}))
	case "number-input":
		return ui.NumberInput("number", "Quantity", &s.number, ui.NumberInputOptions{Min: 0, Max: 100, Step: .25, OnChange: func(v float64) { d.Message = fmt.Sprint("Quantity: ", v) }})
	case "date-input":
		return ui.DateInput("date", "Date", &s.date, ui.DateInputOptions{WeekStart: time.Monday, OnChange: func(v time.Time) { d.Message = v.Format(time.RFC3339) }})
	case "time-input":
		return ui.TimeInput("time", "Time", &s.clock, ui.TimeInputOptions{TwelveHour: true, OnChange: func(v time.Time) { d.Message = v.Format("15:04") }})
	case "search-field":
		return ui.SearchField("search", "Search", d.Buffer, func(v string) { d.Message = "Search: " + v })
	case "rating":
		return ui.Rating("rating", "Rating", s.rating, 5, func(v int) { s.rating = v; d.Message = fmt.Sprintf("%d / 5", v) })
	case "range-slider":
		return ui.RangeSlider("range", "Price range", &s.rangeValue, ui.RangeSliderOptions{Min: 0, Max: 100, Step: 5, OnChange: func(lo, hi float64) { d.Message = fmt.Sprintf("%.0f – %.0f", lo, hi) }})
	case "token-field":
		return ui.TokenField("tokens", "Languages", &s.tokens, ui.TokenFieldOptions{Suggestions: []string{"Go", "Rust", "TypeScript", "Python"}, MaxTokens: 5, OnChange: func(v []string) { d.Message = fmt.Sprint(v) }})
	case "toolbar":
		return ui.Toolbar("toolbar", "Formatting", []ui.ToolbarItem{{ID: "bold", Label: "Bold", Toggle: true, Checked: s.bold, Action: func() { s.bold = !s.bold }}, {ID: "save", Label: "Save", Action: func() { d.Message = "Save callback" }}, {ID: "locked", Label: "Locked", Disabled: true}})
	case "color-picker":
		return ui.ColorPicker("color", "Color", &s.color, ui.ColorPickerOptions{OnChange: func(v color.NRGBA) { d.Message = fmt.Sprintf("RGBA %v", v) }})
	case "color-well":
		return ui.ColorWell("color", "Color", &s.color, ui.ColorPickerOptions{OnChange: func(v color.NRGBA) { d.Message = fmt.Sprintf("RGBA %v", v) }})
	}
	return nil
}
