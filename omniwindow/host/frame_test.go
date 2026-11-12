package host

import (
	"bytes"
	"image/png"
	"testing"

	ui "github.com/herlanai/omnigo/omniui/baseui"
)

func TestFrameReusePixelsAndRelease(t *testing.T) {
	s, err := ui.NewSurface(80, 60, ui.NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var r frameRenderer
	for _, scale := range []float64{1, 1, 2, 2, 1} {
		if err = s.Mount(ui.Button("b", "Save", nil)); err != nil {
			t.Fatal(err)
		}
		before := r.frame
		data, err := r.render(s, 80, 60, scale)
		if err != nil {
			t.Fatal(err)
		}
		if before != nil && before.Bounds() == r.frame.Bounds() && before != r.frame {
			t.Fatal("frame not reused")
		}
		fresh, err := s.RenderScaled(scale, nil)
		if err != nil {
			t.Fatal(err)
		}
		var expected bytes.Buffer
		if err = png.Encode(&expected, fresh); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, expected.Bytes()) {
			t.Fatal("encoding changed")
		}
		if _, err = png.Decode(bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
	}
	r.reset()
	if r.frame != nil || r.encoder != nil || r.encoded.Cap() != 0 {
		t.Fatal("scratch buffers retained after reset")
	}
}

// Fresh and reuse run the same render+PNG workload; both exclude AppKit upload.
func BenchmarkFramePipeline(b *testing.B) {
	for _, reuse := range []bool{false, true} {
		name := "fresh"
		if reuse {
			name = "reuse"
		}
		b.Run(name, func(b *testing.B) {
			s, err := ui.NewSurface(640, 480, ui.NewTheme(false))
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			if err = s.Mount(ui.Card("card", ui.Heading("title", "Account settings", 2), ui.Button("save", "Save changes", nil))); err != nil {
				b.Fatal(err)
			}
			var r frameRenderer
			if reuse {
				if _, err = r.render(s, 640, 480, 2); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if reuse {
					if _, err = r.render(s, 640, 480, 2); err != nil {
						b.Fatal(err)
					}
				} else {
					img, err := s.RenderScaled(2, nil)
					if err != nil {
						b.Fatal(err)
					}
					var encoded bytes.Buffer
					if err = png.Encode(&encoded, img); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}

// Same scene and scale, comparing PNG preparation with raw pixels plus the
// copy required by native ownership. Excludes AppKit and display composition.
func BenchmarkFrameTransfer(b *testing.B) {
	for _, mode := range []string{"png", "pixels-copy"} {
		b.Run(mode, func(b *testing.B) {
			s, err := ui.NewSurface(640, 480, ui.NewTheme(false))
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			if err = s.Mount(ui.Card("card", ui.Heading("title", "Account settings", 2), ui.Button("save", "Save changes", nil))); err != nil {
				b.Fatal(err)
			}
			var r frameRenderer
			frame, err := r.pixels(s, 640, 480, 2)
			if err != nil {
				b.Fatal(err)
			}
			destination := make([]byte, len(frame.Pix))
			if _, err = r.render(s, 640, 480, 2); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if mode == "png" {
					if _, err = r.render(s, 640, 480, 2); err != nil {
						b.Fatal(err)
					}
				} else {
					frame, err = r.pixels(s, 640, 480, 2)
					if err != nil {
						b.Fatal(err)
					}
					copy(destination, frame.Pix)
				}
			}
		})
	}
}
func TestRawFrameMatchesPNG(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, err := ui.NewSurface(80, 60, ui.NewTheme(dark))
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		if err = s.Mount(ui.Button("button", "Save", nil)); err != nil {
			t.Fatal(err)
		}
		var r frameRenderer
		for _, scale := range []float64{1, 1.25, 2} {
			raw, err := r.pixels(s, 80, 60, scale)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := r.render(s, 80, 60, scale)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := png.Decode(bytes.NewReader(encoded))
			if err != nil {
				t.Fatal(err)
			}
			for y := 0; y < raw.Rect.Dy(); y++ {
				for x := 0; x < raw.Rect.Dx(); x++ {
					a, b, c, d := raw.At(x, y).RGBA()
					e, f, g, h := decoded.At(x, y).RGBA()
					if a != e || b != f || c != g || d != h {
						t.Fatal("pixel mismatch", dark, scale, x, y)
					}
				}
			}
		}
	}
}
