package baseui

import (
	"math"
	"time"
)

// AnimationTimeline uses an explicit monotonic clock. No goroutine or timer is
// owned by the state. Surface advances only while the control is visible.
type AnimationTimeline struct {
	Duration, Delay, RepeatDelay time.Duration
	// Repeat is the number of additional plays; -1 repeats indefinitely.
	Repeat                     int
	Alternate, Reverse, Paused bool
	Position                   time.Duration
	elapsed, at                time.Duration
	clock, ended               bool
}

func (t *AnimationTimeline) validate() {
	if t.Duration <= 0 || t.Duration > 24*time.Hour || t.Delay < 0 || t.Delay > 24*time.Hour || t.RepeatDelay < 0 || t.RepeatDelay > 24*time.Hour || t.Repeat < -1 || t.Repeat > 100000 {
		panic("baseui: invalid timeline")
	}
	if t.Repeat > 0 && t.Duration+t.RepeatDelay > (time.Duration(1<<63-1)-t.Delay-t.Duration)/time.Duration(t.Repeat) {
		panic("baseui: timeline duration overflow")
	}
}
func (t *AnimationTimeline) Seek(position time.Duration) {
	t.validate()
	t.Position = max(0, min(t.Duration, position))
	p := t.Position
	if t.Reverse {
		p = t.Duration - p
	}
	t.elapsed = t.Delay + p
	t.ended = false
	t.clock = false
}
func (t *AnimationTimeline) Restart() {
	t.elapsed = 0
	t.clock = false
	t.ended = false
	t.Position = 0
	if t.Reverse {
		t.Position = t.Duration
	}
}
func (t *AnimationTimeline) Progress() float64 {
	if t.Duration <= 0 {
		return 0
	}
	return float64(t.Position) / float64(t.Duration)
}
func (t *AnimationTimeline) Settled() bool { return t.ended }
func (t *AnimationTimeline) Advance(now time.Duration, active, reduceMotion bool) {
	t.validate()
	if !active {
		t.clock = false
		return
	}
	if t.clock && now < t.at {
		panic("baseui: timeline clock moved backwards")
	}
	if t.clock && !t.Paused && !t.ended {
		t.elapsed += min(now-t.at, time.Duration(1<<63-1)-t.elapsed)
	}
	t.at, t.clock = now, true
	if reduceMotion && !t.Paused {
		t.Position = t.Duration
		if t.Reverse {
			t.Position = 0
		}
		t.ended = true
		return
	}
	elapsed := max(time.Duration(0), t.elapsed-t.Delay)
	period := t.Duration + t.RepeatDelay
	cycle := elapsed / period
	position := min(t.Duration, elapsed%period)
	if t.Repeat >= 0 && elapsed >= time.Duration(t.Repeat)*period+t.Duration {
		cycle = time.Duration(t.Repeat)
		position = t.Duration
		t.ended = true
	}
	reverse := t.Reverse != (t.Alternate && cycle%2 == 1)
	if reverse {
		position = t.Duration - position
	}
	t.Position = position
}

// Tween samples a segment sharing this timeline's playhead. smooth uses the
// cubic smoothstep easing; false is linear. Segments may overlap or follow.
func (t *AnimationTimeline) Tween(from, to float64, start, duration time.Duration, smooth bool) float64 {
	if !finite(from) || !finite(to) || start < 0 || duration <= 0 {
		panic("baseui: invalid tween")
	}
	x := math.Max(0, math.Min(1, float64(t.Position-start)/float64(duration)))
	if smooth {
		x = x * x * (3 - 2*x)
	}
	return from + (to-from)*x
}
func Timeline(id string, state *AnimationTimeline, paint func(*Painter, Rect, float64)) *Element {
	if state == nil || paint == nil {
		panic("baseui: nil timeline")
	}
	state.validate()
	e := Canvas(id, "Animated timeline", func(p *Painter, r Rect) { paint(p, r, state.Progress()) }).Height(100)
	e.onFrame = func(now time.Duration, active, reduceMotion bool) { state.Advance(now, active, reduceMotion) }
	e.animating = func() bool { return !state.Paused && !state.Settled() }
	return e
}
