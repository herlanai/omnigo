//go:build darwin && (amd64 || arm64)

package gpu

import (
	"fmt"
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"image"
	"image/color"
	"structs"
	"sync"
	"unsafe"
)

// The native shader is compiled by Metal at runtime; no external shader compiler
// or non-Go build step is required. The algorithm is independently authored.
const shader = `#include <metal_stdlib>
using namespace metal;
struct Fill { float4 bounds; float4 color; };
struct Parameters { uint width; uint height; uint count; uint unused; float4 background; };
kernel void fill_rects(const device Fill* fills [[buffer(0)]], device uchar4* pixels [[buffer(1)]], constant Parameters& p [[buffer(2)]], uint2 position [[thread_position_in_grid]]) {
 if (position.x >= p.width || position.y >= p.height) return;
 float2 point = float2(position) + 0.5;
 float4 value = p.background; value.rgb *= value.a;
 for (uint i = 0; i < p.count; i++) {
  float4 box = fills[i].bounds;
  if (point.x >= box.x && point.y >= box.y && point.x < box.x + box.z && point.y < box.y + box.w) {
   float4 source = fills[i].color; source.rgb *= source.a;
   value = source + value * (1.0 - source.a);
  }
 }
 pixels[position.y * p.width + position.x] = uchar4(clamp(value, 0.0, 1.0) * 255.0 + 0.5);
}`

type metalSize struct {
	_                    structs.HostLayout
	Width, Height, Depth uintptr
}
type fill struct{ Bounds, Color [4]float32 }
type parameters struct {
	Width, Height, Count, Unused uint32
	Background                   [4]float32
}
type metal struct{ device, queue, pipeline objc.ID }

var metalOnce sync.Once
var metalError error
var createDevice func() objc.ID

func sel(s string) objc.SEL { return objc.RegisterName(s) }
func ns(s string) objc.ID {
	return objc.ID(objc.GetClass("NSString")).Send(sel("stringWithUTF8String:"), s)
}
func nativeError(stage string, e objc.ID) error {
	if e != 0 {
		return fmt.Errorf("gpu: %s: %s", stage, objc.Send[string](e.Send(sel("localizedDescription")), sel("UTF8String")))
	}
	return fmt.Errorf("gpu: %s failed", stage)
}
func openRenderer() (renderer, error) {
	metalOnce.Do(func() {
		_, metalError = purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if metalError != nil {
			return
		}
		var lib uintptr
		lib, metalError = purego.Dlopen("/System/Library/Frameworks/Metal.framework/Metal", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if metalError == nil {
			purego.RegisterLibFunc(&createDevice, lib, "MTLCreateSystemDefaultDevice")
		}
	})
	if metalError != nil {
		return nil, metalError
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(sel("new"))
	defer pool.Send(sel("drain"))
	m := &metal{device: createDevice()}
	if m.device == 0 {
		return nil, ErrUnavailable
	}
	success := false
	defer func() {
		if !success {
			m.close()
		}
	}()
	m.queue = m.device.Send(sel("newCommandQueue"))
	if m.queue == 0 {
		return nil, nativeError("create command queue", 0)
	}
	var errorObject objc.ID
	library := m.device.Send(sel("newLibraryWithSource:options:error:"), ns(shader), objc.ID(0), &errorObject)
	if library == 0 {
		return nil, nativeError("compile shader", errorObject)
	}
	defer library.Send(sel("release"))
	function := library.Send(sel("newFunctionWithName:"), ns("fill_rects"))
	if function == 0 {
		return nil, nativeError("find kernel", 0)
	}
	defer function.Send(sel("release"))
	m.pipeline = m.device.Send(sel("newComputePipelineStateWithFunction:error:"), function, &errorObject)
	if m.pipeline == 0 {
		return nil, nativeError("create pipeline", errorObject)
	}
	success = true
	return m, nil
}
func (m *metal) close() {
	for _, id := range []objc.ID{m.pipeline, m.queue, m.device} {
		if id != 0 {
			id.Send(sel("release"))
		}
	}
}
func rgba(c color.NRGBA) [4]float32 {
	return [4]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A) / 255}
}
func (m *metal) render(width, height int, background color.NRGBA, rects []Rect) (*image.RGBA, error) {
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(sel("new"))
	defer pool.Send(sel("drain"))
	fills := make([]fill, max(1, len(rects)))
	for i, r := range rects {
		fills[i] = fill{[4]float32{r.X, r.Y, r.Width, r.Height}, rgba(r.Color)}
	}
	params := parameters{Width: uint32(width), Height: uint32(height), Count: uint32(len(rects)), Background: rgba(background)}
	input := m.device.Send(sel("newBufferWithBytes:length:options:"), unsafe.Pointer(&fills[0]), uintptr(len(fills))*unsafe.Sizeof(fill{}), uintptr(0))
	if input == 0 {
		return nil, nativeError("allocate input", 0)
	}
	defer input.Send(sel("release"))
	output := m.device.Send(sel("newBufferWithLength:options:"), uintptr(width*height*4), uintptr(0))
	if output == 0 {
		return nil, nativeError("allocate output", 0)
	}
	defer output.Send(sel("release"))
	command := m.queue.Send(sel("commandBuffer"))
	if command == 0 {
		return nil, nativeError("create command", 0)
	}
	encoder := command.Send(sel("computeCommandEncoder"))
	if encoder == 0 {
		return nil, nativeError("create encoder", 0)
	}
	encoder.Send(sel("setComputePipelineState:"), m.pipeline)
	encoder.Send(sel("setBuffer:offset:atIndex:"), input, uintptr(0), uintptr(0))
	encoder.Send(sel("setBuffer:offset:atIndex:"), output, uintptr(0), uintptr(1))
	encoder.Send(sel("setBytes:length:atIndex:"), unsafe.Pointer(&params), unsafe.Sizeof(params), uintptr(2))
	groupWidth := m.pipeline.Send(sel("threadExecutionWidth"))
	if groupWidth == 0 {
		groupWidth = 1
	}
	encoder.Send(sel("dispatchThreads:threadsPerThreadgroup:"), metalSize{Width: uintptr(width), Height: uintptr(height), Depth: 1}, metalSize{Width: uintptr(groupWidth), Height: 1, Depth: 1})
	encoder.Send(sel("endEncoding"))
	command.Send(sel("commit"))
	command.Send(sel("waitUntilCompleted"))
	if command.Send(sel("status")) != 4 {
		return nil, nativeError("GPU command", command.Send(sel("error")))
	}
	pointer := objc.Send[unsafe.Pointer](output, sel("contents"))
	if pointer == nil {
		return nil, nativeError("read output", 0)
	}
	frame := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(frame.Pix, unsafe.Slice((*byte)(pointer), len(frame.Pix)))
	return frame, nil
}
