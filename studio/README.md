# Omni Studio

OmniGo 的开发实验室。真正的 macOS 客户端：AppKit 主窗口内嵌 WKWebView，加载编译进 Go 二进制的 TanStack Start + shadcn 风格目录页，通过 OmniWeb 调用同进程 Go 宿主；所有被演示的控件、状态、布局、绘制仍由 `omniui/baseui` 实现。基础控件仍为 Go 实现；Chromium 按用户明确要求直接移植 Golet 插件，保留 MIT 许可证与作者署名，未引入 Golet 框架依赖。

## 运行与打包

需要 Go 1.26+、Node 24+、pnpm 10.33。仓库根目录执行：

```sh
pnpm install --frozen-lockfile
pnpm dev          # 构建并直接打开 Mac 客户端；不启动独立 Vite 或浏览器
pnpm build        # 类型检查、构建前端、嵌入资源、编译 Go 和生成 .app
pnpm start        # 直接运行已经构建的 Studio Mac 客户端
pnpm package      # 重新构建并生成当前平台归档
pnpm check:studio # Go 演示/目录检查与前端类型检查（先 build 生成路由）
```

直接运行 `pnpm dev`，或先 `pnpm build` 再 `pnpm start`。也可以双击 `build/studio/Omni Studio.app`。**不需要启动 Web 服务、外部浏览器或单独的前端进程。** 当前 dev 会先完整构建再运行，没有热更新；修改后重新运行。

生产运行只需要 `.app`，不需要 Node/pnpm、源码目录或网络下载。`studio/assets.go` 将构建产物嵌入 Go 二进制；WKWebView 从同进程管理的随机 loopback 端口读取资源并调用 OmniWeb。该端口不是单独运行的服务，没有固定 7331/5173 端口依赖。导航限制在该应用来源。关闭 Studio 主窗口、菜单退出或 Cmd+Q 会取消后台服务并关闭演示窗口；终端运行也可 Ctrl+C。

macOS 输出 `build/studio/Omni Studio.app`，归档 `build/studio/omnigo-studio-darwin-arm64.zip`。应用为本地未签名版本，不包含签名、公证或自动更新。当前客户端仅支持 macOS arm64，其他构建目标明确返回不支持，不会静默退回浏览器。六目标跨编译不代表已验证这些平台上的原生运行。

## 目录与演示

- 86 个参考控件目录项、16 个插件，以及 8 项框架能力说明。
- 当前 86 个局部实现控件均有 Go 演示，另有 loading 插件的 Ring/Dots/Bars。待重写项完整保留，不显示虚假的启动按钮。
- 名称/API 搜索、类别筛选、仅看可演示、明暗主题、实现路径、迁移状态与连接错误重试。
- 选择控件后打开独立 Go 原生演示；同一控件与主题复用窗口，关闭后重开重置状态。不设 12 个窗口的硬上限；不同控件与主题可同时打开，关闭时释放 Surface 并取消定时器。
- 目录数据来自现有迁移审计；`pnpm studio:catalog` 重新生成。测试检查参考目录、插件与状态没有漏项。

## 实际平台边界

**原生演示当前只在 macOS Apple Silicon 实现。** 宿主通过 purego 调用 AppKit，Go CPU Surface 渲染结果用 NSImageView 显示，鼠标与键盘事件交给 Go 控件。内嵌目录页只做能力浏览与启动操作。其他平台的 Studio 客户端适配仍未完成。

这是 Studio 内部实验宿主，尚不是正式的通用桌面驱动：固定尺寸、无系统 IME/屏幕阅读器、无复杂文字整形、无 GPU。已按窗口 backingScaleFactor 生成物理像素图像，并保持 NSImage 的逻辑尺寸，切换屏幕后重新绘制；真实多屏 Retina 效果仍需实机验收。默认字体不覆盖完整中文字符。每次绘制暂经 PNG 编码传给 AppKit；动画和行为延迟只在可见且需要帧/定时唤醒时请求，仍需后续更换高效像素上传。此路径没有性能优势证明。

`native-select` 当前展示 Select 的 Go 锚定弹层实现，并非系统原生菜单；已支持列表滚动、键盘和选中项滚入，平台无障碍等仍在原审计的未完成范围。CodeBlock 的复制动作仅演示 Go 回调，不写系统剪贴板。各控件的完整限制见 `omniui/baseui/README.md`。

框架能力区目前是实现状态说明，尚不是生命周期、权限、更新、存储、终端等全部功能的交互实验。完整实现覆盖率仍为 0，Studio 的可演示数量不代表与参考框架完全对齐。

新增演示：scroll-area、popover、dropdown-menu、combobox、command、resizable、sheet、aspect-ratio、direction、skeleton。macOS 宿主已转发 mouseMoved 和 scrollWheel；触控板精确增量保留为 DIP，传统滚轮按 16 DIP 步进。真实多屏、触控板和辅助技术体验仍需实机验收。

`go run ./studio/cmd/gallery` 可在不启动窗口的情况下，通过真实事件打开弹层、过滤列表、滚动内容，并生成两种主题的截图。

继续新增 tooltip、hover-card、context-menu、input-otp、file-tree 演示。宿主已接入次键点击、Shift+F10、鼠标离开追踪、Cmd+C/X/V 文本剪贴板和 NextWakeup 一次性定时调度；窗口关闭会取消定时器，过期回调通过代次校验丢弃。这些原生路径目前通过编译与核心接口测试，尚未完成实机剪贴板/辅助技术验收。

新增消息、工作流、问卷/确认、来源、模型/输入框、导航、图表、抽屉和通知演示全部使用 Go 控件。86 项均为 partial，0 pending 不表示完整对齐；高级缺口仍在审计清单中。通知窗口后台暂停已接入 macOS key-window 事件，系统 IME/辅助技术及真实窗口全状态验收尚未完成。

