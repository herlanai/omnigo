package host

import (
	"bytes"
	"image"
	"image/png"
	"math"

	ui "github.com/herlanai/omnigo/omniui/baseui"
)

// frameRenderer is a single UI-goroutine scratch buffer, shared across content
// windows. The native presenter copies pixels before the next render reuses them.
// It retains at most one framebuffer and one PNG encoder, not one per window.
type frameRenderer struct {
	frame   *image.RGBA
	encoded bytes.Buffer
	encoder *png.EncoderBuffer
}

func (r *frameRenderer) Get() *png.EncoderBuffer  { b := r.encoder; r.encoder = nil; return b }
func (r *frameRenderer) Put(b *png.EncoderBuffer) { r.encoder = b }
func (r *frameRenderer) reset()                   { *r = frameRenderer{} }
func (r *frameRenderer) pixels(s *ui.Surface, width, height int, scale float64) (*image.RGBA, error) {
	bounds := image.Rect(0, 0, int(math.Ceil(float64(width)*scale)), int(math.Ceil(float64(height)*scale)))
	if r.frame == nil || r.frame.Bounds() != bounds {
		frame, err := s.RenderScaled(scale, nil)
		if err != nil {
			return nil, err
		}
		r.frame = frame
	} else if err := s.RenderScaledInto(r.frame, scale, nil); err != nil {
		return nil, err
	}
	return r.frame, nil
}

// render retains the PNG path for export and comparative benchmarks.
func (r *frameRenderer) render(s *ui.Surface, width, height int, scale float64) ([]byte, error) {
	if _, err := r.pixels(s, width, height, scale); err != nil {
		return nil, err
	}
	r.encoded.Reset()
	encoder := png.Encoder{BufferPool: r}
	if err := encoder.Encode(&r.encoded, r.frame); err != nil {
		return nil, err
	}
	return r.encoded.Bytes(), nil
}
