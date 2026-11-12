package render

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser"
)

// PDF exports one bounded viewport as a lossless RGB image in a PDF 1.4 page.
// Text is rasterized, not searchable. Width/height are CSS pixels (96 DPI).
func PDF(snapshot omnibrowser.Snapshot, width, height int) ([]byte, error) {
	img, err := Screenshot(snapshot, width, height, false)
	if err != nil {
		return nil, err
	}
	var pixels bytes.Buffer
	zip := zlib.NewWriter(&pixels)
	row := make([]byte, width*3)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := img.RGBAAt(x, y)
			row[x*3], row[x*3+1], row[x*3+2] = c.R, c.G, c.B
		}
		if _, err := zip.Write(row); err != nil {
			return nil, err
		}
	}
	if err := zip.Close(); err != nil {
		return nil, err
	}
	w, h := float64(width)*.75, float64(height)*.75
	content := fmt.Sprintf("q %.2f 0 0 %.2f 0 0 cm /Im0 Do Q", w, h)
	objs := [][]byte{[]byte("<< /Type /Catalog /Pages 2 0 R >>"), []byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"), []byte(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /XObject << /Im0 4 0 R >> >> /Contents 5 0 R >>", w, h)), append(append([]byte(fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>\nstream\n", width, height, pixels.Len())), pixels.Bytes()...), []byte("\nendstream")...), []byte(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := []int{0}
	for i, obj := range objs {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		out.Write(obj)
		out.WriteString("\nendobj\n")
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return out.Bytes(), nil
}
