# Go 控件完整重写审计

## Terminal 演示对齐（2026-11-12）

只读核对 `backend/examples/studio/ai_patterns.go` 与 `backend/ui/forma/{ai_trace,code_block,code_block_icons}.go`。演示改用相同的 `go test ./...` 命令、成功退出状态和两行 `ok  example/backend` / `ok  example/ui` 输出，删除原有额外四态切换栏。控件改为无外层卡片的折叠轨迹、普通状态文字、24 DIP 输出缩进、灰底轻边框输出块、12 DIP 标题与正文、19.5 DIP 行距和图形复制按钮。保留取消、禁用/只读、原字节复制、双轴滚动与跟随行为。

`terminal_test.go` 新增短输出无滚动/无裁切、行距、缩进和多 DPI 回归；明暗截图位于 `build/terminal-alignment`。这仍不是逐像素相同的证明：参考 macOS 使用 CoreText 系统等宽字体，本实现使用纯 Go 的 Go Mono；复制后的定时反馈、文本选择、系统文字栅格化和完整平台交互还未对齐。未运行参考原生窗口同状态截图比较，状态保持 partial。

新增 [shadcn 对齐记录](shadcn.md) 与 [66 项组件盘点](shadcn.json)：Inter 字体、Maia/Nova/Lyra 主题切换、选中状态以及 DatePicker/DataTable 的实现与剩余缺口。原有参考能力清单继续保留。

## 全目录复查（2026-11-11）

本轮覆盖全部 86 个目录控件的参考入口映射、明暗离屏总览和共用交互巡检，逐项索引见 [review.json](review.json)。该索引保留既有缺口；入口映射不是逐行为等价证明，未逐项验证的能力明确保持未验收。

`examples/studio/internal/demos/sweep_test.go` 对所有演示在 320/720 宽度、明暗主题下使用确定事件检查键盘、文本/IME、禁用、只读、取消及卸载计时清理。控件专项回归见 `omniui/baseui/review_regression_test.go`：InputGroup 单一边框/焦点、输入命中、错误和高文本域；竖向 Separator；Link 按钮；来源空 URL/标题回退；消息作者末端对齐及空作者隐藏。

实际修复还包括 Item/Marker 垂直居中、Alert 危险样式和 Context 未知上限提示。字体层级、完整尺寸/视觉状态、ButtonGroup 连续边框、富文本/虚拟化及原生输入和无障碍仍有缺口。当前保持 **86 partial / 0 pending / 0 完整对齐**。

全量截图可复现：`CGO_ENABLED=0 go run ./examples/studio/cmd/gallery -controls all -out build/baseui-review -scale 1`，生成 172 张控件截图与 12 张明暗联系表。截图经过总览检查，但尚无参考框架逐状态像素差异验收或真实窗口全状态验收。

唯一实现位置：**`omniui/baseui`**。包括控件行为、组合、主题与绘制，以及可选的 Go 控件插件。React/shadcn Web 模板不计入这里的完成量。

`reference.json` 包含从本地参考读取的：

- 65 个 Forma 基础目录项及 21 个 AI 组合目录项，共 86 项。
- 2,925 条 UI 相关 Go 导出声明：涵盖 `backend/ui` 的基础控件、修饰器、编辑器、富文本、Markdown、图表等，以及签名中引用 `ui` 的插件声明。
- 2,144 个 UI/插件测试、fuzz 和 benchmark 名称，用于后续逐行为映射。

声明数包含类型、辅助接口和条件编译变体，不是控件数；测试名称也不是已经移植的测试。插件内部未导出的行为、绘制细节、资源生命周期以及没有 `ui` 字样的入口仍须依据原包逐个核对。不能以 86 个目录项全部有函数名为完成标准。

重建只读快照：

```sh
go run ./tools/controlaudit -source /absolute/path/to/golet
```

`status.json` 逐项标记 **partial / pending**，列出当前 Go 实现入口。当前没有任何一项被标记为与原框架完全等价；新基础行为的测试不能替代整个控件的状态、平台与功能验收。

