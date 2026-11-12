# 原生控件视觉与绘制优化 · 2026-11-12

本轮改动位于 Go `omniui/baseui`，不引入 React、WebView 或新语言。OmniBrowser 不在本轮范围内。视觉方向参考用户提供的截图与 [shadcn/ui](https://ui.shadcn.com/) 的中性色、表面层次和内容排版；没有复制其 TSX 实现，也没有使用其第三方图片或字体。

## 视觉变化

- 浅色画布与白色卡片分层；暗色画布、表面、嵌套面板使用不同亮度。辅助文字仍通过默认主题对比度测试。
- CardTitle 从 16/regular 提升为 18/bold；卡片增加轻微接触阴影，Secondary 卡片使用无描边的内嵌表面。
- Outline 按钮使用明确的表面色，输入框使用稳定的浅填充；焦点、错误、禁用和只读逻辑继续沿用原生事件系统。
- 滑块使用 6 DIP 轨道和 18 DIP 拇指，保持原有命中区域和键盘语义；进度条轨道在嵌套面板中保持可辨识。

`examples/control-showcase` 是由真实 Card、Chart、Select、Slider、Textarea、Progress、Switch 和 Button 组成的离屏页面，包含明暗主题及键盘状态。测试实际输入文本、调整滑块、触发保存，并检查布局没有越出画布。它不是静态绘图替代控件，也不是新增原生窗口宿主。

```sh
CGO_ENABLED=0 go run ./examples/control-showcase
CGO_ENABLED=0 go run ./examples/studio/cmd/gallery -controls button,input,slider,card,tabs,dialog -scale 2 -out build/visual-refresh/after
```

截图：`build/control-showcase/{light,dark}-{default,keyboard}.png`。同一组合页面的旧样式位于 `build/visual-refresh/showcase-before`；既有 Studio 演示的前后截图位于 `build/visual-refresh/{before,after}`。这些是当前机器上的本地产物，不是 golet 同状态截图，也不证明审美上的量化倍数。

## 渲染变化及正确性

1. 不透明圆角矩形的完全覆盖区域按整行复制；边缘仍走原有抗锯齿路径。
2. 空心边框跳过确定不会绘制的内部矩形，减少大卡片无效扫描。
3. 默认字体按当前高 DPI 比例缓存，切换比例释放上一组，关闭 Surface 释放全部缓存。调用者提供的字体不转移所有权。

不缓存最终画面，不跳过事件，不隐藏动画，也不更改 Render 返回独立图像的约定。全部状态仍归 UI goroutine。

`render_reuse_test.go` 使用原始标量栅格器作为逐像素判据，覆盖透明混合、分数裁切、0.75/1/1.25/2/3 倍缩放、大/小圆角、不同边框宽度及输出所有权；另验证字体复用、缩放切换和关闭释放。

## 测量边界

基线来自本轮修改前的实现（仓库基线 `75e12a95deb11e289cc146e8a055aff4d357c91b`），以临时 Go overlay 加载旧 paint.go、compose.go 与 tokens.json，没有回退工作区或修改参考仓库。工作负载为既有 640×480、八行标签/按钮/徽标的真实绘制基准；新旧运行相同场景，视觉默认值按各自版本生效。

机器为 Apple M5 / darwin arm64，CGO_ENABLED=0，GOMAXPROCS=1。最终复测在本轮 race 检查结束后运行，每项 300ms、五次；共享机器仍可能受其他进程影响。原始记录在 `build/visual-refresh/{baseline,optimized}-settled.txt`。较早与 race 同时运行、波动较大的记录不用于最终比较。

```sh
GOMAXPROCS=1 CGO_ENABLED=0 go test ./omniui/baseui -run '^$' -bench BenchmarkControlsRender -benchtime=300ms -benchmem -count=5
```

这些数据只比较 OmniGo 修改前后，**不是与 golet 的等价跨框架基准**。本地 golet 使用场景指令绘制管线，不能用场景构建耗时与 OmniGo 完整像素渲染直接相除。没有证据支持“超过 golet 100 倍”；本轮不作此承诺，也不升级任何控件的参考覆盖状态。

最终五次测量的中位数（括号为最小–最大值，毫秒/帧）：

| 场景 | 修改前 | 修改后 | 中位耗时减少 |
| --- | ---: | ---: | ---: |
| 1×，新建帧 | 4.60（3.92–10.50） | 1.43（1.41–1.49） | 68.8% |
| 2×，新建帧 | 15.29（14.84–16.09） | 10.92（5.77–26.12） | 28.6% |
| 2×，复用帧 | 14.92（14.63–15.60） | 6.77（5.00–9.49） | 54.6% |

2×复用帧的每帧分配从 153 次降为 123 次，约 35.5 KB 降为 13.9 KB。**2×时间样本仍有显著波动，分布存在重叠，以上仅是本机观测值，不是稳定加速比或生产延迟保证。** 新图像 API 仍必须分配输出位图，不能把复用帧的分配量推广到全部渲染入口。

## 检查结果

- baseui、loading、theme、组合页的测试通过；相关 race 检查及 Studio 演示巡检通过。
- `go vet ./...`、OmniWeb SDK 四项测试、六个桌面目标的 CGO_ENABLED=0 编译通过。
- 已执行 `sh scripts/check.sh`；其全量测试阶段被另一程序正在修改的 OmniBrowser 测试失败中断。没有修改该组件；其余检查单独执行。交叉编译不代表真实窗口兼容性验收。
- 完整 golet 等价性能基准、原生窗口逐状态截图以及主观视觉胜出评价仍未完成。既有功能覆盖审计保持原状。
