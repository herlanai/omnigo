package baseui

import (
	"context"
	"errors"
	"image"
	"image/draw"
	"image/gif"
	"io"
	"time"
)

type VideoFrame struct {
	Image    image.Image
	Duration time.Duration
}

// VideoDecoder may perform IO on its caller's goroutine. DecodeVideo is a load
// operation, not a UI callback: load off-thread and dispatch the result to UI.
type VideoDecoder interface {
	Next(context.Context) (VideoFrame, error)
}
type VideoPlayer struct {
	frames []VideoFrame
	ends   []time.Duration
	clock  AnimationTimeline
	closed bool
}

// NewVideoPlayer snapshots pixels. The full decoded clip is capped at 64 MiB;
// streamed decode-ahead and audio are not implemented by this frame player.
func NewVideoPlayer(frames []VideoFrame) (*VideoPlayer, error) {
	if len(frames) == 0 || len(frames) > 4096 {
		return nil, errors.New("baseui: invalid video frame count")
	}
	p := &VideoPlayer{}
	pixels := int64(0)
	for _, f := range frames {
		if f.Image == nil {
			return nil, errors.New("baseui: nil video frame")
		}
		b := f.Image.Bounds()
		w, h := b.Dx(), b.Dy()
		if w <= 0 || h <= 0 || w > 4096 || h > 4096 {
			return nil, errors.New("baseui: invalid video dimensions")
		}
		pixels += int64(w) * int64(h)
		if pixels > 16*1024*1024 {
			return nil, errors.New("baseui: decoded video exceeds 64 MiB")
		}
		if f.Duration <= 0 {
			f.Duration = 100 * time.Millisecond
		}
		if f.Duration > time.Hour || p.clock.Duration+f.Duration > 24*time.Hour {
			return nil, errors.New("baseui: video duration exceeds budget")
		}
		snapshot := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(snapshot, snapshot.Bounds(), f.Image, b.Min, draw.Src)
		p.frames = append(p.frames, VideoFrame{snapshot, f.Duration})
		p.clock.Duration += f.Duration
		p.ends = append(p.ends, p.clock.Duration)
	}
	return p, nil
}
func DecodeVideo(ctx context.Context, decoder VideoDecoder) (*VideoPlayer, error) {
	if decoder == nil {
		return nil, errors.New("baseui: nil video decoder")
	}
	// Copy each frame before requesting another: decoders may reuse their pixels.
	frames := []VideoFrame{}
	pixels := int64(0)
	for len(frames) < 4096 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		frame, err := decoder.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		one, err := NewVideoPlayer([]VideoFrame{frame})
		if err != nil {
			return nil, err
		}
		pixels += int64(one.frames[0].Image.Bounds().Dx()) * int64(one.frames[0].Image.Bounds().Dy())
		if pixels > 16*1024*1024 {
			return nil, errors.New("baseui: decoded video exceeds 64 MiB")
		}
		frames = append(frames, one.frames[0])
	}
	if len(frames) == 4096 {
		return nil, errors.New("baseui: decoder frame limit exceeded")
	}
	// Frames already have independent storage; transfer instead of copying again.
	if len(frames) == 0 {
		return nil, errors.New("baseui: empty video")
	}
	p := &VideoPlayer{frames: frames}
	for _, f := range frames {
		p.clock.Duration += f.Duration
		if p.clock.Duration > 24*time.Hour {
			return nil, errors.New("baseui: video duration exceeds budget")
		}
		p.ends = append(p.ends, p.clock.Duration)
	}
	return p, nil
}