## 每项必须满足的完成条件

1. 原有 Go 控件的参数、状态、组合和插件行为均有明确映射；新 API 可不同，但不得省略能力。
2. 交互覆盖指针、键盘、禁用/只读、焦点、取消、粘贴、输入法、边界值与生命周期。
3. 统一样式覆盖明暗、尺寸、错误、选中、悬停、按压、焦点、减少动态效果和高对比场景；RTL、DPI 和平台差异单独验收。
4. 复杂控件覆盖虚拟化、选择、滚动、排序、列宽、编辑历史和流式更新等真实行为。
5. 插件覆盖资源所有权、暂停/恢复、隐藏时不请求帧、卸载与错误路径，不能把所有重依赖塞入默认链接路径。
6. 在真实窗口中验证最终输入、文字、绘制和无障碍结果，再比较等价负载性能。

目前已建立 Go 离屏绘制和行为测试链路，并新增 Studio 内部 macOS arm64 实验窗口宿主；尚无正式跨平台原生驱动。未完成目录项及高级能力必须继续保留，不能删除或用占位绘制标记完成。

## 本轮基础修复证据（2026-11-11）

- 共用绘制：分数像素剪裁、半透明线条、圆角空心边框与预乘 alpha；见 `omniui/baseui/paint_test.go`。
- 高 DPI：1/1.25/2/3 倍输出尺寸、逻辑坐标保持、点击命中、关闭及像素预算错误路径；macOS 宿主接入 backingScaleFactor。真实窗口/多屏效果未验收。
- 样式：中性明暗主题、胶囊按钮/输入、复选/单选/开关/滑块比例、圆角焦点轮廓和表单语义文字；画廊由实际 Go 绘制生成。
- 行为：输入占位提示不进入值；Tabs/RadioGroup 的单个 Tab 停靠点、禁用选择回退、方向键与反向 Tab，见 `state_test.go`。

以上证据仅覆盖列出的子能力。86 项状态继续保留为 partial/pending，不能据此升级为完整对齐。

## 滚动、弹层与布局增量（2026-11-11）

本批将 10 项由 pending 更新为 partial：aspect-ratio、combobox、command、direction、dropdown-menu、popover、resizable、scroll-area、sheet、skeleton。该批结束时为 **44 partial / 42 pending / 0 完整对齐**。Select/native-select 改为真正锚定的可滚动弹层，所有 86 项和高级能力继续保留。

- `overlay_test.go`：滚轮边界/嵌套冒泡、内容缩小、滚动条拖动取消、焦点滚入；弹层翻转、外部点击消费、嵌套焦点恢复；菜单复选/typeahead；Select 键盘选择；Combobox 过滤、IME、只读；Command 关键字与禁用动作。
- `layout_controls_test.go`：分栏最小尺寸、键盘、拖动取消、卸载后 ID 复用；比例容器、RTL 行序、四边 Sheet；骨架屏显式时钟/减少动态效果。
- Studio 新增对应演示，`examples/studio/cmd/gallery` 用真实事件生成明暗截图。AppKit 接入滚轮/触控板与悬停移动事件；尚未进行真实窗口的完整验收。

保留缺口：普通滚动未虚拟化/惯性化；菜单没有悬停意图、完整快捷键注册、触摸或平台无障碍；弹层没有完整阴影/碰撞/过渡策略；Direction 不覆盖双向整形；Resizable 没有吸附和折叠；其余 42 项、编辑器/富文本/图表/AI、可选插件及跨平台宿主仍待实现。测试通过不替代这些能力的验收。

## 悬停、输入与文件树增量（2026-11-11）

新增 tooltip、hover-card、context-menu、input-otp、file-tree 的实际 Go 行为及 Studio 演示，状态由 pending 改为 partial。当前总计 **49 partial / 37 pending / 0 完整对齐**。

