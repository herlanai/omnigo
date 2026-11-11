package baseui

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

type KeyEvent struct {
	Key                        string
	Shift, Command, Repeat, Up bool
	// RequestFocus applies after the next Mount, so rebuilding a calendar month
	// or changing a panel can move focus to a not-yet-mounted control.
	RequestFocus func(string)
}
type PointerEvent struct {
	X, Y float64
	Kind string
} // down, move, up, cancel

type entry struct {
	element      *Element
	bounds, clip Rect
	disabled     bool
	modal        string
}
type Accessibility struct {
	ID, Role, Label, Description, Value                            string
	Bounds                                                         Rect
	Disabled, ReadOnly, Invalid, Focused, Checked, Mixed, Selected bool
}

// Surface owns a retained snapshot, layout and interaction state. Call its
// methods only on the owning UI goroutine. Platform drivers can use the
// accessibility snapshot and Render output without executing any browser code.
type Surface struct {
	width, height                             int
	theme                                     Theme
	face                                      font.Face
	ownedFace                                 bool
	root                                      *Element
	entries                                   []entry
	index                                     map[string]int
	focus, pressed, hover, modal, returnFocus string
	requestedFocus                            string
	closed                                    bool
	now                                       time.Duration
}

func NewSurface(width, height int, theme Theme) (*Surface, error) {
	return NewSurfaceWithFont(width, height, theme, nil)
}

// A supplied face is caller-owned. The default Go font supports Latin; callers
// must supply suitable fonts for CJK. Complex-script shaping is not implemented.
func NewSurfaceWithFont(width, height int, theme Theme, face font.Face) (*Surface, error) {
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || int64(width)*int64(height) > 64_000_000 {
		return nil, errors.New("baseui: invalid surface dimensions")
	}
	for _, n := range []float64{theme.Height, theme.FontSize, theme.Radius, theme.Spacing} {
		if !finite(n) || n < 0 {
			return nil, errors.New("baseui: invalid theme metrics")
		}
	}
	if theme.Height == 0 || theme.FontSize == 0 {
		return nil, errors.New("baseui: theme height and font size must be positive")
	}
	s := &Surface{width: width, height: height, theme: theme, face: face, index: make(map[string]int)}
	if face == nil {
		f, err := opentype.Parse(goregular.TTF)
		if err != nil {
			return nil, err
		}
		s.face, err = opentype.NewFace(f, &opentype.FaceOptions{Size: theme.FontSize, DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			return nil, err
		}
		s.ownedFace = true
	}
	return s, nil
}
func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }
func (s *Surface) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	s.root = nil
	s.entries = nil
	clear(s.index)
	if s.ownedFace {
		return s.face.Close()
	}
	return nil
}
func (s *Surface) Mount(root *Element) error {
	if s.closed {
		return errors.New("baseui: closed surface")
	}
	if root == nil {
		return errors.New("baseui: root is nil")
	}
	seen := make(map[string]bool)
	count := 0
	var copyTree func(*Element, int) (*Element, error)
	copyTree = func(e *Element, depth int) (*Element, error) {
		count++
		if depth > 128 || count > 100000 {
			return nil, errors.New("baseui: tree budget exceeded")
		}
		if e == nil || e.id == "" || seen[e.id] {
			return nil, errors.New("baseui: nil element or duplicate/empty ID")
		}
		seen[e.id] = true
		if e.kind > CustomKind || e.variant > Destructive {
			return nil, errors.New("baseui: invalid control kind or variant")
		}
		for _, n := range []float64{e.width, e.height, e.grow, e.gap, e.padding} {
			if !finite(n) || n < 0 {
				return nil, fmt.Errorf("baseui: invalid geometry for %s", e.id)
			}
		}
		if (e.kind == InputKind || e.kind == TextareaKind) && e.buffer == nil {
			return nil, fmt.Errorf("baseui: %s needs a TextBuffer", e.id)
		}
		if e.kind == SliderKind {
			if _, err := Snap(e.value, e.min, e.max, e.step); err != nil {
				return nil, err
			}
		}
		if e.kind == ProgressKind && (!finite(e.value) || e.value < 0 || e.value > 1) {
			return nil, errors.New("baseui: progress must be in [0,1]")
		}
		out := *e
		out.children = make([]*Element, len(e.children))
		for i, c := range e.children {
			child, err := copyTree(c, depth+1)
			if err != nil {
				return nil, err
			}
			out.children[i] = child
		}
		return &out, nil
	}
	cloned, err := copyTree(root, 0)
	if err != nil {
		return err
	}
	previousModal := s.modal
	s.root = cloned
	s.entries = s.entries[:0]
	clear(s.index)
	s.modal = ""
	bounds := Rect{W: float64(s.width), H: float64(s.height)}
	if cloned.width > 0 {
		bounds.W = min(bounds.W, cloned.width)
	}
	if cloned.height > 0 {
		bounds.H = min(bounds.H, cloned.height)
	}
	s.layout(cloned, bounds, Rect{W: float64(s.width), H: float64(s.height)}, false, "")
	if previousModal != s.modal {
		if s.modal != "" {
			s.returnFocus = s.focus
			s.focus = ""
			s.focusNext(false)
		} else {
			s.focus = s.returnFocus
			s.returnFocus = ""
		}
	}
	if _, ok := s.enabled(s.focus); !ok {
		s.focus = ""
	}
	if _, ok := s.enabled(s.pressed); !ok {
		s.pressed = ""
	}
	if s.requestedFocus != "" {
		s.Focus(s.requestedFocus)
		s.requestedFocus = ""
	}
	return nil
}

