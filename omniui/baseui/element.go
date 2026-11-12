// Package baseui implements Go-owned controls, layout, input and CPU rendering.
// It is the native component layer of OmniUI; it does not execute HTML, CSS,
// JavaScript or a WebView. A Surface is confined to its UI goroutine.
package baseui

import (
	"image/color"
	"strconv"
	"time"

	"omnigo.dev/omnigo/omniui/baseui/theme"
)

type Kind uint8

const (
	ColumnKind Kind = iota
	RowKind
	TextKind
	ButtonKind
	CheckboxKind
	SwitchKind
	InputKind
	TextareaKind
	SliderKind
	ProgressKind
	SeparatorKind
	BadgeKind
	CardKind
	RadioKind
	ModalKind
	SpinnerKind
	CustomKind
	ScrollKind
	PopupKind
	AspectKind
	SplitKind
)

type Variant uint8

const (
	Primary Variant = iota
	Outline
	Secondary
	Ghost
	Destructive
	Link
)

type Rect struct{ X, Y, W, H float64 }

func (r Rect) Contains(x, y float64) bool { return x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H }
func (r Rect) inset(n float64) Rect       { return Rect{r.X + n, r.Y + n, max(0, r.W-2*n), max(0, r.H-2*n)} }
func intersect(a, b Rect) Rect {
	x, y := max(a.X, b.X), max(a.Y, b.Y)
	return Rect{x, y, max(0, min(a.X+a.W, b.X+b.W)-x), max(0, min(a.Y+a.H, b.Y+b.H)-y)}
}

type Theme struct {
	Chart                                                                                                    [5]color.NRGBA
	Background, Text, Surface, Muted, MutedText, Border, Accent, AccentText, Hover, Danger, Focus, Selection color.NRGBA
	Height, Radius, Spacing, FontSize                                                                        float64
	ReduceMotion                                                                                             bool
}

func NewTheme(dark bool) Theme {
	t := theme.Default()
	p := t.Light
	if dark {
		p = t.Dark
	}
	c := func(key string) color.NRGBA {
		n, err := strconv.ParseUint(p[key][1:], 16, 32)
		if err != nil {
			panic(err)
		}
		return color.NRGBA{uint8(n >> 16), uint8(n >> 8), uint8(n), 255}
	}
	return Theme{Chart: [5]color.NRGBA{c("chart1"), c("chart2"), c("chart3"), c("chart4"), c("chart5")}, Background: c("background"), Text: c("foreground"), Surface: c("surface"), Muted: c("muted"), MutedText: c("mutedForeground"), Border: c("border"), Accent: c("primary"), AccentText: c("primaryForeground"), Hover: c("hover"), Danger: c("danger"), Focus: c("focus"), Selection: c("selection"), Height: t.ControlHeight, Radius: t.Radius, Spacing: t.Spacing, FontSize: t.FontSize}
}

// Element is a build description. Mount snapshots it before handling input,
// so changes to caller-owned builders cannot alter the displayed frame.
type Element struct {
	inlineLink                            bool
	nativeView                            *WebViewState
	monospace, captureTab                 bool
	codeRows                              [][]codeRun
	onFrame                               func(time.Duration, bool, bool)
	animating                             func() bool
	fontSize, controlPadding              float64
	minHeight                             float64
	fontWeight                            int
	mutedText                             bool
	radius                                *float64
	iconSource                            IconSource
	iconOptions                           IconOptions
	measureContent                        func(float64) (float64, float64)
	inputGroup, groupedInput, centerItems bool
	paddingX                              float64
	onHover                               func(PointerEvent, Rect)
	detail                                string
	submenu                               bool
	flat                                  bool
	notifications                         *ToastManager
	toast                                 *toastRecord
	onPointer                             func(PointerEvent, Rect, Rect)
	drawer                                *DrawerState
	wrap                                  bool
	branch                                bool
	followTail                            bool

	skipTab bool

	textIndent float64
	treeLevel  int

	onContext      func(float64, float64, bool)
	pointAnchor    bool
	pointX, pointY float64

	hoverOptions *hoverOptions
	passive      bool
	otpLength    int

	expanded bool
	controls string

	sheet     bool
	side      Side
	ratio     float64
	split     *SplitState
	vertical  bool
	direction LayoutDirection

	onChange                     func(string)
	onText                       func(string, time.Duration) string
	alignStart, allowAnchorFocus bool
	revealChild                  string

	scroll            *ScrollState
	horizontal        bool
	anchor, autoFocus string
	placement         Placement
	dismissOutside    bool

	id, label, description, text, placeholder, focusGroup         string
	kind                                                          Kind
	role                                                          string
	children                                                      []*Element
	width, height, grow, gap, padding                             float64
	disabled, readOnly, invalid, hidden, checked, mixed, selected bool
	variant                                                       Variant
	value, min, max, step                                         float64
	activate                                                      func()
	changeNumber                                                  func(float64)
	buffer                                                        *TextBuffer
	paint                                                         func(*Painter, Rect)
	onKey                                                         func(KeyEvent) bool
	onSubmit                                                      func()
	animated                                                      bool
}

func element(id string, k Kind) *Element { return &Element{id: id, kind: k, gap: 8} }

// Wrap enables plain-text line wrapping; it does not perform rich text shaping.
func (e *Element) Wrap(v bool) *Element         { e.wrap = v; return e }
func (e *Element) Width(n float64) *Element     { e.width = n; return e }
func (e *Element) Height(n float64) *Element    { e.height = n; return e }
func (e *Element) Grow(n float64) *Element      { e.grow = n; return e }
func (e *Element) MinHeight(n float64) *Element { e.minHeight = n; return e }

