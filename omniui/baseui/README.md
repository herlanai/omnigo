# OmniUI / baseui

**控件只用 Go 实现，统一放在 `omniui/baseui`。** 本包不包含 TSX、React 控件、HTML/CSS 解释器或 WebView 运行时。已有 OmniWeb 默认 Web 项目继续独立工作，但不计入 Go 控件覆盖率。

这里开始重写本地参考的 `ui`、`ui/forma` 和控件插件能力，接口、布局与绘制代码为新实现，不是对旧库的导入包装。已逐项阅读基础输入、Forma 主题/导航/日历/弹层/数据组件，以及 loading、borderbeam、glass 的接口与实现边界。

## 已实现的运行链路

Go 构造 Element → Mount 校验并快照 → Row/Column 布局 → 指针/键盘命中与焦点 → 控件状态/回调 → Go CPU 绘制。`Surface.Accessibility()` 输出角色、名称、值、几何与状态，供未来平台无障碍适配器使用；目前尚未接入系统屏幕阅读器。

```go
import "omnigo.dev/omnigo/omniui/baseui"

name := baseui.NewTextBuffer("OmniGo")
enabled := true
build := func() *baseui.Element {
    return baseui.Card("settings",
        baseui.Field("name-field", "Name",
            baseui.Input("name", "Name", name), "", ""),
        baseui.Switch("enabled", "Enabled", enabled,
            func(value bool) { enabled = value }),
        baseui.Button("save", "Save", func() { save(name.String()) }),
    )
}
surface, err := baseui.NewSurface(640, 480, baseui.NewTheme(false))
// Handle err; defer surface.Close().
err = surface.Mount(build())
// The platform driver forwards input, then remounts build() when state changes.
// surface.Pointer(...), surface.Key(...), surface.TextInput(...)
image, err := surface.Render()
```

此示例中的 `save` 为应用函数。所有 Surface、TextBuffer 和控件状态由同一 UI goroutine 使用；后台任务应通过已有 `runtime/dispatch` 回到 UI 线程。回调在调用线程同步运行，panic 转为错误。应用必须处理返回错误，状态改变后重建并 Mount；本阶段没有自动响应式订阅。

## 控件和行为

- 基础：Text、Row、Column、Card、Button、Badge、Separator、Canvas。
- 输入：Input、Textarea、Checkbox/混合态、Switch、RadioGroup、ToggleGroup、Slider、Stepper、Field、Form、InputGroup。
- 导航与组合：Tabs、Accordion、Collapsible、Pagination、Select、Carousel。
- 数据与反馈：Calendar 单选/多选/范围、固定窗口 Table、Progress、Meter、Alert、Empty、CodeBlock、Kbd、Dialog、AlertDialog。
- 可选 Go 插件：`loading` 的 Ring、Dots、Bars。使用相同 Painter、主题、虚拟时钟和减少动态效果策略。

这些入口是**局部重写成果，不是完整对齐声明**。Select 当前为内联展开，Table 只有显式可见行窗口与排序回调，CodeBlock 不含高亮/选择/滚动，Carousel 不含手势；日历不含本地化文本与 RTL；Tabs 等分组尚未完成全部 roving-tabindex 语义。尚未实现的菜单、弹出层、编辑器、富文本、图表、AI 组合及全部插件见 [覆盖清单](../../migration/controls/README.md)。

## 统一样式

`theme/tokens.json` 仅保存设计数据，由 Go 读取：14 DIP 正文、36 DIP 基础控件高度、8 DIP 圆角、4 DIP 间距单位，以及明暗主题的语义颜色。控件共用边框、焦点、禁用、选中、危险和表单错误表现。没有逐组件独立 CSS。

默认文本对比度测试检查正文、辅助文字、主按钮文字与错误文字。自定义主题不自动保证对比度；颜色可通过返回的 Theme 值覆盖。字体绘制使用 `golang.org/x/image` 的纯 Go 字体栈，默认 Go Regular 字体主要支持 Latin；CJK 需要调用方通过 `NewSurfaceWithFont` 提供合适字体。复杂文字整形、字素簇导航、系统字体回退和平台 IME 连接仍待实现。

## 验证与展示

```sh
go test ./omniui/baseui/...
go test -race ./omniui/baseui/...
go run ./examples/baseui-gallery
```

画廊生成 `build/baseui-gallery/light.png` 和 `dark.png`。它们由上述 Go 控件实际绘制，不是网页截图或效果设计图。当前示例为离屏展示，没有打开原生窗口。

固定时钟、无 sleep 测试覆盖点击取消、禁用继承、Space 单次触发、焦点隔离/返回、Unicode 编辑和撤销、IME 组合/提交、只读、滑块边界、日历键盘跨月、可见行窗口、剪裁、绘制结果所有权和动画暂停。默认没有周期轮询；驱动可在 `NeedsAnimation()` 为 true 时请求下一帧。

## 仍然缺失

真实窗口、系统无障碍与 IME、GPU、DPI 缩放、完整排版、滚动/布局算法、全部控件和插件尚未完成。CPU 绘制只支持当前基础形状和文字。没有证据证明性能优于参考框架，也没有把接口数量当作功能完成数量。