- `hover_test.go` 覆盖精确显示/关闭时刻、键盘立即显示、Escape 抑制重开、跨间隙停留、面板焦点、禁用/隐藏/卸载/关闭清理、ReduceMotion、模态内的源顺序无关定位与点击不穿透。
- `otp_test.go` 覆盖数字过滤、长度、选择替换、整次粘贴撤销/重做、IME 提交、空组合不删选区、复制/剪切、只读/禁用、格子命中和非法初始内容。
- `context_menu_test.go` 覆盖次键不触发主操作、键盘/指针定位、焦点返回、禁用和保留调用方 anchor；`file_tree_test.go` 覆盖展开/折叠、父子键盘导航、禁用项跳过、元数据快照、输入定位、焦点滚入、只读和重复 ID。
- macOS 实验宿主新增一次性唤醒、次键/离开事件、文本剪贴板适配；系统 IME、屏幕阅读器、其他平台宿主及实机全状态验收仍未完成。

FileTree 仍非虚拟树；触摸长按、文件树拖放/重命名/多选、OTP 自动填充、完整提示排版和所有剩余高级能力均保留为缺口。几何图标和离屏截图不构成完整原生控件对齐证明。

## 剩余 37 个目录项的首批实现（2026-11-11）

本批为全部 37 个 pending 目录项加入 Go 行为/状态、绘制与可运行 Studio 示例，改为 **partial**。当前为 **86 partial / 0 pending / 0 完整对齐**。这表示目录中的各项均已开始实现，**不表示全量重写完成或达到 100%**；`status.json` 为本批每项保留 `remaining` 能力缺口。此前 49 项的高级缺口同样继续有效。

- 消息：末尾跟随仅在原本接近底部时生效；向上翻阅后追加内容不抢回底部。纯文本换行、用户气泡、作者/动作、流式期间禁用动作、反馈、附件元数据与移除、消息分隔线。
- 输入/决策：PromptInput 的 Enter/Shift+Enter、IME 预编辑保护、提交数据快照、忙碌/失败状态、停止与附件回调；ModelSelector 切换后保留有效 effort 或回退；Confirmation 一次性决定；Questionnaire 必填与禁用选项校验、上下步和防重复提交。
- 工作流：计划进度、任务/队列独立折叠、应用提供的公开步骤摘要、工具输入输出/错误、只读命令日志、子代理元数据、堆栈回调、失败测试套件默认展开；不执行命令、模型、恢复或上传。
- 导航/数据：侧栏和导航的 roving focus、面包屑、菜单栏键盘跨菜单；来源 URL 校验与去重、引用预览、上下文用量；头像图片所有权快照和圆形剪裁；图表柱/线/面积、缺失值断点、包含零的数值域、指针/键盘选择与清除。
- 弹层/通知：Drawer 手柄拖动阈值、取消复位和卸载释放；Sonner 非模态视口、显式时钟截止点、hover/focus/后台暂停、更新与动作一次性执行、卸载不再请求计时。macOS key-window 激活状态接入通知暂停。
- 共用布局：横排先分配 grow 宽度，再按实际宽度计算换行高度；窄容器避免长说明文字挤出动作。图表使用明暗语义色并在图例显示对应色块。

`advanced_test.go` 使用显式事件/时钟验证上述关键边界；Studio 覆盖本批全部 37 个明暗演示。所有测试通过与离屏画廊均不构成原生窗口全状态验收，也不提供性能领先证据。

保留高级缺口：流式 Markdown/富文本编辑与选择、原生 IME/辅助技术、图表曲线/轴/提示配置、历史消息/树/日志虚拟化、文件拖放、复杂导航、触摸/惯性/动画、完整视觉尺寸变体与正式跨平台窗口。Terminal 当前为日志组件，不是终端仿真器；Reasoning/ChainOfThought 仅显示应用显式提供的内容。

## 按参考行为收敛，而非目录计数（2026-11-11）

100% 对齐仍以原始能力、交互、视觉、平台和性能证据全部满足为准。目录状态不升级为 complete。本批逐段核对本地只读参考 `forma/chart.go`、`forma/chart_test.go`、`chart/chart.go` 的 Select 行为，以及 `forma/model_selector.go`：

