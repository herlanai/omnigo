# Borderbeam 与 Studio 对齐记录

2026-11-12。状态：partial。

只读参考：本地 golet 的 `backend/plugins/borderbeam/README.md`、公开 Options 与测试，以及 `backend/examples/studio/borderbeam.go`。没有复制参考渲染器实现体、生成表或引入 golet 依赖。本实现以独立的椭圆色场、圆角距离函数和边缘衰减重写。

## 已实现

- 可选实现移至 `omniui/baseui/borderbeam`；原 `omniui/plugins/borderbeam` 仅保留 OmniGo 自身 API 的兼容导出。
- Medium/Small 为不同遮罩的旋转色场与内辉光；Line 为底边移动光束和向上衰减的光柱；PulseInner 为分区呼吸内光，PulseOutside 为轮廓呼吸与两层外部衰减。
- 默认周期分别为 1.96、1.96、3.1、2.3、2.3 秒。提供 Theme/Colorful/Ocean/Sunset/Mono、复制后的自定义色板、Strength/Glow/Width/Radius、暂停、隐藏、减少动态效果。
- 外发光可越过自身边界，仍受祖先卡片、滚动视口和窗口边界裁剪。装饰不增加命中区域，不拦截子控件输入。
- 当前效果纹理复用，暂停时命中同帧缓存；主题色、尺寸、模式、范围等变化触发更新。单块纹理限制约 120 万像素，不缓存整段动画，不启动 goroutine。Disabled 释放纹理；状态本身由 UI goroutine 持有。
- Studio 使用独立 960×820 演示页、28 DIP 外边距、20 DIP 主间距，五个模式按钮、暂停/隐藏开关、三个滑块、主题主卡片及 Prism/Ocean/Sunset/Aurora 四张配色卡片。模式切换恢复对应默认周期，全部卡片共享配置且各自持有时钟/缓存。

## 验证

`omniui/baseui/borderbeam/borderbeam_test.go` 覆盖五种模式与明暗主题的动画变化、暂停缓存、颜色所有权、隐藏释放、减少动态效果，以及内/外光的不同像素范围。原插件交互测试继续验证子按钮可操作。Studio 确定事件测试覆盖五卡片入口、周期切换及暂停/隐藏同步。

固定时钟截图：

```sh
CGO_ENABLED=0 go run ./examples/studio/cmd/gallery -controls plugin/borderbeam -beam-preset 4 -scale 1.25 -out build/borderbeam-alignment/preset-4
```

`-beam-preset` 接受 0～4。已生成五组明暗截图，检查了 Medium 与 PulseOutside 的暗色效果。完整检查使用 `sh scripts/check.sh`，构建使用 `go run ./scripts/devtool build`。

## 尚未完全一致

当前采用独立色场系数与解析衰减，未复刻参考生成色斑表、高斯模糊和精确分层混合，因此不能称为与 golet 逐像素相同。参考的 StaticColors、Brightness、Saturation、HueRange、Mode 参数仍未实现，颜色周期目前与运动相位绑定；重绘使用框架调度而非插件独立的固定 30 FPS 节流。

Studio 保留对应布局、交互和英文标题，但说明文字目前为英文，未复制中文文案并解决 CJK 字体回退；模式按钮使用 OmniUI 当前主题。未进行参考原生窗口同状态截图或实机跨平台验收，也没有新的等价性能对比。上述缺口不因构建通过或已有同名模式而清除。