Terminal 演示现提供 Pending/Running/Success/Failure 切换及输出折叠；Terminal 和 CodeBlock 的 Copy 动作接入 macOS 文本剪贴板。`plugin/terminal` 是另一个尚未实现的 PTY/VT 插件，不再映射到 Forma Terminal 的日志演示。其他插件也只允许显式演示映射。

插件实现统一位于 `omniui/plugins`，Studio 清单从其 `status.json` 生成。新增可运行的 Borderbeam、Glass、Scene3D、Game 演示，以及原有 Loading。网络、数据库、PTY 等服务插件的部分实现不会被显示为同名基础控件演示。16 项均为 partial，Chromium 的 CEF 宿主现已直接移植并通过 macOS 浏览器实测，Windows 原生验收及 Linux 宿主仍未完成；Terminal 尚未提供完整 VT 视口。

## Icon 图标库与性能监控

左侧 **Icon 图标库** 独立展示全部内置 Lucide 图标（含上游别名），支持名称搜索、72 项分页、16–48 DIP 尺寸、0.5–4 线宽与明暗主题。右侧提供选中图标的大图、`baseui.Icon` / `IconButton` Go 示例以及代码复制。图片由 `omniui/icon` 的 Go 栅格器生成并以 2× PNG 展示，不使用浏览器 SVG 效果替代 Go 效果。图片服务限定尺寸、线宽和并发；不永久缓存全部图标的栅格数据。

左侧 **性能监控** 与顶部摘要显示实际采样：Studio 主进程 CPU、常驻内存 RSS、系统 CPU、系统非空闲内存，以及 Go 堆、运行时申请空间、goroutine、GC 次数和最近暂停时间。macOS 使用 Mach `host_statistics`、`proc_pidinfo`、`getrusage` 和 `hw.memsize`，无需 CGO 或轮询 shell 子进程。采样间隔 2 秒，宿主历史有界为 120 点；退出时停止采样并释放 Mach port / 动态库。非 macOS 平台明确显示系统指标未接入，仍可取得 Go runtime 数据。

口径：进程 CPU 以单核 100% 计，系统 CPU 为全核平均；首次采样及读取失败显示未知，不伪装成 0。RSS 不包含 WebKit 辅助进程；系统非空闲内存 = 物理总量 − 空闲页，包含缓存，不等于活动监视器的“已使用内存”。Go 堆和运行时申请空间不是 RSS。页面暂停冻结展示；宿主继续采样。页面隐藏停止轮询，断连显示错误并保留上次采样。趋势用于观察当前运行，不构成性能优势或帧率基准证明。

验证：`studio/internal/insights` 测试覆盖 Go PNG 输出、非法参数、CPU 差值/首次采样/计数回绕、有界历史和 macOS 实际数据读取。


## Chromium 浏览器

左侧 Chromium 浏览器入口选择对应插件，点击“打开 Chromium 浏览器”启动独立的 CEF Alloy 窗口，支持地址栏、标签页、新标签首页和导航。相同 Studio 会话复用已有浏览器，关闭后可重新打开；退出 Studio 会关闭浏览器及其子进程。运行时缺失时明确提示安装，不使用系统 WebView 假装 Chromium。

`pnpm setup:chromium` 构建可选宿主到 `build/chromium-runtime`；`CEF_ROOT=/path/to/sdk pnpm setup:chromium` 可复用官方固定版本 SDK。随后 `pnpm build` 把它附带到 Studio Resources，无需在运行时下载。`pnpm dev:browser --style chrome` 启动 Chrome 样式，`--extensions /absolute/path` 加载显式指定的未打包扩展；`--app --url https://example.com` 使用内容应用窗口。默认使用正常持久化配置和安全存储；一次性开发配置仅在显式 `--ephemeral` 或 E2E 测试中启用。

Go API、主题、CDP、JS 求值、助手及用户脚本详见 `omniui/plugins/chromium/README.md`。来源和改动见 `PORT.md`，许可证为 `LICENSE.golet`。CEF 运行时有自己的许可证与体积，安装包包含授权文件。macOS E2E 验证通过，不表示 Windows/Intel 实机已验收，也不表示支持布局内嵌 CEF 控件。

演示视觉基线：原生窗口与 gallery 统一为 1040×840，32 DIP 外边距、24 DIP 主间距，使用 Maia 卡片章节与独立滚动区。macOS 正文读取系统字体；字体不可解析时使用内置 Go 字体。全量预览命令 `CGO_ENABLED=0 go run ./studio/cmd/gallery -controls all -out build/golet-style-alignment` 包含可运行绘制插件。仍需逐状态参考截图验收，不能用这一统一基线声称每个控件已经像素等价。

Icon 图标库顶部的“打开 Go 原生演示”通过 studio.open 打开已有的原生 icon 窗口（相同主题复用窗口）。窗口内容由 Go baseui/Icon/IconButton 绘制，支持全 Lucide 目录搜索与分页、尺寸/线宽调整、选择预览、名称复制和对应 Go 代码查看/复制。网页目录仍作为浏览入口；原生演示当前支持 macOS arm64。

OmniBrowser 演示位于 Plugins → OmniBrowser，提供原生预览、HTML 编辑、JavaScript 求值、DOM 提取和截图。核心是 `omnibrowser` 的纯 Go 文档引擎，网络/脚本结果经 dispatch 回到 UI；不是 Chromium 包装。当前预览只实现语义 HTML 排版，完整 CSS、Web API、CDP/MCP 和 Obscura 对齐缺口见 `omnibrowser/README.md`、`capabilities.json`。
