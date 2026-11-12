# 可选 GPU 渲染

`gpu.Open()` 在 macOS Intel/ARM64 创建 Metal 设备、command queue 和 compute pipeline；`Render` 在 GPU 上执行有序矩形填充与 alpha 合成，并读回独立 RGBA 帧；`Close` 释放原生资源。默认 baseui 不导入此包，未使用 GPU 的应用没有该依赖链。

```go
d, err := gpu.Open()
// Handle err; defer d.Close().
frame, err := d.Render(640, 480, color.NRGBA{A:255}, []gpu.Rect{
    {X:20, Y:20, Width:100, Height:60, Color:color.NRGBA{R:255,A:128}},
})
```

所有宿主代码使用 Go/purego，`CGO_ENABLED=0`。Metal 原生着色器以内嵌字符串提供，由系统运行时编译；不需要 Python、Node、C 编译器或独立着色器构建步骤。没有复制 golet 的 GPU 实现或着色器。

已在 macOS ARM64 的真实 Metal 设备验证背景、覆盖、透明度混合、边界裁剪和像素读回。运行：

```sh
OMNIGO_NATIVE_TEST=1 CGO_ENABLED=0 go test ./omniui/baseui/gpu -run TestMetalNativePixels -v
```

这是 GPU 的首条实际运行路径，**不是完整 baseui GPU renderer**。目前没有文本、曲线、抗锯齿、阴影/模糊、纹理/损伤追踪、surface 直呈、异步帧同步、设备丢失恢复，也没有 D3D/OpenGL/浏览器后端；其他平台明确返回 ErrUnavailable，不把 CPU 回退标为 GPU。整数尺寸、有限几何与单帧工作量受预算限制。Intel 只交叉编译，待原生设备验收；没有声明性能倍数。
