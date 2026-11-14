# OmniUI / baseui

**控件只用 Go 实现，统一放在 `omniui/baseui`。** 本包不包含 TSX、React 控件、HTML/CSS 解释器或 WebView 运行时。已有 OmniWeb 默认 Web 项目继续独立工作，但不计入 Go 控件覆盖率。

这里开始重写本地参考的 `ui`、`ui/forma` 和控件插件能力，接口、布局与绘制代码为新实现，不是对旧库的导入包装。已逐项阅读基础输入、Forma 主题/导航/日历/弹层/数据组件，以及 loading、borderbeam、glass 的接口与实现边界。

## 已实现的运行链路

Go 构造 Element → Mount 校验并快照 → Row/Column 布局 → 指针/键盘命中与焦点 → 控件状态/回调 → Go CPU 绘制。`Surface.Accessibility()` 输出角色、名称、值、几何与状态，供平台无障碍适配器使用；macOS 宿主已接入 NSAccessibility 对象、操作和通知，VoiceOver 人工验收与其他平台适配仍待完成。

```go
import "github.com/herlanai/omnigo/omniui/baseui"

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

此示例中的 `save` 为应用函数。所有 Surface、TextBuffer 和控件状态由同一 UI goroutine 使用；后台任务应通过已有 `internal/runtime/dispatch` 回到 UI 线程。回调在调用线程同步运行，panic 转为错误。应用必须处理返回错误，状态改变后重建并 Mount；本阶段没有自动响应式订阅。

## 控件和行为

- 基础：Text、Row、Column、Card、Button、Badge、Separator、Canvas。
- 输入：Input、InputOTP、Textarea、Checkbox/混合态、Switch、RadioGroup、ToggleGroup、Slider、Stepper、Field、Form、InputGroup。
- 导航与组合：Tabs、Accordion、Collapsible、Pagination、锚定弹层 Select、Combobox、Command、DropdownMenu 多级菜单、ContextMenu、Popover、Tooltip、HoverCard、Sheet、Carousel。
- 布局：ScrollArea（滚轮/键盘/滚动条/焦点滚入）、Resizable（水平/垂直与取消恢复）、AspectRatio、Direction（行顺序与键盘方向）。Skeleton 使用显式时钟和减少动态效果策略。
- 数据与反馈：FileTree、Calendar 单选/多选/范围、固定窗口 Table、Progress、Meter、Alert、Empty、CodeBlock、Kbd、Dialog、AlertDialog。
- 可选 Go 插件：`omniui/baseui/loading` 的 Ring、Dots、Bars、Clock、Ripple、Orbit、Pulse。使用相同 Painter、主题、虚拟时钟和减少动态效果策略。

这些入口是**局部重写成果，不是完整对齐声明**。Select 已使用可滚动的锚定弹层并支持键盘与 typeahead，Table 只有显式可见行窗口与排序回调，CodeBlock 支持 Go 词法高亮，尚不含文字选择，Carousel 不含手势；日历不含本地化文本与 RTL；Tabs/RadioGroup 已使用单个 Tab 停靠点并支持方向键切换；其他组合的 roving-tabindex 语义仍待逐项验收。菜单栏、图表、AI 组合的剩余能力，以及尚未实现的编辑器、富文本与其他插件见 [覆盖清单](../../migration/controls/README.md)。

## 统一样式

InputGroup 共用外框及 focus-within，内部输入不重复绘制边框，并保留错误和文本域高度。`Separator(id, true)` 提供竖向分隔线；`Button(...).Variant(Link)` 提供透明链接样式和悬停下划线，保留按钮交互与禁用语义。此次全目录复查记录见 [审计](../../migration/controls/README.md)。

`theme/tokens.json` 仅保存设计数据，由 Go 读取：14 DIP 正文、36 DIP 基础控件高度、16 DIP 基础圆角（按钮/单行输入为胶囊，文本域为 12 DIP）、4 DIP 间距单位，以及参照 golet Forma 的中性明暗语义颜色。错误文字使用更深的浅色主题红色以维持对比度。控件共用边框、焦点、禁用、选中、危险和表单错误表现。没有逐组件独立 CSS。

默认文本对比度测试检查正文、辅助文字、主按钮文字与错误文字。自定义主题不自动保证对比度；颜色可通过返回的 Theme 值覆盖。字体绘制使用 `golang.org/x/image` 的纯 Go 字体栈，所有平台默认嵌入 Inter 4.1，使用真实 400/500/600/700 静态字重；富文本使用同族斜体，代码使用 Go Mono。Inter 的 OFL 许可证随资源与 Studio 分发。Inter 字形覆盖 Latin/Greek/Cyrillic，缺字时延迟使用已安装的系统字体（见下文“系统字体回退”）。没有候选字体时可通过 `NewSurfaceWithFont` 提供字体。复杂文字整形、字素簇导航和完整平台 IME 连接仍待实现。

## 验证与展示

```sh
go test ./omniui/baseui/...
go test -race ./omniui/baseui/...
go run ./examples/baseui-gallery
```

画廊生成 `build/baseui-gallery/light.png` 和 `dark.png`。它们由上述 Go 控件实际绘制，不是网页截图或效果设计图。当前示例为离屏展示，没有打开原生窗口。

固定时钟、无 sleep 测试覆盖点击取消、禁用继承、Space 单次触发、焦点隔离/返回、Unicode 编辑和撤销、IME 组合/提交、只读、滑块边界、日历键盘跨月、可见行窗口、剪裁、绘制结果所有权和动画暂停。默认没有周期轮询；驱动可在 `NeedsAnimation()` 为 true 时请求下一帧。

## 仍然缺失

通用跨平台窗口、系统无障碍与 IME、GPU、完整排版、虚拟滚动/惯性/完整布局算法、全部控件和插件尚未完成。Studio 已提供 macOS arm64 实验宿主，可实际打开并交互现有 Go 控件；见 [Studio](../../examples/studio/README.md)。CPU 绘制只支持当前基础形状和文字。没有证据证明性能优于参考框架，也没有把接口数量当作功能完成数量。

## 高 DPI 绘制与当前视觉边界

`RenderScaled(scale, face)` 按物理像素重新光栅化形状和文字；布局、命中测试与无障碍坐标始终为 DIP。支持非整数缩放，限制单帧像素预算。默认字体自动缩放；自定义字体在非 1 倍时必须显式传入对应像素字号的 face，调用方保留其所有权。绘制返回独立图像，不能保留 Painter。

Painter 的 Fill、Stroke 和 Line 使用覆盖率抗锯齿和正确的预乘 alpha 合成。Stroke 不涂覆内部；单条半透明线不再因多次采样叠加而变粗。按钮、输入、复选、单选、开关、滑块和焦点轮廓已调整；输入支持 Placeholder，Field 区分辅助文字与错误文字。

这不是完整 Forma 视觉对齐：字体字重/字号分级、完整尺寸变体、阴影、外扩 focus-visible、RTL、高对比及所有高级组件仍需实现和逐项验证。离屏截图与像素测试不能替代真实屏幕验收。

## 滚动、弹层和新增交互

```go
scroll := &baseui.ScrollState{}
menu := &baseui.MenuState{}
search := &baseui.ComboboxState{}
split := &baseui.SplitState{Fraction: .5, MinFirst: 100, MinSecond: 100}
```

这些状态由调用方持有并在 UI goroutine 上跨构建复用。ScrollArea 在自身视口内剪裁，支持双轴滚轮、可选水平滚动、滚动条拖动、Home/End/PageUp/PageDown；焦点可导航到屏外子项并自动滚入，内容缩小时会钳制偏移。`Surface.Wheel` 接受 DIP 增量；Studio 已转发 AppKit 的滚轮/触控板事件。普通 ScrollArea **不虚拟化**，不会声称大量子节点只物化可见项。

Popover 在常规内容布局之后统一定位/绘制，超出窗口时翻转并钳制；点击外部关闭会消费该次点击，Escape 关闭最上层并恢复此前焦点。Popup 不受普通父容器剪裁；模态 Dialog/AlertDialog 仍独立保留行为。AlertDialog 的背景点击不会关闭。IME 组合期间的 Escape 先取消组合，再次 Escape 才关闭弹层。

DropdownMenu 支持禁用、分隔符、复选项、子菜单、方向键、Home/End、显式时钟 typeahead；快捷键字符串仅作提示，不自动注册全局快捷键。Select 在打开时聚焦并滚入选中项。Combobox 保持搜索输入焦点，提交 IME 后过滤，仅选择结果时提交 Value；Command 支持标签/分组/关键字搜索与动作回调。

当前限制仍包括：菜单悬停意图/延迟、触摸与手势、菜单栏的完整 pointer 行为、完整弹层碰撞策略和阴影、系统无障碍适配器；Direction 不等于 Unicode 双向文字整形；Resizable 尚无吸附/折叠/持久化；Sheet 无拖动关闭动画。它们均为部分实现。

新增交互截图可复现：

```sh
go run ./examples/studio/cmd/gallery -scale 2
```

输出 `build/baseui-interactions/` 中的明暗图片，来自实际 Go 控件和事件，而非静态设计图。

## 悬停、右键、验证码与文件树

- `Tooltip(id, anchorID, text)` 默认指针延迟 500ms，键盘焦点立即显示；`HoverCard` 默认显示延迟 250ms、关闭延迟 200ms，指针或焦点进入面板后保持打开。`HoverDelay` 可覆盖延迟。两者不抢占焦点、不锁住 Tab，Escape 关闭后须离开触发区域才重新显示；卸载或禁用锚点会清理定时状态。
- 宿主通过 `Surface.NextWakeup()` 获取一次性唤醒延迟，再调用 `Advance` 和 Render。静止提示不轮询；ReduceMotion 不禁用行为延迟。输入事件应先把 Surface 时钟推进到事件时间，避免空闲后的提示立即过期。Studio 已接入此调度和鼠标离开事件。
- `ContextMenu(id, anchor, state, items)` 复用菜单的行为与样式，支持次键点击、Shift+F10、指针位置/键盘锚定以及焦点恢复；不会覆盖 anchor 的原有点击行为。
- `InputOTP` 使用一个 TextBuffer 绘制 1–32 个数字格。输入/粘贴/IME 提交过滤为 ASCII 数字，支持选择替换、撤销重做、格子命中、只读与禁用；初始内容和历史必须满足数字与长度限制。不会触发网络验证或读取短信。
- `CopySelection` / `CutSelection` / `Paste` 是平台适配器使用的 Go 编辑接口；Studio macOS 已连接系统文本剪贴板。复制不导出 IME 预编辑文本，剪切和粘贴遵守只读/禁用约束。原生 IME 协议仍未连接。
- `FileTree` 接收唯一 ID 的文件元数据，只显示展开分支。支持父子导航、Home/End、跳过禁用项、typeahead、单个 Tab 停靠点与焦点滚入。文件打开完全由回调决定，不自行读磁盘。层级深度限制 64，节点数限制 100,000；它还不是虚拟树，也不包含拖放、重命名或多选。

文件树箭头、子菜单箭头和菜单勾选使用 Go 几何绘制，不依赖默认字体是否包含这些符号。新控件仍未完成系统辅助技术、触摸/长按、完整排版和真实窗口全状态验收。

## 消息、工作流、导航、图表和通知

新增的 37 个目录项都有 Go 实现及 Studio 示例；审计状态为 **86 partial / 0 pending**，完整参考对齐仍为 **0**。每项缺口见 `migration/controls/status.json` 的 `remaining`。

- `MessageScroller`/`Chat` 使用调用方保存的 `ScrollState`。追加内容仅在原来处于底部时自动跟随；用户上滚后保留位置。`Text.Wrap(true)` 支持纯文本折行，尚非 Unicode 字素/双向排版或 Markdown。
- `AIMessage` 明确区分用户/助手动作，streaming 时禁用；所有复制、编辑、重试回调由应用处理。`PromptInputState.Text` 使用 `TextBuffer`，提交为文本、模型、effort、附件元数据的独立快照。控件不自动清空输入，不启动上传/网络；应用回调后更新状态并 Mount。IME preedit 不提交，Shift+Enter 换行。
- `ModelSelectorState` 同时保存模型、effort 和弹层状态；切换模型时保留被支持的 effort，否则回退首个可用值。`Confirmation` 直到调用方重置前只接收一次决定；问卷校验必填/禁用选项并防重复提交。
- `Plan`、`Task`、`Queue`、`ChainOfThought`、`Reasoning`、`Tool`、`Terminal`、`Subagent`、`StackTrace`、`TestResults` 和 `Trace` 显示应用提供的数据。终端是只读日志；推理摘要是应用给出的公开文本；均不获取模型内部内容或执行任务。DisclosureState/QueueState/TestResultsState 保持独立展开状态。
- `Sources` 去重 URL，仅 http/https、非空主机且不含用户凭据的链接可调用 Open；`InlineCitation` 复用 HoverCard。`Context` 遇到未知 limit 显示未知，不编造百分比。`Avatar` 复制图片并圆形剪裁，默认 initials；无图片 IO。
- `NavigationMenu`/`Sidebar` 使用 roving focus，方向键只移动焦点；`Breadcrumb` 最后一项是当前页；`Menubar` 支持键盘切换打开的顶级菜单、子菜单与 Escape。鼠标跨菜单切换/hover intent 等仍未完成。
- `Chart` 支持柱、直线、面积，非有限/缺失值断开，域包含零；`ChartState.Selected=-1` 表示未选，方向/Home/End/Escape 与指针选择共用状态。输入数据被快照；坐标范围采用归一化避免有限极值相减溢出。主题提供五个图表语义色。高级曲线/轴/提示/虚拟化仍待实现。
- `Drawer` 仅在手柄上拖动，达到窗口高度四分之一关闭；取消/卸载/只读清理拖动；支持 Escape/外部关闭与焦点作用域。触摸惯性/过渡未完成。
- `ToastManager.Add` 支持按 ID 更新，Duration=0 常驻；`Sonner` 连接 `Surface.NextWakeup`，没有后台 goroutine。hover/focus 暂停倒计时，`Surface.SetActive(false)` 暂停后台窗口；应先 Advance 到事件时刻再改变激活状态。卸载/关闭不再调度。通知动作只调用一次；应用自行处理动作副作用。完整 placement/堆叠/动画/系统 live-region 未完成。

这些实现复用同一 Go 布局、主题、焦点和事件系统；不引入额外依赖。新增截图示例：

```sh
go run ./examples/studio/cmd/gallery -controls ai-message,prompt-input,plan,chart,drawer,sonner,message-scroller -scale 2
```

### 图表与模型选择的参考行为增量

Chart 新增 `Label`、`Height`、`Curve`、`HideLegend`、`HideGrid`、`ShowYAxis`；默认启用图例/网格，与参考一致。旧 `Grid`/`Legend` 字段保留以兼容源码，关闭时请使用新的 Hide 字段。插值支持 `CurveLinear`、`CurveMonotone`、`CurveStepAfter`、`CurveStepBefore`、`CurveStep`；面积使用主题色透明渐变，数据绘制受 plot 剪裁。数值域排除无分类标签的多余值，支持整洁刻度、极值和缺失点分段。

指针悬停选择最近分类、离开清除；静止指针不会抢回键盘选择。未选中时 Left 选择最后一项；Home/End/Escape、禁用/只读和取消/卸载均有测试。选中提示显示分类和各有限数据值，使用 FormatValue；底部文本和可访问描述同步提供结果。其余底层 chart 能力及真实窗口辅助技术仍待实现。

ModelSelector 按 provider 首次出现顺序分组，显示描述并对缺失名称回退到 ID。新增可选 `ModelSelectorOptions{Placeholder, Disabled}`；动态禁用模型的 effort 不可编辑，空列表不打开弹层。重复选择相同值不触发回调、不重写 effort；切换模型才执行 effort 的保留/回退。

```sh
go run ./examples/studio/cmd/gallery -controls chart,model-selector -chart-kind 2 -chart-curve 1 -out build/baseui-parity -scale 2
```

`ChartContainer(id, label, axes, paint)` 提供四轴与 16:9 布局，回调收到已剪裁的逻辑 plot 矩形；不能保留 Painter。`ChartLegend` 快照标签/颜色并使用主题回退；`ChartTooltip` 接受任意 Go 子控件，复用弹层焦点/关闭行为。

### Terminal 命令输出与 CodeBlock

`Terminal(..., TerminalOptions{Open: &disclosure, OnCopy: copyText, MaxHeight: 320})` 支持应用持有的折叠状态和复制回调。运行中状态优先；未报告退出值时为 pending，非零退出为 error。输出使用 Go Mono 等宽字体、4 列 Tab 停靠和双向滚动，复制不改动原始文本。追加输出只在原来接近末尾时跟随。CodeBlock 默认也使用等宽滚动区；设置 `CodeBlockState.Wrap` 可换行，设置 `Language: "go"` 可复用编辑器词法高亮。复制保留原文。文字选择仍缺失；这两个控件都不是 PTY/VT 插件。

可选插件的规范位置已按用户要求迁为 `omniui/plugins`；其中 Loading 按最新要求合并，唯一实现与测试位于 `omniui/baseui/loading`，旧导入路径已移除。基础控件不会导入插件集合。`Element.PaintSurface` 为 Go 插件提供当前元素表面绘制回调（先于子节点）；`Painter.Snapshot` 返回受限、自有背景快照，`Painter.Image` 提供剪裁和圆角位图合成。回调仍只能在 UI goroutine 内使用，不能保留 Painter。

### Markdown 阅读控件

`omniui/plugins/markdown` 通过可选 Goldmark 解析器生成 Go 原生控件，支持标题、行内样式、列表、引用、代码块、GFM 表格和链接回调。详见该包 README。独立于原有 86 项目录，在 `migration/controls/advanced.json` 保留未完成能力；不等同于富文本编辑器。

## `ui` 基础控件缺项补充

新增 NumberInput、DateInput、TimeInput、SearchField、Rating、RangeSlider、TokenField、Toolbar、ColorPicker、ColorWell、Image、Grid、CheckboxGroup、EditableText 和 FindBar，全部在本包实现，已有控件保持复用。Studio 各有独立示例。所有状态及回调仅在 UI goroutine 使用；处理事件后重新 Mount。

- 数值输入保留未完成的草稿，仅有效范围内的数字更新值，Enter/Escape 归一化。失焦恢复有效值草稿且不重复通知；地区数字格式尚未实现。
- 日期选择保留时间、时区；时间的小时/分钟方向键循环而不改变日期，支持可选 AM/PM。地区格式、自动跳段仍缺失。
- 范围滑块提供共同轨道、两个焦点手柄、最近手柄命中、步进和取消恢复。刻度与完整 RTL 尚未完成。
- 颜色选择提供 HSV 方块、彩色色相轨道、透明度轨道、RGB/RGBA 十六进制和预设色。没有系统颜色面板或广色域管理。
- Image 接收调用方解码的位图并复制像素，支持 contain/cover/stretch；无隐式读取。Grid 是有限的等宽列布局，不是虚拟网格。
- EditableText 通过 Enter/F2 或按钮进入编辑，选中文件扩展名前的部分；Enter/Save 提交，Escape/Cancel 恢复。没有延迟重命名、双击或失焦提交。
- TokenField 支持标签添加/删除和最多 20 个过滤建议；Toolbar 支持单 Tab 停靠点的方向键导航。标签换行、弹层建议、工具栏溢出等仍缺失。

完整构造入口盘点为 `migration/controls/ui.json`，逐项能力见 `advanced.json`；名称映射仅说明已有代码位置，不是完整对齐。`ui_controls_test.go` 覆盖取消、IME、只读/禁用、范围、像素所有权、编辑历史及状态回调等边界。

### 矢量图标

`Icon`、`IconButton` 和 `Painter.Icon` 接收 `IconSource`，支持文字主题色、描边宽度、DPI 和按钮交互状态。纯 Go SVG 解析器位于 `omniui/icon`，固定版本 Lucide 资源位于 `omniui/icon/lucide`。基础包通过接口使用图标，不依赖解析器或完整图标库；见 [图标用法](../icon/README.md)。

### 共用视觉层级与控件尺寸

新增 `Heading`、`Label`、`FontSize`、`FontWeight`、`Muted`、`CornerRadius` 和 `Size`。标题/正文/说明使用不同字号与字重，文字测量、换行、输入点击命中和 DPI 绘制使用一致的字体指标。字体对象由 Surface 缓存并在 Close 释放；字号按半 DIP 量化。Inter 的 400/500/600/700 分别使用 Regular/Medium/SemiBold/Bold，不合成字重；调用方提供的自定义 face 保持原有字号和所有权，不会被默认字体替换。

按钮尺寸为 24/32/36/40 DIP；Badge 为 20 DIP、12 DIP 文字和内容宽度，不再在 Column 中拉伸。菜单项圆角统一，Tabs 使用低对比轨道与选中表面，表格增加左对齐标题、稳定行高和轻量交替行背景。Field、Dialog、Alert、Empty、Item、计划/问卷和代码标题已接入文字层级。

键盘导航显示外扩焦点环，普通鼠标点击不再套重焦点框；输入框保留当前编辑边框。外环允许进入布局容器的内边距，但受卡片/弹层/滚动视口边界约束。弹层通过纯 Go 的有界柔和阴影与背景区分；不保留 Painter。相关测试见 `visual_style_test.go`，完整离屏预览由 Studio gallery 生成。

这是共用视觉层的改进，不是全部控件与 golet 的逐像素对齐声明。自定义绘制控件、完整系统字体发现、全部状态/动画、原生窗口与系统无障碍仍需后续验收。


## Maia 演示视觉基线

按只读参考 `ui/forma/theme.go`、`inputs.go`、`layout.go`、`navigation.go` 的尺寸与状态规则调整：Card 外边距 24、区段间距 24，CardHeader 间距 8、标题 16/500、说明 14，CardContent 间距 16；Field 间距 8、Form 间距 24；Tabs 内容 30 + 上下各 3、条目圆角 12。新增 CardHeader/CardTitle/CardDescription/CardContent/CardFooter 组合入口。

Maia 输入和 Outline 使用半透明 input 底色；按钮各变体有独立悬停色，禁用按钮使用半透明绘制，保留底层背景。默认与高 DPI 字体、文字测量和富文本使用同一 Inter 解析字体集合，500 使用真正的 Medium；这不是完整 CoreText 文字整形；系统字体字形回退见下文。

Studio 采用参考的 1040×840 逻辑画布、32 外边距、24 主间距，统一卡片章节及可滚动演示区，输入/按钮/Badge 同屏展示更多状态。`gallery -controls all` 现在包含所有可运行 Go 控件与绘制插件；CEF 浏览器是独立宿主，不作为 CPU gallery 绘制。

## 精确小数与通用树形数据

`DecimalInput` 使用 `math/big.Int` 进行精确十进制运算，`Step` 的小数位数确定精度；支持可选上下限、合法千位分组、输入校验、方向键步进和 Enter/Escape 规范化草稿。输入法组合期间不步进，外部 Value 更新会替换草稿。失焦格式化与本地化小数分隔符仍待补充。

`TreeOutline[K]`（避开既有 `Outline` 按钮样式常量）和 `OutlineTable[K]` 通过 ID、Label、Children 回调接受业务数据，按键保持展开和选择状态。支持父子与同级键盘导航、typeahead、禁用继承、独立展开按钮和激活回调；表格支持固定表头、固定/弹性列宽与自定义 Go 单元格。设置 OutlineOptions.RowHeight（至少 32 DIP）可启用固定行高虚拟化，Height 默认 280；自定义单元格必须适配该高度。仍需遍历展开层级以维护键盘导航；多选、拖拽和列排序/调整宽度仍未实现。

## 可复用的 CPU 帧缓冲

持续绘制的原生宿主可用 `Surface.RenderScaledInto(dst, scale, face)` 复用调用方拥有的 RGBA 图像。目标必须从 (0,0) 开始、紧密排列，尺寸为 Surface 逻辑尺寸乘 scale 向上取整；每帧会完整重绘背景。须在上一个消费者读取完毕后、在 UI goroutine 调用，Surface 不保留目标；绘制回调报错时目标可能只完成部分绘制。需要持久快照时继续使用 Render/RenderScaled，二者仍返回独立图像。实测边界见 [性能记录](../../docs/performance.md)。

## 剩余高级控件的初步重写

- `VirtualList` / `GridView`：固定行高、仅构建可见窗口与 overscan、稳定业务键、键盘 Home/End/Page 与二维导航。滚动事件后需要重新构建并 Mount；GridView 的 Width 由宿主提供，变化时重建。可变行高、多选、拖拽、插入锚定仍缺失。
- `CodeEditor`：Go 语法着色、等宽字体、行号、Tab 缩进、换行继承缩进、上下行导航、查找、文本历史与 IME 组合隔离。Ctrl/Cmd+Tab 离开编辑区。大文件增量存储/布局、补全、折叠、多光标和 LSP 未实现。
- `RichTextEditor`：独立可编辑样式文档模型，选区粗体/斜体/代码/删除线，文字与格式共同撤销/重做，实时富文本预览。目前为文字编辑加预览，不是完整内联所见即所得块编辑器。
- `Timeline` / `AnimationTimeline`：显式单调时钟、暂停、seek、延迟、重复、交替、反向和共享播放头补间。`Surface.Advance` 驱动；不可见、停用或卸载后不计入暂停时间，不创建后台 goroutine。标签定位、完整 easing/spring 和 arrival 回调尚未实现。
- `Video`：独立帧像素所有权、时长播放、暂停/seek/循环、GIF 帧合成以及可取消的 VideoDecoder 加载接口。总解码像素限制 64 MiB。加载应在后台完成，再经 dispatch 交给 UI；当前预解码整个片段，没有音频、流式 decode-ahead 或通用编解码器。
- `WebView`：只承载明确创建的原生网页视图，其他控件仍由 Go 绘制。宿主实现 NativeWebView，调用 Surface.SyncNativeViews 应用布局和裁切，创建者负责最终释放，Surface 卸载/关闭负责隐藏。Studio macOS arm64 使用 WKWebView；其他平台、完整弹层合成、圆角/变换裁切与 reusable 导航桥仍待补齐。

新增行为测试见 `rewrite_controls_test.go`。真实 WebKit 加载、JS 读取与卸载验证：`CGO_ENABLED=0 go run ./examples/studio/cmd/webviewcheck`（macOS arm64，会打开一个窗口并自动关闭）。CPU gallery 不冒充原生 WebView 的渲染验证。

编辑器辅助控件另有 `DiffView`（逐行比较、切换视图、还原/撤销）、`GoToLine`（行号定位校验）和 `CompletionMenu`（候选过滤及快照校验插入）。状态由 UI goroutine 持有。完整缺口见迁移审计，语言服务、snippet、差异批注/补丁导入尚未实现。

## 表面层次与栅格加速（2026-11-12）

圆角填充批量写入完全覆盖像素，边框跳过空心内部；高 DPI 字体按当前比例复用并在切换或关闭时释放。原有独立图像所有权和 UI goroutine 约束不变。视觉基线已进一步采用下述 shadcn 预设：CardTitle 为 16 DIP/500，卡片保留轻边框。

可运行 `go run ./examples/control-showcase` 查看真实 Go 控件组合的明暗与键盘状态截图。[视觉与性能记录](../../migration/controls/visual-performance.md) 说明基准、像素一致性测试和剩余限制；不宣称相对 golet 的 100 倍提升。

## shadcn 样式与组合控件

默认样式对应用户指定预设 `bbVJxYW` 的 Maia / Inter / Neutral。`ThemePreset(dark, StyleMaia, AccentNeutral, false)` 可选择 Maia/Nova/Lyra、Neutral/Blue/Violet 以及菜单的强调选中背景。`surface.SetTheme(theme)` 必须在 UI goroutine 调用，保留输入缓冲、选择和焦点，取消按压并释放旧字体缓存；自定义 face 的生命周期仍由调用方管理。Studio 演示顶部提供三项实时切换。

`DatePicker` 将 Calendar 与 Popover 组合，支持单日/范围选择、禁用日期、键盘打开、取消和焦点恢复。初始显示月份由调用方提供，不读取系统时钟。`DataTable` 支持文本筛选、稳定文本排序、分页、列显隐及跨页选择；每页最多 100 行、数据源最多 10,000 行，不隐式推断数字排序，也不是服务端表格。

完整的 66 项 shadcn 目录盘点及未实现行为见 [shadcn 审计](../../migration/controls/shadcn.md)。这次覆盖共享绘制层并补充上述组合控件，不表示所有变体和平台行为已逐项对齐。

## 系统字体回退

默认 Inter 普通文本、各字重和富文本在缺字时延迟使用已安装的系统字体。macOS 查找 Arial Unicode / Hiragino Sans GB，Windows 查找微软雅黑 / 明体，Linux 查找常见路径的 Noto Sans CJK。测量与绘制共用字形选择，跨字体不应用 kerning；高 DPI 创建对应字号的字形。Latin 文本不会触发系统字体文件读取，字体解析在进程内共享，每个 face 的光栅资源随 Surface 关闭释放。系统字体不会复制或打包进应用。

此前“默认只支持 Latin、必须提供自定义 face 才能显示中文”的说明由此更新。缺少候选字体的机器仍需安装字体或提供自定义 face；这不是完整的系统字体发现、字重匹配、Unicode bidi 或复杂文字整形。macOS 已测试简繁中文、日文字形及 1×/2× 字号；Windows/Linux 候选路径尚待原生机器验证。

Markdown 共享的 `RichTextOptions` 新增 `LineHeight`（字号倍率）、`Align`（逐行左/中/右对齐）、`NoWrap`（为横向滚动测量完整行宽）及 `PreserveWhitespace`（保留代码缩进）。布局、绘制及链接指针命中使用相同的对齐后坐标，默认行为保持不变。


## 原生文本与增量状态（2026-11-14）

macOS Go 内容窗口通过 `NSTextInputClient` 处理 marked text、UTF-16 范围、提交、
取消和候选框坐标。`Surface.NativeText`、`EditNativeText`、`NativeTextRect` 与
`NativeTextIndex` 供 UI 线程上的原生宿主使用；普通应用仍使用 Input/Textarea。
输入法草稿是虚拟文档的一部分，提交前不进入文本缓冲和撤销历史。

`Surface.Accessibility` 的父节点与能力信息映射到 macOS 原生对象；按钮操作、
滑块调整、文本赋值/选择遵循现有控件校验。对象 ID 在挂载之间稳定，卸载后操作
失效。原生协议探针见 `examples/studio/cmd/inputcheck`。这不代表所有控件已通过
人工 VoiceOver 导航、朗读和通知验收，Windows UIA/Linux AT-SPI 不在此实现内。

CodeEditor 对 Go 代码维护逐行词法状态，编辑只重新分析受影响区域，跨行注释与
原始字符串状态收敛后复用后缀。修改时仍拆分/复制行索引，最坏情况下仍需分析
整个后缀；不是常数时间编辑，也不含语言服务器。

树形控件可显式启用缓存：

```go
options := ui.OutlineOptions[string]{
    Incremental: true, Revision: modelVersion,
    ID: id, Label: label, Children: children,
    RowHeight: 32, Height: 320,
}
// 修改某个节点的孩子顺序或内容后：
state.Invalidate(parentKey)
// 替换根集合后：提高 Revision，或调用 state.Invalidate()。
```

展开状态 `Open` 自动追踪。局部失效仅获取对应子树；普通重建复用现有索引。
结构变化时仍复制扁平索引并验证唯一性，以保护旧挂载树的回调快照。
不开启 Incremental 时保持原来的自动全量读取行为。