| 参考行为 | 当前增量 | 验证 |
| --- | --- | --- |
| 图表仅统计有分类标签的数据，包含零并扩展到整洁刻度 | 修正多余数组值污染数值域；支持次正规数和 float64 极值 | `TestChartNiceScaleRoundTicksAndExtremes`、`TestChartOnlyCategoricalDataAffectsDomain` |
| 柱居中、折线/面积端点、单点居中 | 分离分类坐标和最近点选择 | `TestChartCoordinatesAndNearestSelection` |
| Linear、Monotone、StepAfter、StepBefore、Step | 独立实现五种插值与分段渐变面积；整段一次光栅化避免透明连接处变深 | `TestChartCurvesStepPositionsAndMonotoneBounds`、`TestChartPaintClipGradientAndTransparentJoins` |
| 缺失值分段、空数据、短序列、越界选中 | 可见值快照；断点不连线；空数据清除选择 | `TestChartMissingValuesProduceActualGaps`、`TestChartEmptyAllKindsAndShortSeries` |
| hover 最近分类、离开清除、静止指针不覆盖键盘 | 加入受焦点作用域约束的指针移动通知、卸载清理 | `TestChartHoverKeyboardPrecedenceAndLeave`、`TestChartHoverUnmountAndDisableDoNotEmitUserActions` |
| Label/Height/Curve、隐藏轴网格/图例、FormatValue、OnSelect、选择提示 | 配置和主题提示面板接入实际 Go 绘制；ShowYAxis 可选 | `TestChartSelectionTooltipFormatAndReadOnly`，Studio 交互切换类型/插值 |
| 模型 provider 首次出现顺序分组、描述、名称回退 | 分组顺序与键盘顺序一致；长描述按可用宽度截断 | `TestModelProviderOrderDescriptionsAndFallbackNames` |
| effort 保留/回退/隐藏，禁用模型和空列表 | 禁用当前模型的 effort 保留可见但不可编辑；刷新列表不擅改应用值 | `TestModelDynamicDisabledEmptyAndSnapshot` |
| 仅实际 model/effort 变化通知 | 修正重复选择触发回调/擅自归一化 effort | `TestModelSelectionCallbackOnlyForActualChanges` |
| Placeholder/Disabled 配置 | 新增可选 ModelSelectorOptions | `TestModelSelectorPlaceholderAndOptionsDisabled` |

之前缺口描述中的“模型搜索”并非已读参考 ModelSelector 的必要行为，已纠正为实际缺失的视觉/平台能力；没有据此宣称该控件完整对齐。本批随后补齐 ChartContainer/Legend/Tooltip 的首批组合行为；`ui/chart` 下其他几何/比例尺/堆叠/采样能力仍明确保留，不以高层 Chart 功能覆盖替代。

`BenchmarkChartCurvedAreaRender` 仅提供本实现的可重复测量入口。尚未执行等价参考基准，不宣称速度领先。完整 86 项及原生窗口/编辑器/插件仍继续处于部分实现状态。

图表组合新增证据：`chart_compose_test.go` 验证 16:9 容器四轴布局、有效刻度排序/过滤、DPI 下自定义绘制剪裁、labels/colors 所有权快照、图表提示框 Escape/外部关闭与焦点返回。这些测试不覆盖原参考任意可访问 mark 树、剩余底层 chart API 或原生屏幕阅读器，故 chart 仍为 partial。

## Terminal 与插件入口纠正（2026-11-11）

核对只读参考 `backend/ui/forma/ai_trace.go`、`forma/code_block.go` 和 `backend/plugins/terminal/README.md` 后，确认 Forma Terminal 是命令输出控件，而插件 Terminal 是 PTY + VT 终端。Studio 此前按去掉 `plugin/` 的同名 ID 查找演示，错误地让未实现的终端插件打开日志控件；现改为显式插件映射，只有已实现的 loading 插件接入演示。

