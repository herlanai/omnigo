# 可选原生软件窗口

`apps/nativewindow` 是 Go 实现的实验性 Win32 / Xlib 适配器，可传给 `lifecycle.Manager.Run`。不导入此包的应用不会加载 X11；Windows 使用系统 DLL，Linux 运行时动态加载 `libX11.so.6`，均支持 `CGO_ENABLED=0`。没有复制 golet 实现，没有引入 SDL、GTK、React 或 WebView。

```sh
# 在 Windows / Linux 本机运行；Linux 需 DISPLAY 和 libX11.so.6。
CGO_ENABLED=0 go run ./examples/native-window
# 本机原生生命周期探针（创建、提交一帧、取消、销毁）
OMNIGO_NATIVE_TEST=1 CGO_ENABLED=0 go test ./apps/nativewindow -run TestNativeWindowLifecycle -v
```

PowerShell 请用 `$env:CGO_ENABLED="0"` / `$env:OMNIGO_NATIVE_TEST="1"` 设置变量。交叉构建用 `GOOS=windows|linux GOARCH=amd64|arm64`；不能在 macOS 执行这些二进制。macOS 继续使用 Studio 的 Cocoa 宿主，这个包不会替换它。

## 当前实现

- Win32：窗口注册/创建/关闭、主线程消息循环、鼠标捕获与取消、滚轮、键盘、UTF-16 代理对、IMM32 预编辑/结果/取消、GDI 软件帧提交、客户区缩放与最小尺寸。
- Linux：X11 窗口、WM_DELETE_WINDOW、poll 事件等待、管道唤醒、XImage 帧提交、鼠标/滚轮/键盘/焦点、XIM UTF-8 提交、窗口缩放与最小尺寸。XIM 使用输入法服务器负责的预编辑样式，未实现控件内预编辑回调。XIM 不可用时基础按键输入仍可用，不能据此声称中文输入法可用。
- `Window.Post` 通过 `runtime/dispatch` 回到 UI goroutine；context 取消唤醒原生等待；关闭后拒绝排队任务，原生资源在 UI 线程释放。回调中不能同步等待 Post 的结果。
- 静止窗口阻塞等待；调用方通过 `NextFrame` 请求动画/悬停定时，显式 `Tick` 推进控件时钟。绘制和输入不会由后台 goroutine 执行。
- `Surface.Resize` 保留文本、焦点和组合状态并重新布局；示例展示输入框和有状态按钮。

## 边界与验收

本轮证据是单元测试、race/vet 和四个 Windows/Linux 目标的编译，**没有 Windows/Linux 实机运行结果**。本地没有可用的目标系统或虚拟机；原生探针默认跳过，必须在对应桌面环境显式执行。

这不是完整桌面框架或 Studio 跨平台宿主。仍缺少：WebView2 / WebKitGTK、原生 Wayland、系统剪贴板/菜单/托盘/对话框、拖放、无障碍、触摸、每屏 DPI 与正确候选窗光标定位、窗口状态持久化。没有 GPU 或性能倍数声明。Windows 当前使用系统 DPI 虚拟化；Linux 使用像素坐标，不能宣称 Retina/多屏 DIP 对齐。

Linux 在 Wayland 会话中需要 XWayland；无 DISPLAY 时明确报错。软件输出暂限 24/32 深度、32bpp little-endian RGB visual。XIM 初始化遵循进程环境并设置 LC_CTYPE；应在应用启动阶段创建窗口，避免其他 goroutine 同时更改 locale。输入框默认 Inter 字体不覆盖 CJK，需要调用方提供合适字体。

自定义标题栏（包括 HiddenInset）、TrafficLightPosition、URL 会明确返回不支持，避免静默忽略参数。macOS HiddenInset 仍由现有 Cocoa 宿主实现。此接口当前是一窗口一个事件循环，不是完整多窗口应用管理器。

## API 依据

- [Win32 IMM32 组合输入](https://learn.microsoft.com/en-us/windows/win32/api/imm/nf-imm-immgetcompositionstringw)
- [Xlib 标准](https://www.x.org/releases/X11R7.6/doc/libX11/specs/libX11/libX11.pdf)

系统 API 签名和 ABI 按官方契约重写；purego/x/sys 是项目已有依赖，遵循其原有许可证。

## 窗口管理对齐

现在提供 `SetTitle`、`SetContentBounds`、`Show`、`ShowInactive`、`Hide`、`Focus`、`Minimize`、`Maximize`、`Restore`、`SetAlwaysOnTop`、`SetFullScreen`、`SetOpacity` 和 `Close`。这些方法通过 UI 队列执行，返回 `(完成通道, 入队错误)`；完成通道给出原生调用错误，不可在 UI 回调中等待。context 在执行前取消时不会调用原生 API。

`Options.ShouldClose` 只拦截系统标题栏的关闭请求；显式 `Close` 绕过拦截，拒绝后续任务并正常退出事件循环。context 取消同样用于强制结束生命周期。关闭不会留下可调用的原生句柄。

示例新增标题修改、最小化/最大化/恢复、全屏、置顶、透明度与关闭保护按钮。Windows 全屏保存 WINDOWPLACEMENT 和样式，退出时恢复；Linux 通过 EWMH 请求窗口管理器处理。Linux 全屏/置顶/几何改变需要窗口管理器支持，透明度需要合成器；成功返回表示已发送请求，并非已查询确认最终状态。Windows 的前台激活也可能被系统焦点策略拒绝。

`Bounds` 当前是客户区像素坐标，未冒充 DIP。未实现的状态查询、最小/最大尺寸动态修改、可缩放/可移动属性、菜单/托盘/图标/拖放/进度栏等继续保留在审计中。标题栏 HiddenInset 仍只在现有 Cocoa 宿主可用。

参考：[EWMH 状态与窗口消息](https://specifications.freedesktop.org/wm/latest-single/)、[Windows WINDOWPLACEMENT](https://learn.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-windowplacement)。

## Owned child windows

`parent.NewChild(options)` 从父窗口的 Ready/Event/Post UI 回调创建子窗口。子窗口共享根事件循环，不调用自己的 Run。Windows 使用 owner HWND，X11 使用同一 Display 的 transient-for 关系，并将事件按窗口分发。父窗口关闭时按叶子到根销毁子树，创建失败会清理已创建的后代；显式 Close 只关闭该子树。示例增加 Open owned child window。

父窗隐藏/最小化会隐藏可见子窗；父窗恢复后只恢复由父窗隐藏的子窗，保留用户主动隐藏的状态。单树最多 128 个子窗口，子窗有独立派发、控件状态、绘制与定时截止时间。此前“X11 仅一个窗口”的限制现改为“一个共享事件循环/连接下的一棵窗口树”。

确定事件测试覆盖销毁顺序、失败清理、关闭后派发拒绝与显式时钟；新增 opt-in `TestNativeOwnedChildLifecycle` 可在 Windows/Linux 本机验证创建/帧提交/关闭，目前没有目标系统实机结果。Cocoa 公共驱动、完整模态关系、跨屏/全屏/置顶组合及多父窗多窗口树策略仍未完成，不能据此宣称 golet childwindow 全覆盖。
