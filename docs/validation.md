# 初始验证记录

日期：2026-11-11。环境：Apple M5，darwin/arm64，Go 1.26.2。

本次使用 `GOCACHE=/tmp/omnigo-go-cache`，避免受限环境下默认缓存目录的写入限制。

| 检查 | 结果 |
| --- | --- |
| `go test ./...` | 通过 |
| `go vet ./...` | 通过 |
| `go test -race ./...` | 通过 |
| `CGO_ENABLED=0 go build ./...` 跨平台 | darwin、linux、windows 的 amd64/arm64 六个目标通过 |
| `go run ./examples/lifecycle` | 启动/调用/差分/虚拟范围/清理完整执行 |
| `FuzzDiff`，5 秒预算 | 655,708 次执行，通过 |
| `FuzzRequest`，5 秒预算 | 1,104,223 次执行，通过；包含收尾实际约 6 秒 |
| 参考清单工具 | 确定性输出、排除实现体与依赖目录的测试通过 |

原始微基准见 [核心基准](benchmarks/core-darwin-arm64.txt)。该记录是在上述验证结束后单独执行三轮产生的初始数据，没有参考框架对照，不包含 UI 渲染或桌面应用耗时。

没有执行真实窗口、GPU、IME、无障碍、WebView、安装包或升级验证，因为这些能力尚未实现。Linux/Windows 结果仅为交叉编译，不能解释为实机兼容性验收。

## OmniWeb 增量验证

同日新增 OmniWeb v1、TypeScript SDK、静态资源宿主和默认模板后，重新执行完整检查脚本：Go 单元测试、vet、race、六目标交叉编译与 4 项 Node SDK 测试全部通过。

- HTTP 测试验证 Host/Origin 拒绝、媒体类型、请求大小、版本、未知字段、重复批量 ID、执行前校验、取消、错误信息脱敏与 SPA 回退边界。
- `FuzzEnvelope` 运行 5 秒预算，实际含收尾约 6 秒，完成 233,118 次执行并通过。
- CLI 在两个新目录生成默认项目；初次安装和携带锁文件后的全新 `npm ci` 均成功。锁文件加入可移植路径测试，不含本地临时目录或 file 依赖。
- 两个生成项目均完成 `vite build && tsc --noEmit`，生成 `dist/client/index.html`；验证版本包括 Node 24.15.0、TypeScript 7.0.2、Vite 8.3.8、TanStack Start 1.168.59 和 Router 1.170.40。
- 生成的 Go 宿主编译成功。在真实浏览器访问 loopback 宿主并点击“发送问候”，显示“你好，OmniWeb。来自 Go 的问候。”，验证客户端 → OmniWeb → Go 服务 → 页面结果的完整链路。
- npm 下载与 Start 预渲染的临时监听在受限环境外执行；预渲染所需 loopback 监听在受限环境内会被拒绝，属于执行环境限制。

此验证未覆盖系统 WebView、浏览器事件流、自动类型生成和桌面打包，也没有与参考框架进行等价性能对比。

## Go baseui 增量验证

控件实现统一到 `omniui/baseui`，本轮新增的 React 控件草稿已移除，已有 OmniWeb 模板保留。Go 控件目录不含 JS/TS/TSX 实现。

- 整体 tests、vet、race、SDK 测试与六目标交叉编译通过。
- 新测试覆盖原子 Mount、构造树快照所有权、禁用继承、指针取消、Space 单次激活、对话框焦点隔离与返回、可见文字/无障碍名称分离、文本撤销/重做、组合输入、IME Enter 不提交表单、只读导航、滑块边界、日历跨月焦点、表格可见行访问、绘制剪裁和图片所有权。
- `FuzzTextEdits` 以 5 秒预算完成 376,512 次执行并通过；文本偏移使用 rune，尚未实现完整字素簇导航。
- 基础主题测试检查正文、辅助文字、主按钮和危险文字对比度；loading 指标使用虚拟时钟测试动画/暂停。
- `examples/baseui-gallery` 成功生成并完成 light/dark PNG 的视觉检查。发现的日历可见文字使用了无障碍名称的问题已修复，并加入像素一致性回归测试。图像由 Go CPU 控件绘制，不是 Web 页面或设计稿。
- 覆盖门禁验证全部 86 个目录项都有状态，部分实现都指向 `omniui/baseui` 下真实的 Go 函数；当前 34 项 partial、52 项 pending，没有完整对齐声明。

仍未验证原生窗口、系统 IME/无障碍、GPU、DPI 缩放、复杂文本整形、完整高级控件与全部插件。当前默认字体不覆盖全部 CJK，需显式提供字体。尚无相对参考框架的性能提升证明。

## Studio 内嵌 Mac 客户端增量验证

2026-11-11，在 macOS arm64 验证：

- `pnpm build`、`pnpm package`、`pnpm check:studio` 成功；`pnpm dev` 实际完成构建并直接打开 AppKit + WKWebView 客户端，没有启动独立 Vite 服务或外部浏览器。TanStack 构建时短暂使用本机预渲染服务，生产运行不依赖它。
- 目录页资源经 `go:embed` 编译进 Go 二进制。最终应用包约 10 MiB 可执行文件、约 5.9 MiB zip，包含第三方许可证；未签名、公证或验证分发安装。
- 真实客户端主窗口显示 110 项目录能力、35 个可打开的局部 Go 演示。原生 UI 自动化观察到 WKWebView 加载、Go 服务连接、实现路径和迁移状态。点击“打开 Go 原生演示”进入独立 Accordion 窗口，再点击折叠项，实际展开 Go 绘制内容。
- 自动化期间部分 AX 点击和键盘调用超时；改用可见窗口截图定位的鼠标点击完成原生指针交互验证。未将这些超时操作计入通过项，未完成所有控件的系统键盘/IME 验收。
- 所有演示在 Go 测试中均通过明暗 Surface Mount 与 Render；另测按钮回调/禁用、对话框打开、新实例状态重置。目录门禁检查全部 86 个参考控件项、16 个插件和当前状态没有遗漏，34 个 partial 控件均有可执行演示。
- 新增宿主关闭与后台提交竞争测试。最终完整 `scripts/check.sh` 的 Go tests、vet、race、SDK 测试和六目标 CGO=0 交叉编译通过。进程 SIGINT 退出成功；没有把交叉编译当作其他平台客户端实机验证。

当前宿主为 Studio 内部实验实现，仅 macOS Apple Silicon 支持；固定尺寸 CPU 控件窗口、PNG 上传、无 Retina 绘制/系统 IME/原生无障碍/GPU。框架功能区仍是说明，待重写控件与插件仍不可演示。无性能领先或完整覆盖结论。