本批 Terminal 补齐可折叠输出、运行状态优先、未知退出状态 pending、非零退出危险色、等宽文本、Tab 展示、保留原始字节复制、双轴滚动和追加输出跟随。CodeBlock 复用等宽滚动区；Studio 提供 Terminal 四种状态切换，并为 Terminal/CodeBlock 接入 macOS 剪贴板。`terminal_test.go` 验证状态、折叠/键盘、禁用/只读、复制字节、滚动跟随及明暗高 DPI 渲染。系统剪贴板和实机输入仍待手工验收。

这没有实现终端插件。Studio 尚未实现的 15 个插件为 automation、borderbeam、chromium、dsh-go、fetch、game、glass、permissionflow、scene3d、sfsymbols、sqlite、terminal、updater、watch、websocket。它们包含控件插件和服务插件，不能计作 86 个 Forma 目录控件。Terminal/CodeBlock 的文字选择、高亮及其余高级操作仍未完成，状态保持 partial。

### Markdown 与插件补充

新增 `advanced.json` 单独审计原 Forma 目录外的 Markdown 阅读器与富文本编辑器。Markdown 已有原生样式排版与 Studio 编辑/预览演示；选择、图片、数学、增量解析等仍未实现。终端新增有限 VT 屏幕与原生视口；Studio 演示明确为本地输入回显，不自动启动 shell。Loading 新增四种独立动画。以上仍为 partial，CEF 原生宿主仍缺失。

### `ui` 全目录构造入口与新增基础控件

使用 `go run ./scripts/devtool audit-ui --source /absolute/path/to/golet` 生成 `ui.json`，盘点 `backend/ui/api_gen.go` 的全部 100 个返回 Element 的构造入口及子包目录。原始完整声明/测试身份仍保存在 `reference.json`；构造入口盘点不是所有方法、渲染能力或行为的完成证据。已有入口通过显式映射复用，不重复实现别名。

本批在 `omniui/baseui` 补充 15 个原来缺失的控件，并在 `advanced.json` 逐项列出剩余能力与测试：数值/日期/时间、搜索、评分、范围滑块、标签输入、工具栏、颜色选择/色井、图像、网格、复选分组、就地编辑、查找栏。Studio 有独立可交互演示。

未映射入口和编辑器、虚拟列表/网格、Outline、SVG、媒体等高级缺口继续保留。所有新项均为 partial；不以 100 个入口盘点、15 个新增实现或构建成功宣称全量对齐。

### Lucide / SVG 图标

参考 `backend/ui/svg.go` 的 ParseSVG、Icon 与 Painter.Icon 接口，新增可选 `omniui/icon` 解析/光栅化包、`omniui/icon/lucide` 官方固定版资源及 `baseui.Icon` / `IconButton` / `Painter.Icon`。2,350 个含别名的 SVG 均有解析、着色与非空绘制测试；DPI、按钮禁用/只读和缓存所有权另有测试。未复制参考实现体。

当前支持 Lucide 使用的路径/基础形状，保留圆头/圆角连接，支持单色着色和描边。复杂 SVG 的变换、渐变、CSS、遮罩、滤镜和文字明确拒绝，仍保持 partial；全图标测试不等于全 SVG 标准或原生系统辅助技术验收。

### 全局视觉优化

对照只读参考 `backend/ui/forma/theme.go` 的 24/32/36/40 按钮高度、20 DIP Badge、字体层级和外扩键盘焦点样式，重做 OmniGo 共用文字测量/绘制、紧凑尺寸、焦点可见性及弹层层次，并调整表单、表格、菜单、Tabs、消息和工作流标题。保留既有行为和所有能力缺口，不升级完成状态。

`visual_style_test.go` 覆盖字号与布局/DPI一致性、字号改变后的输入命中、嵌套行焦点外环、鼠标与键盘焦点区别、Badge 内容宽度、阴影剪裁和字体释放。Studio 全目录明暗画廊位于 `build/style-review-all`；离屏审图不替代真实原生窗口或参考逐像素差异验收。

