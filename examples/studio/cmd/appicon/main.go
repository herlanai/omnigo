// Command appicon rasterizes Studio's vector mark and writes a macOS ICNS file.
// PNG-backed ICNS entries cover standard and Retina sizes without external tools.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

func main() {
	source := flag.String("source", "examples/studio/assets/app-icon.svg", "vector source")
	output := flag.String("out", "build/studio/AppIcon.icns", "ICNS destination")
	preview := flag.String("preview", "", "optional 1024px PNG destination")
	flag.Parse()
	data, err := os.ReadFile(*source)
	if err != nil {
		log.Fatal(err)
	}
	var payload bytes.Buffer
	for _, entry := range []struct {
		tag  string
		size int
	}{{"icp4", 16}, {"icp5", 32}, {"icp6", 64}, {"ic07", 128}, {"ic08", 256}, {"ic09", 512}, {"ic10", 1024}, {"ic11", 32}, {"ic12", 64}, {"ic13", 256}, {"ic14", 512}} {
		svg, err := oksvg.ReadIconStream(bytes.NewReader(data), oksvg.StrictErrorMode)
		if err != nil {
			log.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, entry.size, entry.size))
		svg.SetTarget(0, 0, float64(entry.size), float64(entry.size))
		svg.Draw(rasterx.NewDasher(entry.size, entry.size, rasterx.NewScannerGV(entry.size, entry.size, img, img.Bounds())), 1)
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, img); err != nil {
			log.Fatal(err)
		}
		payload.WriteString(entry.tag)
		_ = binary.Write(&payload, binary.BigEndian, uint32(encoded.Len()+8))
		payload.Write(encoded.Bytes())
		if entry.size == 1024 && *preview != "" {
			write(*preview, encoded.Bytes())
		}
	}
	var result bytes.Buffer
	result.WriteString("icns")
	_ = binary.Write(&result, binary.BigEndian, uint32(payload.Len()+8))
	result.Write(payload.Bytes())
	write(*output, result.Bytes())
}
func write(path string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Fatal(err)
	}
}