func (s *Surface) lineHeight() float64 { return float64(s.face.Metrics().Height.Ceil()) + 4 }
func (s *Surface) textWidth(text string) float64 {
	return float64(font.MeasureString(s.face, text).Ceil())
}
func (s *Surface) measure(e *Element, width float64) (float64, float64) {
	if e.hidden {
		return 0, 0
	}
	w, h := e.width, e.height
	switch e.kind {
	case ColumnKind, CardKind, ModalKind, RowKind:
		var cw, ch float64
		visible := 0
		for _, child := range e.children {
			if child.hidden || child.kind == ModalKind {
				continue
			}
			a, b := s.measure(child, max(0, width-2*e.padding))
			visible++
			if e.kind == RowKind {
				cw += a
				ch = max(ch, b)
			} else {
				cw = max(cw, a)
				ch += b
			}
		}
		if e.kind == RowKind {
			cw += float64(max(0, visible-1)) * e.gap
		} else {
			ch += float64(max(0, visible-1)) * e.gap
		}
		if w == 0 {
			w = cw + 2*e.padding
		}
		if h == 0 {
			h = ch + 2*e.padding
		}
	case TextKind:
		if w == 0 {
			for _, line := range strings.Split(e.text, "\n") {
				w = max(w, s.textWidth(line))
			}
		}
		if h == 0 {
			h = s.lineHeight() * float64(len(strings.Split(e.text, "\n")))
		}
	case ButtonKind, BadgeKind:
		if w == 0 {
			w = s.textWidth(e.text) + 24
		}
		if h == 0 {
			h = s.theme.Height
		}
	case CheckboxKind, RadioKind:
		if w == 0 {
			w = s.textWidth(e.text) + 32
		}
		if h == 0 {
			h = s.theme.Height
		}
	case SwitchKind:
		if w == 0 {
			w = s.textWidth(e.text) + 52
		}
		if h == 0 {
			h = s.theme.Height
		}
	case InputKind, TextareaKind, SliderKind:
		if w == 0 {
			w = 220
		}
		if h == 0 {
			h = s.theme.Height
		}
	default:
		if w == 0 {
			w = 100
		}
		if h == 0 {
			h = s.theme.Height
		}
	}
	return w, h
}

func (s *Surface) layout(e *Element, r, clip Rect, disabled bool, modal string) {
	if e.hidden {
		return
	}
	disabled = disabled || e.disabled
	if e.kind == ModalKind {
		modal = e.id
		s.modal = e.id
		w, h := s.measure(e, float64(s.width)-32)
		w = min(w, float64(s.width)-32)
		h = min(h, float64(s.height)-32)
		r = Rect{(float64(s.width) - w) / 2, (float64(s.height) - h) / 2, w, h}
		clip = Rect{W: float64(s.width), H: float64(s.height)}
	}
	entryClip := intersect(r, clip)
	s.index[e.id] = len(s.entries)
	s.entries = append(s.entries, entry{e, r, entryClip, disabled, modal})
	inner := r.inset(e.padding)
	childClip := intersect(inner, clip)
	if e.kind != ColumnKind && e.kind != RowKind && e.kind != CardKind && e.kind != ModalKind {
		return
	}
	var fixed, grow float64
	count := 0
	for _, c := range e.children {
		if c.hidden || c.kind == ModalKind {
			continue
		}
		w, h := s.measure(c, inner.W)
		if e.kind == RowKind {
			fixed += w
		} else {
			fixed += h
		}
		grow += c.grow
		count++
	}
	space := inner.H
	if e.kind == RowKind {
		space = inner.W
	}
	extra := max(0, space-fixed-float64(max(0, count-1))*e.gap)
	x, y := inner.X, inner.Y
	for _, c := range e.children {
		if c.hidden || c.kind == ModalKind {
			continue
		}
		w, h := s.measure(c, inner.W)
		bonus := 0.
		if grow > 0 {
			bonus = extra * c.grow / grow
		}
		if e.kind == RowKind {
			w += bonus
			if c.height == 0 {
				h = min(h, inner.H)
			}
		} else {
			h += bonus
			if c.width == 0 {
				w = inner.W
			}
		}
		s.layout(c, Rect{x, y, w, h}, childClip, disabled, modal)
		if e.kind == RowKind {
			x += w + e.gap
		} else {
			y += h + e.gap
		}
	}
	// Modals render and hit-test above ordinary children regardless of source order.
	for _, c := range e.children {
		if c.kind == ModalKind {
			s.layout(c, r, clip, disabled, modal)
		}
	}
}