func (e *Element) Gap(n float64) *Element     { e.gap = n; return e }
func (e *Element) Padding(n float64) *Element { e.padding = n; return e }
func (e *Element) Disabled(v bool) *Element   { e.disabled = v; return e }
func (e *Element) ReadOnly(v bool) *Element   { e.readOnly = v; return e }
func (e *Element) Invalid(v bool) *Element    { e.invalid = v; return e }
func (e *Element) Hidden(v bool) *Element     { e.hidden = v; return e }

// Placeholder displays a hint only while an input is empty; it is not its accessible label.
func (e *Element) Placeholder(v string) *Element         { e.placeholder = v; return e }
func (e *Element) Label(v string) *Element               { e.label = v; return e }
func (e *Element) Description(v string) *Element         { e.description = v; return e }
func (e *Element) Variant(v Variant) *Element            { e.variant = v; return e }
func (e *Element) OnKey(fn func(KeyEvent) bool) *Element { e.onKey = fn; return e }

// OnTextInput receives committed text on focused custom controls. Platform IME
// preedit is not forwarded; custom composition rendering is not yet supported.
func (e *Element) OnTextInput(fn func(string)) *Element {
	if fn == nil {
		e.onText = nil
	} else {
		e.onText = func(value string, _ time.Duration) string { fn(value); return "" }
	}
	return e
}
func (e *Element) OnSubmit(fn func()) *Element { e.onSubmit = fn; return e }
func (e *Element) Animate(v bool) *Element     { e.animated = v; return e }

// PaintSurface draws after this element's own surface and before its children.
// The callback runs on the UI goroutine and must not retain Painter.
func (e *Element) PaintSurface(fn func(*Painter, Rect)) *Element { e.paint = fn; return e }

func (e *Element) Role(role string) *Element { e.role = role; return e }

func Column(id string, children ...*Element) *Element {
	e := element(id, ColumnKind)
	e.children = children
	return e
}
func Row(id string, children ...*Element) *Element {
	e := element(id, RowKind)
	e.children = children
	return e
}
func Card(id string, children ...*Element) *Element {
	e := Column(id, children...)
	e.kind = CardKind
	e.padding = 24
	e.gap = 24
	return e
}
func Text(id, text string) *Element {
	e := element(id, TextKind)
	e.text = text
	e.label = text
	e.role = "text"
	return e
}
func Button(id, label string, action func()) *Element {
	e := element(id, ButtonKind)
	e.label = label
	e.text = label
	e.role = "button"
	e.activate = action
	return e
}
func Checkbox(id, label string, checked bool, change func(bool)) *Element {
	e := Button(id, label, func() {
		if change != nil {
			change(!checked)
		}
	})
	e.kind = CheckboxKind
	e.role = "checkbox"
	e.checked = checked
	return e
}
func MixedCheckbox(id, label string, state CheckState, change func(CheckState)) *Element {
	e := Checkbox(id, label, state == Checked, func(checked bool) {
		if change != nil {
			if checked {
				change(Checked)
			} else {
				change(Unchecked)
			}
		}
	})
	e.mixed = state == Mixed
	return e
}
func Switch(id, label string, checked bool, change func(bool)) *Element {
	e := Checkbox(id, label, checked, change)
	e.kind = SwitchKind
	e.role = "switch"
	return e
}
func Radio(id, label string, checked bool, action func()) *Element {
	e := Button(id, label, action)
	e.kind = RadioKind
	e.role = "radio"
	e.checked = checked
	return e
}
func Toggle(id, label string, pressed bool, change func(bool)) *Element {
	e := Checkbox(id, label, pressed, change)
	e.kind = ButtonKind
	e.role = "togglebutton"
	e.selected = pressed
	return e.Variant(Outline)
}
func Input(id, label string, buffer *TextBuffer) *Element {
	e := element(id, InputKind)
	e.label = label
	e.role = "textbox"
	e.buffer = buffer
	return e
}
func Textarea(id, label string, buffer *TextBuffer) *Element {
	e := Input(id, label, buffer)
	e.kind = TextareaKind
	e.height = 96
	return e
}
func Slider(id, label string, value, min, max, step float64, change func(float64)) *Element {
	e := element(id, SliderKind)
	e.role = "slider"
	e.label = label
	e.value = value
	e.min = min
	e.max = max
	e.step = step
	e.changeNumber = change
	return e
}
func Progress(id, label string, value float64) *Element {
	e := element(id, ProgressKind)
	e.role = "progressbar"
	e.label = label
	e.value = value
	e.min = 0
	e.max = 1
	e.height = 10
	return e
}
func Separator(id string, vertical ...bool) *Element {
	e := element(id, SeparatorKind)
	e.role = "separator"
	e.height = 1
	if len(vertical) > 0 && vertical[0] {
		e.vertical = true
		e.width = 1
		e.height = 0
	}
	return e
}
func Badge(id, text string) *Element {
	e := Text(id, text)
	e.kind = BadgeKind
	e.height = 20
	e.fontSize = 12
	e.controlPadding = 8
	return e
}
func Spinner(id, label string) *Element {
	e := element(id, SpinnerKind)
	e.role = "progressbar"
	e.label = label
	e.width = 24
	e.height = 24
	return e
}
func Canvas(id, label string, paint func(*Painter, Rect)) *Element {
	e := element(id, CustomKind)
	e.label = label
	e.role = "img"
	e.paint = paint
	return e
}

type CheckState uint8

const (
	Unchecked CheckState = iota
	Checked
	Mixed
)
