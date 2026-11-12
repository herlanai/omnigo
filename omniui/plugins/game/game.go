// Package game provides explicit-clock fixed-step simulation and camera math.
package game

import (
	"errors"
	"math"
	"time"
)

type Loop struct {
	Step              time.Duration
	MaxSteps          int
	last, accumulator time.Duration
	started, paused   bool
}
type Frame struct {
	Steps   int
	Alpha   float64
	Dropped time.Duration
}

// Advance never creates a timer or goroutine. The host calls it on its UI thread.
func (l *Loop) Advance(now time.Duration, paused bool, update func(time.Duration)) (Frame, error) {
	if now < 0 || update == nil {
		return Frame{}, errors.New("game: invalid clock or update")
	}
	step := l.Step
	if step == 0 {
		step = time.Second / 60
	}
	if step < 0 {
		return Frame{}, errors.New("game: invalid step")
	}
	limit := l.MaxSteps
	if limit <= 0 {
		limit = 8
	}
	if !l.started {
		l.started = true
		l.last = now
		l.paused = paused
		return Frame{}, nil
	}
	if now < l.last {
		return Frame{}, errors.New("game: clock moved backwards")
	}
	elapsed := now - l.last
	l.last = now
	if paused || l.paused {
		l.paused = paused
		return Frame{Alpha: float64(l.accumulator) / float64(step)}, nil
	}
	frame := Frame{}
	// Bound elapsed before adding it, avoiding duration overflow after long idle.
	maxDelta := step * time.Duration(limit)
	if maxDelta/step != time.Duration(limit) || maxDelta < 0 {
		return Frame{}, errors.New("game: invalid catch-up limit")
	}
	if elapsed > maxDelta {
		frame.Dropped = elapsed - maxDelta
		elapsed = maxDelta
	}
	for elapsed >= step-l.accumulator {
		elapsed -= step - l.accumulator
		l.accumulator = 0
		update(step)
		frame.Steps++
	}
	l.accumulator += elapsed
	frame.Alpha = float64(l.accumulator) / float64(step)
	return frame, nil
}
func (l *Loop) Reset() { l.started = false; l.accumulator = 0; l.paused = false }

type Point struct{ X, Y float64 }
type Camera struct {
	Position       Point
	Zoom, Rotation float64
}

func (c Camera) WorldToScreen(p, viewport Point) Point {
	z := c.Zoom
	if z == 0 {
		z = 1
	}
	x, y := p.X-c.Position.X, p.Y-c.Position.Y
	co, si := math.Cos(c.Rotation), math.Sin(c.Rotation)
	return Point{viewport.X/2 + (x*co+y*si)*z, viewport.Y/2 + (-x*si+y*co)*z}
}
func (c Camera) ScreenToWorld(p, viewport Point) Point {
	z := c.Zoom
	if z == 0 {
		z = 1
	}
	x, y := (p.X-viewport.X/2)/z, (p.Y-viewport.Y/2)/z
	co, si := math.Cos(c.Rotation), math.Sin(c.Rotation)
	return Point{c.Position.X + x*co - y*si, c.Position.Y + x*si + y*co}
}

type Fit struct{ X, Y, Width, Height, Scale int }

// IntegerFit uses whole pixels. If the canvas cannot fit, Scale=0 explicitly.
func IntegerFit(width, height, canvasWidth, canvasHeight int) Fit {
	if width <= 0 || height <= 0 || canvasWidth <= 0 || canvasHeight <= 0 {
		return Fit{}
	}
	s := min(width/canvasWidth, height/canvasHeight)
	w, h := canvasWidth*s, canvasHeight*s
	return Fit{(width - w) / 2, (height - h) / 2, w, h, s}
}
