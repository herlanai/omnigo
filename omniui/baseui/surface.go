package baseui

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"golang.org/x/image/font"
)

type KeyEvent struct {
	Ctrl, Alt, Super           bool // Physical modifiers; Command retains primary-shortcut compatibility.
	Key                        string
	Shift, Command, Repeat, Up bool
	RTL                        bool
	// RequestFocus applies after the next Mount, so rebuilding a calendar month
	// or changing a panel can move focus to a not-yet-mounted control.
	RequestFocus func(string)
}
type PointerEvent struct {
	Button int // 0 primary, 1 secondary; other buttons are ignored.
	X, Y   float64
	Kind   string
} // down, move, up, cancel

type entry struct {
	element      *Element
	bounds, clip Rect
	paintClip    Rect
	disabled     bool
	modal        string
	parent       string
	rtl          bool
}
type Accessibility struct {
	Level       int
	DescribedBy string

	Expanded, HasPopup bool
	Controls           string

	ID, Role, Label, Description, Value                            string
	Bounds                                                         Rect
	Disabled, ReadOnly, Invalid, Focused, Checked, Mixed, Selected bool
}

// Surface owns a retained snapshot, layout and interaction state. Call its
// methods only on the owning UI goroutine. Platform drivers can use the
// accessibility snapshot and Render output without executing any browser code.
type Surface struct {
	nativeViews          map[*WebViewState]bool
	textFaces            map[textFaceKey]font.Face
	rasterFaces          map[textFaceKey]font.Face
	rasterScale          float64
	focusVisible         bool
	hoverListener        string
	inactive             bool
	notificationManagers map[*ToastManager]bool
	hoverStates          map[string]*hoverRuntime
	hoverCandidates      []*Element
	syncingHover         bool
	pointerKnown         bool
	pointerX, pointerY   float64

	splitDrag  string
	splitStart float64

	overlays         []deferredOverlay
	layers           []string
	focusLayers      []focusLayer
	placing          string
	scrollDrag       string
	scrollHorizontal bool
	scrollGrab       float64

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

// A supplied face is caller-owned. Default faces use Inter with lazy system
// font fallback where available. Complex-script shaping is not implemented.
func NewSurfaceWithFont(width, height int, theme Theme, face font.Face) (*Surface, error) {
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || int64(width)*int64(height) > 64_000_000 {
		return nil, errors.New("baseui: invalid surface dimensions")
	}
	for _, n := range []float64{theme.Height, theme.FontSize, theme.Radius, theme.Spacing} {
		if !finite(n) || n < 0 {
			return nil, errors.New("baseui: invalid theme metrics")
		}
	}
	if theme.Style > StyleLyra {
		return nil, errors.New("baseui: invalid visual style")
	}
	if theme.Height == 0 || theme.FontSize == 0 {
		return nil, errors.New("baseui: theme height and font size must be positive")
	}
	s := &Surface{width: width, height: height, theme: theme, face: face, index: make(map[string]int)}
	if face == nil {
		s.face = richFace(0, theme.FontSize)
		s.ownedFace = true
	}
	return s, nil
}
func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }
func (s *Surface) Close() error {
	if s.closed {
		return nil
	}
	if item, ok := s.enabled(s.pressed); ok && item.element.onPointer != nil {
		_ = guarded(func() {
			item.element.onPointer(PointerEvent{Kind: "cancel"}, item.bounds, Rect{W: float64(s.width), H: float64(s.height)})
		})
	}
	for m := range s.notificationManagers {
		m.initialized = false
		m.interacting = false
	}
	s.notificationManagers = nil
	_ = s.notifyPointerHover("", PointerEvent{Kind: "cancel"})
	for _, item := range s.entries {
		if item.element.onFrame != nil {
			item.element.onFrame(s.now, false, s.theme.ReduceMotion)
		}
	}
	for view := range s.nativeViews {
		view.view.Place(Rect{}, Rect{}, false, false)
	}
	s.nativeViews = nil
	s.closed = true
	s.root = nil
	s.hoverStates = nil
	s.hoverCandidates = nil
	s.pointerKnown = false
	s.entries = nil
	clear(s.index)
	s.overlays = nil
	s.focusLayers = nil
	s.layers = nil
	s.scrollDrag = ""
	s.splitDrag = ""
	for _, face := range s.textFaces {
		_ = face.Close()
	}
	s.textFaces = nil
	s.clearRasterFaces()
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
	var copyTree func(*Element, int, bool) (*Element, error)
	copyTree = func(e *Element, depth int, readOnly bool) (*Element, error) {
		count++
		if depth > 128 || count > 100000 {
			return nil, errors.New("baseui: tree budget exceeded")
		}
		if e == nil || e.id == "" || seen[e.id] {
			return nil, errors.New("baseui: nil element or duplicate/empty ID")
		}
		seen[e.id] = true
		if e.hoverOptions != nil && (e.hoverOptions.show < 0 || e.hoverOptions.hide < 0) {
			return nil, errors.New("baseui: invalid hover delay")
		}
		if e.otpLength < 0 || e.otpLength > 32 {
			return nil, errors.New("baseui: invalid OTP length")
		}
		if e.otpLength > 0 && (e.buffer == nil || len(e.buffer.String()) > e.otpLength || digits(e.buffer.String()) != e.buffer.String()) {
			return nil, errors.New("baseui: invalid OTP content")
		}
		if e.otpLength > 0 {
			for _, history := range [][]textSnapshot{e.buffer.undo, e.buffer.redo} {
				for _, old := range history {
					text := old.document.String()
					if len(text) > e.otpLength || digits(text) != text {
						return nil, errors.New("baseui: OTP history contains invalid content")
					}
				}
			}
		}
		if e.kind > SplitKind || e.variant > Link {
			return nil, errors.New("baseui: invalid control kind or variant")
		}
		if e.side > Bottom || e.direction > RightToLeft {
			return nil, errors.New("baseui: invalid layout direction")
		}
		if e.kind == AspectKind && (!finite(e.ratio) || e.ratio <= 0 || len(e.children) != 1) {
			return nil, errors.New("baseui: invalid aspect ratio")
		}
		if e.kind == SplitKind {
			if e.split == nil || len(e.children) != 3 {
				return nil, errors.New("baseui: invalid split state")
			}
			for _, v := range []float64{e.split.Fraction, e.split.MinFirst, e.split.MinSecond} {
				if !finite(v) || v < 0 {
					return nil, errors.New("baseui: invalid split metric")
				}
			}
			if e.split.Fraction > 1 {
				return nil, errors.New("baseui: split fraction exceeds 1")
			}
		}
		if e.pointAnchor && (!finite(e.pointX) || !finite(e.pointY)) {
			return nil, errors.New("baseui: invalid popup position")
		}
		if e.placement > Beside {
			return nil, errors.New("baseui: invalid popup placement")
		}
		if e.kind == ScrollKind && (e.scroll == nil || !finite(e.scroll.X) || !finite(e.scroll.Y)) {
			return nil, errors.New("baseui: invalid scroll state")
		}
		for _, n := range []float64{e.width, e.height, e.minHeight, e.grow, e.gap, e.padding, e.textIndent, e.paddingX} {
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
		out.applyStyle(s.theme)
		if e.hoverOptions != nil {
			h := *e.hoverOptions
			out.hoverOptions = &h
		}
		out.readOnly = e.readOnly || readOnly
		out.children = make([]*Element, len(e.children))
		for i, c := range e.children {
			child, err := copyTree(c, depth+1, out.readOnly)
			if err != nil {
				return nil, err
			}
			out.children[i] = child
		}
		return &out, nil
	}
	cloned, err := copyTree(root, 0, false)
	if err != nil {
		return err
	}
	var validateAnchors func(*Element) error
	validateAnchors = func(e *Element) error {
		if e.kind == PopupKind && !e.hidden && (e.anchor == e.id || !seen[e.anchor]) {
			return fmt.Errorf("baseui: invalid popup anchor %q", e.anchor)
		}
		for _, c := range e.children {
			if err := validateAnchors(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := validateAnchors(cloned); err != nil {
		return err
	}
	defer s.syncHover()
	previous, hadPointer := s.enabled(s.pressed)
	previousHover, hadHover := s.enabled(s.hoverListener)
	oldFrames := []entry{}
	for _, item := range s.entries {
		if item.element.onFrame != nil {
			oldFrames = append(oldFrames, item)
		}
	}
	s.root = cloned
	s.relayout()
	for _, item := range oldFrames {
		if i, ok := s.index[item.element.id]; !ok || s.entries[i].element.onFrame == nil {
			item.element.onFrame(s.now, false, s.theme.ReduceMotion)
		}
	}
	if hadHover && previousHover.element.onHover != nil {
		current, ok := s.enabled(s.hoverListener)
		if !ok || current.element.readOnly || current.element.onHover == nil {
			s.hoverListener = ""
			if err := guarded(func() { previousHover.element.onHover(PointerEvent{Kind: "cancel"}, previousHover.bounds) }); err != nil {
				return err
			}
		}
	}
	if hadPointer && previous.element.onPointer != nil {
		current, ok := s.enabled(s.pressed)
		if !ok || current.element.readOnly || current.element.onPointer == nil {
			if err := guarded(func() {
				previous.element.onPointer(PointerEvent{Kind: "cancel"}, previous.bounds, Rect{W: float64(s.width), H: float64(s.height)})
			}); err != nil {
				return err
			}
			s.pressed = ""
			s.relayout()
		}
	}
	s.updateLayerFocus()
	reveal := []string{}
	for _, item := range s.entries {
		if item.element.revealChild != "" {
			reveal = append(reveal, item.element.revealChild)
		}
	}
	for _, id := range reveal {
		s.reveal(id)
	}

	if !s.focusEligible(s.focus) {
		s.focus = ""
	}
	if _, ok := s.enabled(s.pressed); !ok {
		s.pressed = ""
	}
	if i, ok := s.index[s.scrollDrag]; !ok || s.entries[i].element.scroll == nil || s.entries[i].disabled || s.entries[i].element.readOnly {
		s.scrollDrag = ""
	}
	if i, ok := s.index[s.splitDrag]; !ok || s.entries[i].element.split == nil || s.entries[i].disabled || s.entries[i].element.readOnly {
		s.splitDrag = ""
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
	w, h := s.measureNatural(e, width)
	if e.hidden {
		return w, h
	}
	return w, max(h, e.minHeight)
}
func (s *Surface) measureNatural(e *Element, width float64) (float64, float64) {
	switch e.kind {
	case TextKind, ButtonKind, BadgeKind, CheckboxKind, RadioKind, SwitchKind, InputKind, TextareaKind:
		s = s.textSurface(e)
	}
	if e.hidden || (e.toast != nil && !e.toast.active) {
		return 0, 0
	}
	w, h := e.width, e.height
	if e.measureContent != nil {
		available := width
		if w > 0 {
			available = min(available, w)
		}
		mw, mh := e.measureContent(max(1, available))
		if w == 0 {
			w = mw
		}
		if h == 0 {
			h = mh
		}
		return w, h
	}
	if e.width > 0 {
		width = min(width, e.width)
	}
	if e.kind == RowKind {
		sizes := s.rowSizes(e, max(0, width-2*(e.padding+e.paddingX)))
		cw, ch := 0., 0.
		for _, child := range e.children {
			r, ok := sizes[child]
			if !ok {
				continue
			}
			cw += r.W
			ch = max(ch, r.H)
		}
		if w == 0 {
			w = cw + float64(max(0, len(sizes)-1))*e.gap + 2*(e.padding+e.paddingX)
		}
		if h == 0 {
			h = ch + 2*e.padding
		}
		return w, h
	}
	switch e.kind {
	case ColumnKind, CardKind, ModalKind, PopupKind, ScrollKind, RowKind:
		var cw, ch float64
		visible := 0
		for _, child := range e.children {
			if child.hidden || overlay(child) {
				continue
			}
			a, b := s.measure(child, max(0, width-2*(e.padding+e.paddingX)))
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
	case AspectKind:
		if w == 0 {
			w = width
		}
		if h == 0 {
			h = w / e.ratio
		}
	case TextKind:
		limit := width
		if e.width > 0 {
			limit = min(limit, e.width)
		}
		lines := s.textLines(e, limit)
		if w == 0 {
			for _, line := range lines {
				w = max(w, s.textWidth(line))
			}
			if e.wrap {
				w = min(w, max(0, limit))
			}
		}
		if h == 0 {
			h = s.lineHeight() * float64(len(lines))
		}
	case ButtonKind, BadgeKind:
		if w == 0 {
			padding := e.controlPadding
			if padding == 0 {
				padding = 12
			}
			if e.inlineLink {
				padding = 0
			}
			w = s.textWidth(e.text) + 2*padding
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

func (s *Surface) layout(e *Element, r, clip Rect, disabled bool, modal, parent string) {
	if e.hidden || (e.toast != nil && !e.toast.active) {
		return
	}
	disabled = disabled || e.disabled
	if overlay(e) {
		if s.placing != e.id {
			s.deferOverlay(e, disabled, modal, parent)
			return
		}
		s.placing = ""
		var ok bool
		r, ok = s.placeOverlay(e)
		if !ok {
			return
		}
		if e.passive {
			if i, ok := s.index[e.anchor]; ok {
				modal = s.entries[i].modal
			}
		}
		if !e.passive {
			modal = e.id
			s.modal = e.id
			s.layers = append(s.layers, e.id)
		}
		clip = Rect{W: float64(s.width), H: float64(s.height)}
	}
	entryClip := intersect(r, clip)
	s.index[e.id] = len(s.entries)
	rtl := e.direction == RightToLeft
	if e.direction == InheritDirection && parent != "" {
		rtl = s.entries[s.index[parent]].rtl
	}
	decorationClip := clip
	if !overlay(e) {
		decorationClip = Rect{W: float64(s.width), H: float64(s.height)}
		for ancestor := parent; ancestor != ""; {
			a := s.entries[s.index[ancestor]]
			switch a.element.kind {
			case ScrollKind, CardKind, ModalKind, PopupKind, AspectKind:
				decorationClip = intersect(decorationClip, a.clip)
			}
			ancestor = a.parent
		}
	}
	s.entries = append(s.entries, entry{element: e, bounds: r, clip: entryClip, paintClip: decorationClip, disabled: disabled, modal: modal, parent: parent, rtl: rtl})
	if e.kind == AspectKind {
		s.layout(e.children[0], r, entryClip, disabled, modal, e.id)
		return
	}
	if e.kind == SplitKind {
		s.layoutSplit(e, r, clip, disabled, modal)
		return
	}
	if e.kind == ScrollKind {
		s.layoutScroll(e, r, clip, disabled, modal)
		return
	}
	inner := r.inset(e.padding)
	inner.X += e.paddingX
	inner.W = max(0, inner.W-2*e.paddingX)
	childClip := intersect(inner, clip)
	if e.kind != ColumnKind && e.kind != RowKind && e.kind != CardKind && e.kind != ModalKind && e.kind != PopupKind {
		return
	}
	rowSizes := map[*Element]Rect{}
	if e.kind == RowKind {
		rowSizes = s.rowSizes(e, inner.W)
	}
	var fixed, grow float64
	count := 0
	for _, c := range e.children {
		if c.hidden || overlay(c) || (c.toast != nil && !c.toast.active) {
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
	if rtl && e.kind == RowKind {
		x = inner.X + inner.W
	}
	for _, c := range e.children {
		if c.hidden || overlay(c) || (c.toast != nil && !c.toast.active) {
			continue
		}
		w, h := s.measure(c, inner.W)
		bonus := 0.
		if grow > 0 {
			bonus = extra * c.grow / grow
		}
		if e.kind == RowKind {
			size := rowSizes[c]
			w, h = size.W, size.H
			if c.height == 0 {
				h = min(h, inner.H)
			}
		} else {
			h += bonus
			if c.width == 0 && c.kind != BadgeKind && c.role != "tablist" {
				w = inner.W
			}
		}
		if rtl && e.kind == RowKind {
			x -= w
		}
		if c.kind == ScrollKind {
			h = min(h, max(0, inner.Y+inner.H-y))
			w = min(w, inner.W)
		}
		childY := y
		if e.kind == RowKind {
			if c.kind == SeparatorKind && c.vertical && c.height == 0 {
				h = inner.H
			}
			if e.centerItems {
				childY = inner.Y + max(0, (inner.H-h)/2)
			}
		}
		s.layout(c, Rect{x, childY, w, h}, childClip, disabled, modal, e.id)
		if e.kind == RowKind {
			if rtl {
				x -= e.gap
			} else {
				x += w + e.gap
			}
		} else {
			y += h + e.gap
		}
	}
	// Modals render and hit-test above ordinary children regardless of source order.
	for _, c := range e.children {
		if overlay(c) {
			s.deferOverlay(c, disabled, modal, e.id)
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
	defer s.syncHover()
	if i, ok := s.index[id]; ok && s.focusEligible(id) && interactive(s.entries[i].element) {
		s.reveal(id)
		s.focus = id
		if e := s.entries[i].element; e.nativeView != nil && !e.readOnly {
			e.nativeView.view.Focus()
		}
		s.focusVisible = true
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
	return e, !e.disabled && s.inScope(e) && e.clip.W > 0 && e.clip.H > 0
}
func interactive(e *Element) bool {
	switch e.kind {
	case ButtonKind, CheckboxKind, SwitchKind, RadioKind, InputKind, TextareaKind, SliderKind, ScrollKind:
		return true
	}
	return e.onKey != nil || e.onContext != nil || e.onPointer != nil
}
func (s *Surface) focusNext(back bool) {
	// A selection group occupies one Tab stop. Prefer the focused member,
	// then its selected member, then the first available member.
	groups := map[string]string{}
	for _, item := range s.entries {
		e := item.element
		if e.focusGroup == "" {
			continue
		}
		if !s.focusEligible(e.id) || !interactive(e) {
			continue
		}
		prior := groups[e.focusGroup]
		if prior == s.focus && prior != "" {
			continue
		}
		if prior == "" || e.id == s.focus || e.selected || e.checked {
			groups[e.focusGroup] = e.id
		}
	}
	ids := []string{}
	for _, item := range s.entries {
		e := item.element
		if !e.skipTab && s.focusEligible(e.id) && interactive(e) && (e.focusGroup == "" || groups[e.focusGroup] == e.id) {
			ids = append(ids, e.id)
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
	s.Focus(ids[at])
}
func (s *Surface) hit(x, y float64) string {
	for i := len(s.entries) - 1; i >= 0; i-- {
		e := s.entries[i]
		if e.clip.Contains(x, y) && overlay(e.element) && (e.element.hoverOptions == nil || !e.element.hoverOptions.tooltip) {
			return ""
		}
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
	if ev.Kind == "down" {
		s.focusVisible = false
	}
	if s.closed {
		return errors.New("baseui: closed surface")
	}
	if !finite(ev.X) || !finite(ev.Y) {
		return errors.New("baseui: invalid pointer position")
	}
	defer s.syncHover()
	s.pointerX, s.pointerY, s.pointerKnown = ev.X, ev.Y, ev.Kind != "cancel" && ev.Kind != "leave"
	if ev.Button != 0 {
		if ev.Button == 1 && ev.Kind == "down" {
			return s.contextAt(ev.X, ev.Y)
		}
		return nil
	}
	target := s.hit(ev.X, ev.Y)
	if ev.Kind == "move" || ev.Kind == "leave" || ev.Kind == "cancel" {
		hoverTarget := target
		if ev.Kind != "move" {
			hoverTarget = ""
		}
		if err := s.notifyPointerHover(hoverTarget, ev); err != nil {
			return err
		}
	}

	if ev.Kind != "down" && s.pressed != "" {
		if item, ok := s.enabled(s.pressed); ok && item.element.onPointer != nil && !item.element.readOnly {
			if ev.Kind == "up" || ev.Kind == "cancel" {
				s.pressed = ""
			}
			err := guarded(func() { item.element.onPointer(ev, item.bounds, Rect{W: float64(s.width), H: float64(s.height)}) })
			s.relayout()
			return err
		}
	}

	switch ev.Kind {
	case "down":
		if s.modal != "" {
			panel := s.entries[s.index[s.modal]]
			if !panel.bounds.Contains(ev.X, ev.Y) && !(panel.element.allowAnchorFocus && s.entries[s.index[panel.element.anchor]].bounds.Contains(ev.X, ev.Y)) {
				s.pressed = ""
				if panel.element.dismissOutside {
					return guarded(panel.element.activate)
				}
				return nil
			}
		}
		for i := len(s.entries) - 1; i >= 0; i-- {
			item := s.entries[i]
			if item.element.scroll != nil && item.clip.Contains(ev.X, ev.Y) && s.inScope(item) && s.startScrollDrag(item, ev.X, ev.Y) {
				s.pressed = ""
				return nil
			}
		}
		s.pressed = target
		if target != "" {
			s.focus = target
		}
		if i, ok := s.index[target]; ok {
			item := s.entries[i]
			if item.element.split != nil && !item.element.readOnly {
				s.splitDrag = target
				s.splitStart = item.element.split.Fraction
				return nil
			}
		}
		if item, ok := s.enabled(target); ok && item.element.onPointer != nil && !item.element.readOnly {
			err := guarded(func() { item.element.onPointer(ev, item.bounds, Rect{W: float64(s.width), H: float64(s.height)}) })
			s.relayout()
			return err
		}
		return s.drag(ev.X, ev.Y, false)
	case "move":
		if s.splitDrag != "" {
			s.dragSplit(ev.X, ev.Y)
			return nil
		}
		if s.scrollDrag != "" {
			s.dragScroll(ev.X, ev.Y)
			return nil
		}
		s.hover = target
		return s.drag(ev.X, ev.Y, true)
	case "leave":
		s.hover = ""
	case "cancel":
		s.hover = ""
		s.scrollDrag = ""
		if i, ok := s.index[s.splitDrag]; ok {
			s.entries[i].element.split.Fraction = s.splitStart
			s.relayout()
		}
		s.splitDrag = ""
		s.pressed = ""
	case "up":
		if s.splitDrag != "" {
			s.splitDrag = ""
			s.pressed = ""
			return nil
		}
		if s.scrollDrag != "" {
			s.scrollDrag = ""
			return nil
		}
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
	if e.element.otpLength > 0 {
		s.selectOTP(e, x, extend)
		return nil
	}
	if e.element.buffer != nil {
		metrics := s.textSurface(e.element)
		b := e.element.buffer
		lines := strings.Split(b.text, "\n")
		row := 0
		if e.element.kind == TextareaKind {
			row = max(0, min(len(lines)-1, int((y-e.bounds.Y-8)/metrics.lineHeight())))
		}
		offset := 0
		for i := 0; i < row; i++ {
			offset += len([]rune(lines[i])) + 1
		}
		inset := inputTextInset(e.element)
		local := x - e.bounds.X - inset
		if e.element.kind == InputKind {
			local += max(0, metrics.textWidth(string([]rune(b.text)[:b.caret]))-(e.bounds.W-2*inset))
		}
		runes := []rune(lines[row])
		column := len(runes)
		for i := range runes {
			if local < (metrics.textWidth(string(runes[:i]))+metrics.textWidth(string(runes[:i+1])))/2 {
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
	if !ev.Up {
		s.focusVisible = true
	}
	defer s.syncHover()
	if ev.Command {
		ev.Key = strings.ToUpper(ev.Key)
	}
	if s.closed {
		return errors.New("baseui: closed surface")
	}
	if !ev.Up && !ev.Repeat && (ev.Key == "ContextMenu" || ev.Key == "F10" && ev.Shift) {
		return s.contextFocused()
	}
	if ev.Key == "Tab" && !ev.Up {
		if item, ok := s.enabled(s.focus); ok && item.element.captureTab && !item.element.readOnly && !ev.Command && item.element.onKey != nil {
			handled := false
			if err := guarded(func() { handled = item.element.onKey(ev) }); err != nil {
				return err
			}
			if handled {
				return nil
			}
		}
		s.focusNext(ev.Shift)
		return nil
	}
	if ev.Key == "Escape" && !ev.Up {
		if focused, ok := s.enabled(s.focus); ok && focused.element.buffer != nil && focused.element.buffer.composition != "" {
			focused.element.buffer.CancelComposition()
			return nil
		}
	}
	if ev.Key == "Escape" && !ev.Up && s.dismissHover() {
		return nil
	}
	if ev.Key == "Escape" && !ev.Up && s.modal != "" {
		return guarded(s.entries[s.index[s.modal]].element.activate)
	}
	ev.RequestFocus = func(id string) { s.requestedFocus = id }
	s.reveal(s.focus)
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
	if s.splitKey(e, ev) {
		return nil
	}
	if s.scrollKey(e, ev) {
		return nil
	}
	ev.RTL = e.rtl
	if v.onKey != nil && !v.readOnly {
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
			before := v.buffer.String()
			handled := v.buffer.key(ev, v.kind == TextareaKind, v.readOnly)
			if before != v.buffer.String() && v.onChange != nil {
				v.onChange(v.buffer.String())
			}
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
	if !ok || e.element.readOnly {
		return nil
	}
	if e.element.onText != nil {
		return guarded(func() {
			if id := e.element.onText(text, s.now); id != "" {
				s.Focus(id)
			}
		})
	}
	if e.element.buffer == nil {
		return nil
	}
	return guarded(func() { s.replaceInput(e.element, text) })
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
	return guarded(func() {
		text := e.element.buffer.Composition()
		if text == "" {
			return
		}
		e.element.buffer.CancelComposition()
		s.replaceInput(e.element, text)
	})
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
		describedBy := ""
		for _, candidate := range s.hoverCandidates {
			if candidate.hoverOptions.tooltip && candidate.anchor == e.id {
				if _, visible := s.index[candidate.id]; visible {
					describedBy = candidate.id
					break
				}
			}
		}
		out = append(out, Accessibility{DescribedBy: describedBy, ID: e.id, Role: e.role, Label: e.label, Description: e.description, Value: value, Bounds: item.bounds, Level: e.treeLevel, Expanded: e.expanded, HasPopup: e.controls != "", Controls: e.controls, Disabled: item.disabled || !s.inScope(item), ReadOnly: e.readOnly, Invalid: e.invalid, Focused: s.focus == e.id, Checked: e.checked, Mixed: e.mixed, Selected: e.selected})
	}
	return out
}
func (s *Surface) Advance(now time.Duration) error {
	if s.closed {
		return errors.New("baseui: closed surface")
	}
	if now < s.now {
		return errors.New("baseui: animation time cannot move backwards")
	}
	s.now = now
	for _, item := range s.entries {
		if item.element.onFrame != nil {
			if err := guarded(func() {
				item.element.onFrame(now, !s.inactive && !item.disabled && !item.element.readOnly && item.clip.W > 0 && item.clip.H > 0, s.theme.ReduceMotion)
			}); err != nil {
				return err
			}
		}
	}
	s.syncHover()
	return nil
}
func (s *Surface) NeedsAnimation() bool {
	if s.theme.ReduceMotion || s.inactive {
		return false
	}
	for _, e := range s.entries {
		if (e.element.kind == SpinnerKind || e.element.animated || e.element.animating != nil && !e.disabled && !e.element.readOnly && e.element.animating()) && e.clip.W > 0 && e.clip.H > 0 {
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