func (s *Surface) Bounds(id string) (Rect, bool) {
	i, ok := s.index[id]
	if !ok {
		return Rect{}, false
	}
	return s.entries[i].bounds, true
}
func (s *Surface) Focused() string { return s.focus }
func (s *Surface) Focus(id string) bool {
	if e, ok := s.enabled(id); ok && interactive(e.element) {
		s.focus = id
		s.pressed = ""
		return true
	}
	return false
}
func (s *Surface) enabled(id string) (entry, bool) {
	i, ok := s.index[id]
	if !ok {
		return entry{}, false
	}
	e := s.entries[i]
	return e, !e.disabled && (s.modal == "" || e.modal == s.modal) && e.clip.W > 0 && e.clip.H > 0
}
func interactive(e *Element) bool {
	switch e.kind {
	case ButtonKind, CheckboxKind, SwitchKind, RadioKind, InputKind, TextareaKind, SliderKind:
		return true
	}
	return e.onKey != nil
}
func (s *Surface) focusNext(back bool) {
	ids := []string{}
	for _, e := range s.entries {
		if _, ok := s.enabled(e.element.id); ok && interactive(e.element) {
			ids = append(ids, e.element.id)
		}
	}
	if len(ids) == 0 {
		return
	}
	at := -1
	for i, id := range ids {
		if id == s.focus {
			at = i
			break
		}
	}
	if back {
		if at < 0 {
			at = 0
		}
		at = (at - 1 + len(ids)) % len(ids)
	} else {
		at = (at + 1) % len(ids)
	}
	s.focus = ids[at]
	s.pressed = ""
}
func (s *Surface) hit(x, y float64) string {
	for i := len(s.entries) - 1; i >= 0; i-- {
		e := s.entries[i]
		if e.clip.Contains(x, y) && interactive(e.element) {
			if _, ok := s.enabled(e.element.id); ok {
				return e.element.id
			}
		}
	}
	return ""
}
func guarded(action func()) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("baseui: callback panic: %v", value)
		}
	}()
	if action != nil {
		action()
	}
	return
}
func (s *Surface) Pointer(ev PointerEvent) error {
	if s.closed {
		return errors.New("baseui: closed surface")
	}
	if !finite(ev.X) || !finite(ev.Y) {
		return errors.New("baseui: invalid pointer position")
	}
	target := s.hit(ev.X, ev.Y)
	switch ev.Kind {
	case "down":
		s.pressed = target
		if target != "" {
			s.focus = target
		}
		return s.drag(ev.X, ev.Y, false)
	case "move":
		s.hover = target
		return s.drag(ev.X, ev.Y, true)
	case "cancel":
		s.pressed = ""
	case "up":
		pressed := s.pressed
		s.pressed = ""
		if pressed != "" && pressed == target {
			e, ok := s.enabled(pressed)
			if ok && !e.element.readOnly && e.element.kind != SliderKind {
				return guarded(e.element.activate)
			}
		}
	default:
		return errors.New("baseui: unknown pointer event")
	}
	return nil
}
func (s *Surface) drag(x, y float64, extend bool) error {
	e, ok := s.enabled(s.pressed)
	if !ok {
		return nil
	}
	if e.element.buffer != nil {
		b := e.element.buffer
		lines := strings.Split(b.text, "\n")
		row := 0
		if e.element.kind == TextareaKind {
			row = max(0, min(len(lines)-1, int((y-e.bounds.Y-8)/s.lineHeight())))
		}
		offset := 0
		for i := 0; i < row; i++ {
			offset += len([]rune(lines[i])) + 1
		}
		local := x - e.bounds.X - 10
		if e.element.kind == InputKind {
			local += max(0, s.textWidth(string([]rune(b.text)[:b.caret]))-(e.bounds.W-24))
		}
		runes := []rune(lines[row])
		column := len(runes)
		for i := range runes {
			if local < (s.textWidth(string(runes[:i]))+s.textWidth(string(runes[:i+1])))/2 {
				column = i
				break
			}
		}
		caret := offset + column
		anchor := caret
		if extend {
			anchor = b.anchor
		}
		b.Select(anchor, caret)
		return nil
	}
	if e.element.kind != SliderKind || e.element.readOnly {
		return nil
	}
	v := e.element
	portion := max(0, min(1, (x-e.bounds.X-10)/max(1, e.bounds.W-20)))
	number, _ := Snap(v.min+portion*(v.max-v.min), v.min, v.max, v.step)
	if number == v.value {
		return nil
	}
	v.value = number
	return guarded(func() {
		if v.changeNumber != nil {
			v.changeNumber(number)
		}
	})
}
func (s *Surface) Key(ev KeyEvent) error {
	if s.closed {
		return errors.New("baseui: closed surface")
	}
	if ev.Key == "Tab" && !ev.Up {
		s.focusNext(ev.Shift)
		return nil
	}
	if ev.Key == "Escape" && !ev.Up && s.modal != "" {
		return guarded(s.entries[s.index[s.modal]].element.activate)
	}
	ev.RequestFocus = func(id string) { s.requestedFocus = id }
	e, ok := s.enabled(s.focus)
	if !ok {
		return nil
	}
	v := e.element
	if ev.Up {
		if ev.Key == "Space" && s.pressed == s.focus {
			s.pressed = ""
			if !v.readOnly {
				return guarded(v.activate)
			}
		}
		return nil
	}
	if v.onKey != nil {
		handled := false
		if err := guarded(func() { handled = v.onKey(ev) }); err != nil {
			return err
		}
		if handled {
			return nil
		}
	}
	if v.buffer != nil {
		return guarded(func() {
			handled := v.buffer.key(ev, v.kind == TextareaKind, v.readOnly)
			if !handled && ev.Key == "Enter" && !ev.Repeat && v.onSubmit != nil && !v.readOnly {
				v.onSubmit()
			}
		})
	}
	if v.kind == SliderKind && !v.readOnly {
		number := v.value
		switch ev.Key {
		case "Left", "Down":
			number -= v.step
		case "Right", "Up":
			number += v.step
		case "Home":
			number = v.min
		case "End":
			number = v.max
		default:
			return nil
		}
		number, _ = Snap(number, v.min, v.max, v.step)
		if number != v.value {
			v.value = number
			return guarded(func() {
				if v.changeNumber != nil {
					v.changeNumber(number)
				}
			})
		}
		return nil
	}
	if !ev.Repeat && !v.readOnly {
		switch ev.Key {
		case "Enter":
			return guarded(v.activate)
		case "Space":
			s.pressed = s.focus
		}
	}
	return nil
}
func (s *Surface) TextInput(text string) error {
	e, ok := s.enabled(s.focus)
	if !ok || e.element.readOnly || e.element.buffer == nil {
		return nil
	}
	if e.element.kind == InputKind {
		text = strings.ReplaceAll(strings.ReplaceAll(text, "\n", ""), "\r", "")
	}
	return guarded(func() { e.element.buffer.Replace(text) })
}
func (s *Surface) SetComposition(text string) {
	e, ok := s.enabled(s.focus)
	if ok && !e.element.readOnly && e.element.buffer != nil {
		e.element.buffer.SetComposition(text)
	}
}
func (s *Surface) CommitComposition() error {
	e, ok := s.enabled(s.focus)
	if !ok || e.element.readOnly || e.element.buffer == nil {
		return nil
	}
	return guarded(e.element.buffer.CommitComposition)
}
func (s *Surface) Accessibility() []Accessibility {
	out := make([]Accessibility, 0, len(s.entries))
	for _, item := range s.entries {
		e := item.element
		if e.role == "" {
			continue
		}
		value := ""
		if e.buffer != nil {
			value = e.buffer.String()
		}
		out = append(out, Accessibility{ID: e.id, Role: e.role, Label: e.label, Description: e.description, Value: value, Bounds: item.bounds, Disabled: item.disabled || s.modal != "" && item.modal != s.modal, ReadOnly: e.readOnly, Invalid: e.invalid, Focused: s.focus == e.id, Checked: e.checked, Mixed: e.mixed, Selected: e.selected})
	}
	return out
}
func (s *Surface) Advance(now time.Duration) error {
	if now < s.now {
		return errors.New("baseui: animation time cannot move backwards")
	}
	s.now = now
	return nil
}
func (s *Surface) NeedsAnimation() bool {
	if s.theme.ReduceMotion {
		return false
	}
	for _, e := range s.entries {
		if (e.element.kind == SpinnerKind || e.element.animated) && e.clip.W > 0 && e.clip.H > 0 {
			return true
		}
	}
	return false
}

func Snap(value, min, max, step float64) (float64, error) {
	if !finite(value) || !finite(min) || !finite(max) || !finite(step) || max < min || step <= 0 {
		return 0, errors.New("baseui: invalid numeric range")
	}
	value = math.Max(min, math.Min(max, value))
	number := min + math.Round((value-min)/step)*step
	if !finite(number) {
		return 0, errors.New("baseui: numeric range overflow")
	}
	return math.Max(min, math.Min(max, number)), nil
}
