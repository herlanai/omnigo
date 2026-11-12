package native

import (
	"bytes"
	"image"
	"image/png"
	"math"

	ui "omnigo.dev/omnigo/omniui/baseui"
)

// frameRenderer is a single UI-goroutine scratch buffer, shared across demo
// windows. NSData copies the encoded bytes before the next render can reuse it.
// It retains at most one framebuffer and one PNG encoder, not one per window.
type frameRenderer struct {
	frame   *image.RGBA
	encoded bytes.Buffer
	encoder *png.EncoderBuffer
}

func (r *frameRenderer) Get() *png.EncoderBuffer  { b := r.encoder; r.encoder = nil; return b }
func (r *frameRenderer) Put(b *png.EncoderBuffer) { r.encoder = b }
func (r *frameRenderer) reset()                   { *r = frameRenderer{} }
func (r *frameRenderer) render(s *ui.Surface, width, height int, scale float64) ([]byte, error) {
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
	r.encoded.Reset()
	encoder := png.Encoder{BufferPool: r}
	if err := encoder.Encode(&r.encoded, r.frame); err != nil {
		return nil, err
	}
	return r.encoded.Bytes(), nil
}