// NewGIFVideo composites already-decoded GIF frames with disposal handling.
// The caller chooses how and where to decode the GIF input stream.
func NewGIFVideo(g *gif.GIF) (*VideoPlayer, error) {
	if g == nil || len(g.Image) == 0 || len(g.Image) != len(g.Delay) || g.Config.Width <= 0 || g.Config.Height <= 0 || g.Config.Width > 4096 || g.Config.Height > 4096 || int64(g.Config.Width)*int64(g.Config.Height)*int64(len(g.Image)) > 16*1024*1024 {
		return nil, errors.New("baseui: invalid or oversized GIF")
	}
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	frames := make([]VideoFrame, 0, len(g.Image))
	for i, frame := range g.Image {
		if frame == nil {
			return nil, errors.New("baseui: nil GIF frame")
		}
		var previous *image.RGBA
		disposal := byte(0)
		if i < len(g.Disposal) {
			disposal = g.Disposal[i]
		}
		if disposal == gif.DisposalPrevious {
			previous = image.NewRGBA(canvas.Bounds())
			copy(previous.Pix, canvas.Pix)
		}
		draw.Draw(canvas, frame.Bounds().Intersect(canvas.Bounds()), frame, frame.Bounds().Min, draw.Over)
		snapshot := image.NewRGBA(canvas.Bounds())
		copy(snapshot.Pix, canvas.Pix)
		duration := time.Duration(g.Delay[i]) * 10 * time.Millisecond
		if duration <= 0 {
			duration = 100 * time.Millisecond
		}
		frames = append(frames, VideoFrame{snapshot, duration})
		if disposal == gif.DisposalBackground {
			draw.Draw(canvas, frame.Bounds().Intersect(canvas.Bounds()), image.Transparent, image.Point{}, draw.Src)
		}
		if previous != nil {
			canvas = previous
		}
	}
	p := &VideoPlayer{frames: frames}
	for _, f := range frames {
		p.clock.Duration += f.Duration
		p.ends = append(p.ends, p.clock.Duration)
	}
	p.clock.Repeat = g.LoopCount
	if g.LoopCount == 0 {
		p.clock.Repeat = -1
	}
	if g.LoopCount < 0 {
		p.clock.Repeat = 0
	}
	return p, nil
}
func (p *VideoPlayer) Pause(v bool) { p.clock.Paused = v }
func (p *VideoPlayer) Paused() bool { return p.clock.Paused }
func (p *VideoPlayer) Loop(v bool) {
	if v {
		p.clock.Repeat = -1
	} else {
		p.clock.Repeat = 0
	}
}
func (p *VideoPlayer) Seek(at time.Duration) {
	if !p.closed {
		p.clock.Seek(at)
	}
}
func (p *VideoPlayer) Position() time.Duration { return p.clock.Position }
func (p *VideoPlayer) Duration() time.Duration { return p.clock.Duration }
func (p *VideoPlayer) Close()                  { p.closed = true; p.frames = nil; p.ends = nil; p.clock.Paused = true }
func Video(id string, player *VideoPlayer) *Element {
	if player == nil || player.closed {
		panic("baseui: closed or nil video player")
	}
	e := Canvas(id, "Video", func(p *Painter, r Rect) {
		if player.closed {
			return
		}
		index := 0
		for index < len(player.ends)-1 && player.clock.Position >= player.ends[index] {
			index++
		}
		bitmap := player.frames[index].Image
		b := bitmap.Bounds()
		scale := min(r.W/float64(b.Dx()), r.H/float64(b.Dy()))
		box := Rect{W: float64(b.Dx()) * scale, H: float64(b.Dy()) * scale}
		box.X = r.X + (r.W-box.W)/2
		box.Y = r.Y + (r.H-box.H)/2
		p.Fill(r, p.theme.Muted, 12)
		p.Image(box, bitmap, 0)
	}).Height(240).Role("img")
	e.onFrame = func(now time.Duration, active, reduceMotion bool) {
		if !player.closed {
			if reduceMotion {
				player.clock.Position = 0
				player.clock.Advance(now, false, false)
				return
			}
			player.clock.Advance(now, active, reduceMotion)
		}
	}
	e.animating = func() bool {
		return !player.closed && !player.clock.Paused && !player.clock.Settled() && len(player.frames) > 1
	}
	return e
}