## Maia 演示共用视觉基线（2026-11-11）

只读对照来源：`backend/examples/studio/forma.go` 及 `backend/ui/forma/{theme,inputs,layout,navigation}.go`。本轮独立调整 Go 绘制/布局，不复制控件实现：1040×840 演示画布、32/24 页面间距、Card 24 间距、16/500 标题与 14 说明、230 最小内容高度、Field/Form 8/24 间距、Tabs 30+3+3、菜单条目圆角 12、输入半透明底色、按钮变体 hover/disabled 配色。macOS 使用可解析的系统字体，其他平台保留 Go 字体。

`build/golet-style-alignment` 本轮生成 108 个可运行 Go 演示的 216 张明暗图与 14 张总览（含绘制插件，不含独立 CEF）。总览用于人工检查共同结构与明显裁切；并非与 golet 同状态截图做像素差异验收。部分演示增加了状态组合，其余示例内容、定制绘制和全部交互状态仍未与参考一一等价。因此原能力状态不升级为 complete，系统字重/复杂排版/CJK、个别组合与插件精确效果仍属于未完成范围。

## 精确小数与通用树形控件增量

新增 `DecimalInput`、`TreeOutline[K]`、`OutlineTable[K]` 及 Studio 演示。高级清单中的 decimal-input、outline、outline-table 从 pending 改为 partial；没有升级为完整对齐。精确小数测试覆盖超出 float64 安全整数的金额、分组/精度校验、上下限、取消、IME 和只读/禁用；树形测试覆盖展开、跳过禁用节点、父子导航、重新排序后的键选择、收起移除子项及循环拒绝。通用演示巡检覆盖明暗和窄宽布局。

虚拟列表/网格、编辑器、视频、原生 WebView 仍为 pending；新控件的剩余能力逐项保留在 advanced.json。CEF 外部浏览器不计作原生 WebView 控件。

## 原“等待重写 10 项”的新增实现

在此前 DecimalInput / TreeOutline / OutlineTable 的基础上，本批新增 VirtualList、GridView、CodeEditor、RichTextEditor、Timeline、Video 和 WebView 的实际局部实现。`advanced.json` 中这些 7 项改为 partial；100 个构造入口现均有映射，**这只表示没有完全空缺的入口，完整对齐仍为 0**。

虚拟控件测试用百万项数据验证可见窗口的构建数量、远端键盘跳转、删除后滚动夹取和禁用项；编辑器测试验证内容/格式历史、缩进、搜索与 IME；显式时钟测试覆盖暂停、seek、交替、卸载和后台窗口恢复；视频验证像素所有权、解码取消、GIF disposal 与关闭释放；WebView 验证裁切、焦点、只读和卸载协议。Studio 6 项 Go 演示含明暗渲染，原生 WebView 单独通过 WKWebView 导航回调与 JavaScript 结果验证。

每项未完成能力保留在 advanced.json。尤其：富文本目前为分离编辑/预览；视频没有音频和流式预解码；WebView 原生适配仅 macOS arm64，禁用页暂隐藏，复杂弹层合成与其他平台未实现。不能将这次 pending 数字归零解读为 golet 完整功能对齐。

## 输入组合与演示留白修复

对照参考 Studio InputGroup 示例的 XS 按钮（24 DIP）和 InputGroupInput（34 DIP），修正原先把 36 DIP 默认按钮塞进 36 DIP 外框的示例。InputGroup 改为最小高度 36、按子内容测量高度，较高的附加控件不再被固定高度截断；保留调用方元素所有权、键盘、只读及单一外框。

演示界面的标题统一为 OmniUI，副标题不再显示参考主题名称；移除所有演示共用的 230 DIP 最小内容高度，卡片根据内容自然伸展。尺寸与窄宽布局测试见 input_group_layout_test.go，界面文案和内容高度回归见 presentation_test.go。此次修复不代表所有参考控件已经逐像素对齐。

## Breadcrumb 对齐修复

