//go:build ios || android

package mobile

import (
	"errors"
	"image/draw"
	"log"
	"math"
	"sync/atomic"
	"time"

	"golang.org/x/mobile/app"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/event/touch"
	"golang.org/x/mobile/exp/gl/glutil"
	"golang.org/x/mobile/geom"
	"golang.org/x/mobile/gl"
)

type metrics struct{ scale, top, right, bottom, left float64 }

var running atomic.Bool

// Run owns the process native event loop and must be called by main. Native
// linking uses cgo only on mobile; this file is excluded from all desktop builds.
func Run(options Options) error {
	if options.Create == nil {
		return errors.New("mobile: Create is required")
	}
	if !running.CompareAndSwap(false, true) {
		return errors.New("mobile: Run may only be called once")
	}
	app.Main(func(a app.App) {
		report := func(err error) {
			if err != nil {
				if options.OnError != nil {
					options.OnError(err)
				} else {
					log.Print(err)
				}
			}
		}
		c, err := newCanvas(options)
		if err != nil {
			report(err)
			return
		}
		defer c.close()
		var ctx gl.Context
		var images *glutil.Images
		var texture *glutil.Image
		var sz size.Event
		var metrics metrics
		viewportOK := false
		pending := false
		requestPaint := func() {
			if !pending {
				pending = true
				a.Send(paint.Event{})
			}
		}
		start := time.Now()
		ready := false
		release := func() {
			if texture != nil {
				texture.Release()
				texture = nil
			}
			if images != nil {
				images.Release()
				images = nil
			}
			ctx = nil
		}
		defer release()
		for event := range a.Events() {
			switch e := a.Filter(event).(type) {
			case lifecycle.Event:
				if e.Crosses(lifecycle.StageVisible) == lifecycle.CrossOn {
					ctx, _ = e.DrawContext.(gl.Context)
					if ctx == nil {
						report(errors.New("mobile: missing native GL context"))
						continue
					}
					images = glutil.NewImages(ctx)
					report(c.activate(true))
					requestPaint()
				} else if e.Crosses(lifecycle.StageVisible) == lifecycle.CrossOff {
					report(c.activate(false))
					release()
				}
				if e.To == lifecycle.StageDead {
					return
				}
			case size.Event:
				sz = e
				viewportOK = false
				if e.WidthPx > 0 && e.HeightPx > 0 && e.PixelsPerPt > 0 {
					metrics = nativeMetrics(float64(e.PixelsPerPt), options.Dark)
					w := int(math.Floor(float64(e.WidthPx)/metrics.scale - metrics.left - metrics.right))
					h := int(math.Floor(float64(e.HeightPx)/metrics.scale - metrics.top - metrics.bottom))
					err := c.resize(w, h, metrics.scale)
					report(err)
					viewportOK = err == nil
					requestPaint()
				}
			case touch.Event:
				kind := ""
				switch e.Type {
				case touch.TypeBegin:
					kind = "down"
				case touch.TypeMove:
					kind = "move"
				case touch.TypeEnd:
					kind = "up"
				}
				if kind != "" {
					report(c.touch(int64(e.Sequence), kind, float64(e.X)-metrics.left*metrics.scale, float64(e.Y)-metrics.top*metrics.scale))
					requestPaint()
				}
			case paint.Event:
				pending = false
				if !viewportOK || ctx == nil || images == nil || !c.active || c.surface == nil || sz.WidthPx < 1 || sz.HeightPx < 1 {
					continue
				}
				frame, err := c.render(time.Since(start))
				if err != nil {
					report(err)
					continue
				}
				if texture == nil || texture.RGBA.Bounds().Size() != frame.Bounds().Size() {
					if texture != nil {
						texture.Release()
					}
					texture = images.NewImage(frame.Bounds().Dx(), frame.Bounds().Dy())
				}
				draw.Draw(texture.RGBA, texture.RGBA.Bounds(), frame, frame.Bounds().Min, draw.Src)
				ctx.Viewport(0, 0, sz.WidthPx, sz.HeightPx)
				if options.Dark {
					ctx.ClearColor(0.07, 0.07, 0.07, 1)
				} else {
					ctx.ClearColor(1, 1, 1, 1)
				}
				ctx.Clear(gl.COLOR_BUFFER_BIT)
				texture.Upload()
				left := geom.Pt(metrics.left * metrics.scale / float64(sz.PixelsPerPt))
				top := geom.Pt(metrics.top * metrics.scale / float64(sz.PixelsPerPt))
				right := left + geom.Pt(float64(frame.Bounds().Dx())/float64(sz.PixelsPerPt))
				bottom := top + geom.Pt(float64(frame.Bounds().Dy())/float64(sz.PixelsPerPt))
				texture.Draw(sz, geom.Point{X: left, Y: top}, geom.Point{X: right, Y: top}, geom.Point{X: left, Y: bottom}, texture.RGBA.Bounds())
				a.Publish()
				if !ready {
					ready = true
					if options.OnReady != nil {
						options.OnReady()
					}
				}
				if c.surface.NeedsAnimation() {
					requestPaint()
				}
			}
		}
	})
	return nil
}
