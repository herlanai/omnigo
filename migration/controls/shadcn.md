# shadcn 原生 Go 控件对齐记录

日期：2026-11-12。实现位于 `omniui/baseui`，未使用 React、WebView 或参考 Go 框架实现体。

## 参考与预设

- [用户指定预设](https://ui.shadcn.com/create?preset=bbVJxYW)：依据官方预设编码实现解码为 Maia、Inter、Neutral、默认圆角、subtle 菜单强调、Lucide，标题继承正文字体。预设页面在本次浏览器中停留在加载状态，因此没有声称完成与该页面的直接截图比较。
- [官方代码](https://github.com/shadcn-ui/ui/tree/dd41d19e2f80ff6891951f7ced74c2c9a650b065)：对照 registry 中 Maia/Nova/Lyra 的样式规则，重写 Go 几何、颜色、间距和状态绘制。没有引入其前端运行时。
- [官方组件目录](https://ui.shadcn.com/docs/components)：66 项入口、实现路径和剩余缺口保存在 [shadcn.json](shadcn.json)。所有项目保持 partial；入口存在不等于完整行为或视觉对齐。

## 本次实现

默认字体改为随程序嵌入的 Inter 4.1，400/500/600/700 使用各自真实字重，富文本斜体也来自 Inter。文字测量、输入命中和高 DPI 光栅化共用字体度量；不再混用系统 SF、Helvetica 和 Go 比例字体。字体资源来源、格式转换记录与 OFL 许可证保存在 `omniui/baseui/fonts/inter`，Studio 打包包含许可证。代码字体继续使用 Go Mono。字体不包含中文回退或复杂文字整形。

提供独立的 Maia/Nova/Lyra 几何样式、Neutral/Blue/Violet 强调色和菜单 subtle/bold 选中样式。按钮、输入、卡片、标签、菜单、Tabs、日期与滑块接入共享主题；Maia 卡片标题为 16/500，按钮与输入使用胶囊圆角，Nova 使用较小圆角，Lyra 使用直角。调整浅边框、输入底色、选中背景及弹层阴影，取消选中项通用的重描边。保留显式 `Padding`、`Gap` 和 `CornerRadius` 覆盖。

`Surface.SetTheme` 在 UI goroutine 上切换，保留编辑、选择、焦点和滚动状态，取消未完成按压并释放旧字体缓存。Studio 原生演示顶部可切换样式、强调色和选中强调。明暗主题继续由窗口创建参数决定。

新增两个可交互组合控件：

| 控件 | 已实现 | 仍缺少 |
| --- | --- | --- |
| DatePicker | 明确初始月份，单日/范围选择，禁用日期，键盘打开，Escape/外部取消与焦点恢复 | 本地化日期输入、时间选择、多月布局及平台无障碍验收 |
| DataTable | 大小写无关文本筛选，稳定文本排序，分页，列显隐，跨页选择，当前页混合全选状态 | 自定义/数字排序、服务端数据、列宽拖动、虚拟化和可编辑单元格 |

DataTable 数据源上限 10,000 行、每页上限 100 行。原有 Table 与 VirtualList 仍用于业务显式控制的大数据场景。Native Select 目前使用 Go 绘制的 Select 替代，未实现系统原生弹出选择器，审计明确保留此缺口。

## 验证与复现

`sh scripts/check.sh` 通过：测试、vet、race、SDK 测试、六个桌面交叉编译目标及生命周期冒烟检查。DatePicker/DataTable 专项事件测试覆盖选择、取消、禁用/只读、分页/筛选和状态恢复；样式测试检查真实字重缓存、主题切换时的编辑选择保留、按压取消与关闭资源边界。这里没有新增相对 golet 的等价性能基准，不能据此声称 100 倍加速。

以下生成真实 Go 离屏截图，不是网页截图：

```sh
CGO_ENABLED=0 go run ./studio/cmd/gallery -controls all -exclude plugin/omnibrowser -scale 1.25 -style 0 -out build/shadcn-alignment/maia-all
CGO_ENABLED=0 go run ./studio/cmd/gallery -controls all -exclude plugin/omnibrowser -scale 1.25 -style 1 -accent 1 -bold-menus -out build/shadcn-alignment/nova-all
CGO_ENABLED=0 go run ./studio/cmd/gallery -controls all -exclude plugin/omnibrowser -scale 1.25 -style 2 -accent 2 -bold-menus -out build/shadcn-alignment/lyra-all
CGO_ENABLED=0 go run ./examples/control-showcase -out build/shadcn-alignment/showcase
```

已生成三组全部目录明暗截图，人工检查了组合页、选中菜单、数据表及深色日期弹层。没有逐一人工验收全部截图，也没有完成所有 shadcn 状态的参考截图差异检查。真实窗口的跨平台输入、无障碍、IME、RTL、高对比与完整动画仍需验证；本次交叉编译不代表这些能力已完成。

原有 [status.json](status.json)、[advanced.json](advanced.json) 和参考能力清单继续有效。完整缺口不会因共享视觉更新或新增构造入口而删除。

## 菜单焦点与样式修正

针对用户反馈的 ContextMenu 首行顶部和两侧残缺灰框，移除 menuitem、menuitemcheckbox 和 option 上重复的按钮外扩焦点环。这些复合控件项仍用高亮底色表示键盘焦点；普通按钮与输入框保留原有焦点环。菜单、选择器与命令候选行采用 Inter Regular，按钮继续使用 Medium。

移除 Select、Combobox 和 Tabs 的固定 12 DIP 圆角，使已挂载控件随样式切换更新。Nova 菜单/候选项使用 6 DIP 圆角，Maia 使用 12 DIP，Lyra 使用直角；显式 CornerRadius 覆盖仍生效。

`menu_visual_test.go` 覆盖三种样式、明暗、subtle/bold 与 1/1.25/2 倍绘制，检查焦点高亮仍可见且不再叠加裁切外环，同时验证 Select/Tabs 实时切换圆角。截图位于 `build/menu-polish/maia` 和 `build/menu-polish/lyra`。此修正不代表所有组件的视觉缺口已解决。