移除祖先链接的默认按钮内边距，改为紧凑的 14 DIP 文字链接；链接与当前页使用一致高度，行内容居中，使用 14 DIP 箭头分隔符和 6 DIP 间隔。固定高度 Text 的行距上下对称，使实际字体基线与按钮文字一致。末项保持“当前页”文字语义，不再伪装成可点击链接。

breadcrumb_test.go 同时验证控件几何与实际字形像素的纵向范围，覆盖明暗、1/1.25/2× DPI、窄宽省略、禁用和只读。参考来源为只读 golet ui/forma/layout.go 的 Breadcrumb 视觉规格；未复制框架实现。

## 编辑器子控件补录（2026-11-11）

补录原先 `api_gen.go` 构造器统计未覆盖的编辑器子控件：`DiffView`、`GoToLine`、`CompletionMenu`。三项均在 `omniui/baseui` 独立实现，并加入 Studio 原生演示、明暗画廊及全目录确定事件巡检。参考仓库仅用于接口与能力核对，没有引入其实现或依赖。

- DiffView：逐行差异、双栏共享行网格、统一视图、前后差异导航、逐块还原和撤销。保留原始 CRLF/末尾换行；仅绘制可见行。LCS 表最多一百万单元，较大差异区间退化为整段替换，尚非增量或最小差异保证。长行目前裁剪；行内高亮、折叠、批注、补丁解析与原生无障碍继续保留在清单。
- GoToLine：一基行号、Unicode rune 定位、范围校验与只读保护，不修改文档内容；演示联动原生 Go 编辑器。
- CompletionMenu：过滤、键盘候选选择、禁用项、rune 范围插入和编辑撤销；绑定文档快照，拒绝将过期候选应用到已修改文档。当前是可组合独立候选面板，语言服务、自动触发、光标锚定和 snippet 未实现。

证据：`diff_view_test.go`、`editor_navigation_test.go`、Studio sweep；截图位于 `build/editor-controls-review`。三项状态保持 partial，不能据此宣称全部 golet 控件已经完整对齐。


## 2026-11-14：差异视图、数值输入、树形视口与原生测量

- DiffView 缓存统一视图行映射；分栏视图共享不可变行数组，文本修改/还原/撤销后重算。
- NumberInput 修复 1e-14 等步长被固定 12 位舍入抹除，失焦恢复有效值草稿且不重复通知。保留已有输入精度语义，不复制 golet 对所有输入按步长精度舍入的行为；地区格式和原生 IME 失焦验收仍缺失。
- TreeOutline / OutlineTable 新增 RowHeight / Height，显式启用固定行高视口。仅单元格构建虚拟化，展开层级索引仍为 O(N)。
- framecheck 在进程主线程测 AppKit 复制与同步 display 提交。空闲采样工具为 scripts/measure/idle.py。真正屏幕呈现延迟和长期 RSS 尚未验收。

回归见 alignment_regression_test.go；测量见 docs/performance.md。本轮不升级目录项为 complete。


## 2026-11-14：编辑器缓存、视口绘制与输入生命周期

CodeEditor 以不可变 TextSnapshot 的身份和语言为缓存键，缓存高亮、行数、最大行宽。
编辑、撤销、语言切换和 buffer 替换会失效；旧挂载树引用的高亮切片不原地修改。
文本绘制用 rope 行索引读取视口附近行，保留相邻行以容纳字形与组合文本下划线。
这不是编辑时增量词法分析，也不是复杂文字整形或横向长行虚拟化。

TreeOutline/OutlineTable 的标签仅在可见行构建；typeahead 首次使用时获取全量标签。
行展开索引、唯一性验证和导航选项仍是 O(N)。不复用可能被旧控件回调捕获的可变切片。

Surface 在焦点转移、控件移除/禁用/只读、buffer 替换和关闭时取消组合文本，
不会把草稿提交到另一个输入框，也不会生成撤销记录。原生 NSTextInputClient 和读屏
适配尚未接入。验证见 `omniui/baseui/alignment_cache_test.go`；所有状态保持 partial。
