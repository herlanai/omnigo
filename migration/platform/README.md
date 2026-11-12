# 原生平台对齐审计

最初的六目标检查仅证明 Go 核心可以构建；Studio 的 `unsupported.go` 排除了 Windows、Linux 和 Intel macOS。随后接入 Intel Cocoa 构建，并新增可选 Win32/X11 软件窗口驱动。软件窗口、Studio WebView 宿主和实机兼容性分别记录，不能混为一项支持声明。

## 当前六目标状态

| 系统 / CPU | Go 核心构建 | 原生窗口 / Studio | 运行证据 |
| --- | --- | --- | --- |
| macOS / ARM64 | 支持 | 实验性 AppKit + WKWebView | 本机 titlebar/WebView 探针；完整输入与无障碍仍有缺口 |
| macOS / AMD64（Intel） | 支持 | 已接入实际 AppKit + WKWebView 路径 | 本轮编译通过；当前 ARM 机器无 Rosetta，执行返回 bad CPU type，未做 Intel 实机验收 |
| Windows / AMD64 | 支持 | 实验性 Win32 软件窗口；Studio/WebView2 未实现 | 构建证据，非原生运行证据 |
| Windows / ARM64 | 支持 | 实验性 Win32 软件窗口；Studio/WebView2 未实现 | 构建证据，非原生运行证据 |
| Linux / AMD64 | 支持 | 实验性 X11 软件窗口；Studio/WebKitGTK/原生 Wayland 未实现 | 构建证据，非原生运行证据 |
| Linux / ARM64 | 支持 | 实验性 X11 软件窗口；Studio/WebKitGTK/原生 Wayland 未实现 | 构建证据，非原生运行证据 |

`omniwindow.Platforms()` 和 `go run ./cmd/omni doctor --all --json` 给出可机读的实现状态。实现状态不会因为运行了交叉编译而自动升级。

## 本轮改动

- 窗口管理继续对齐：新增 13 个实际 Win32/X11 操作（标题、客户区几何、显示/隐藏、聚焦、最小化/最大化/恢复、置顶、全屏、透明度、关闭），通过 UI 派发执行并返回错误。系统关闭可被 `ShouldClose` 拦截；显式关闭拒绝后续排队任务。方法映射、测试和未完成能力保存在 [window-actions.json](window-actions.json)。示例增加相应操作按钮；没有把 EWMH 请求成功写成最终状态确认。

- 新增 [omniwindow/nativewindow](../../omniwindow/nativewindow/README.md)：直接调用 Win32 / Xlib 的可选软件窗口驱动，包含事件循环、派发唤醒、像素提交、键鼠和焦点、缩放/最小尺寸、Windows IMM32 组合消息、Linux XIM UTF-8 提交。`examples/native-window` 将真实 Go 输入框和按钮接入驱动；`Surface.Resize` 保留控件状态。窗口可以直接传给 `lifecycle.Manager.Run`。这不包含 Studio WebView、每屏 DPI 或完整 IME 验收。

- macOS 原生宿主文件由 `_darwin_arm64.go` 改为 `_darwin.go`，显式限制 amd64/arm64；Intel 构建不再落入 unsupported 分支。现有 purego 版本包含 amd64 `objc_msgSend_stret` 支持，窗口坐标结构保留 `structs.HostLayout`。
- 新增 `omniwindow/system.OpenURL(ctx, url)` 和 `OpenPath(ctx, path)`：Windows 使用原生 ShellExecuteW（固定线程上初始化/平衡 COM）；macOS 使用系统 open；Linux 使用 xdg-open，缺失时尝试 gio。均不使用 shell 拼接执行，不支持的 scheme 和无可用系统处理程序明确报错。这只是 Shell 类别的一部分，不是原生窗口后端。
- 新增只读 `tools/platformaudit`：从 golet `backend/internal/platform/platform.go` 提取接口签名和原行号，未复制任何实现体。当前保留 26 个接口、297 条记录；[reference.json](reference.json) 是完整契约快照，[status.json](status.json) 列出实际映射与剩余能力。盘点不覆盖 golet 所有顶层模块/插件，不能当成整个框架覆盖率。

```sh
go run ./tools/platformaudit -source /absolute/path/to/golet
go run ./cmd/omni doctor --all
sh scripts/check.sh
CGO_ENABLED=0 go run ./examples/studio/cmd/titlebarcheck
CGO_ENABLED=0 go run ./examples/studio/cmd/webviewcheck
```

## 后续实现依赖

本轮验证（2026-11-12）：`sh scripts/check.sh` 通过（tests、vet、race、SDK 与六目标 CGO=0 构建）；Studio 打包通过。macOS ARM64 的 titlebarcheck 和 webviewcheck 均通过。WebView 探针需要正常桌面系统服务，Studio 预渲染需要本地回环监听；本轮在解除对应沙箱限制后验证成功。Windows/Linux 系统打开接口只有单元测试与构建证据，没有实机启动处理程序的验收结果。

新增 Win32/X11 驱动后再次通过完整检查，并额外通过 Windows amd64 / Linux arm64 专项 vet、目标测试二进制编译。确定性测试覆盖派发、resize、取消、错误释放、定时/空闲等待、像素布局、UTF-16 和 Surface 状态保留。`TestNativeWindowLifecycle` 需要对应桌面环境显式启用，本机默认跳过；没有把它计为 Windows/Linux 原生运行证据。

1. Windows：原生实机验收、每屏 DPI、输入法候选定位、剪贴板和无障碍，之后接 WebView2/菜单/对话框。
2. Linux：X11/XIM 实机验收、内联预编辑、原生 Wayland，再接 WebKitGTK；不能把需要 XWayland 的路径声称为原生 Wayland。
3. 公共窗口驱动：当前软件窗口已接入 `lifecycle.Manager`；仍需统一 Cocoa、实现多窗口管理、完整窗口状态和事件契约。仅增加同名类型或构造函数不能代替驱动。
4. 两种 CPU 均要原生验收：打开/关闭、resize、跨屏 DPI、焦点、IME、剪贴板、标题栏和后台/恢复。Windows ARM64、Linux ARM64 与 Intel Mac 当前无实机证据。

OmniBrowser/Chromium 是独立项目范围，本轮未修改；它们的运行能力不计为 OmniUI 的跨平台原生驱动。
