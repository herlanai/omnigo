# Omni Studio

OmniGo 的开发实验室。真正的 macOS 客户端：AppKit 主窗口内嵌 WKWebView，加载编译进 Go 二进制的 TanStack Start + shadcn 风格目录页，通过 OmniWeb 调用同进程 Go 宿主；所有被演示的控件、状态、布局、绘制仍由 `omniui/baseui` 实现。没有复制参考项目的实现，没有将 Go 控件改写成 React。

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
- 当前 34 个局部实现控件均有 Go 演示，另有 loading 插件的 Ring/Dots/Bars。待重写项完整保留，不显示虚假的启动按钮。
- 名称/API 搜索、类别筛选、仅看可演示、明暗主题、实现路径、迁移状态与连接错误重试。
- 选择控件后打开独立 Go 原生演示；同一控件与主题复用窗口，关闭后重开重置状态。最多 12 个并存窗口。
- 目录数据来自现有迁移审计；`pnpm studio:catalog` 重新生成。测试检查参考目录、插件与状态没有漏项。

## 实际平台边界

**原生演示当前只在 macOS Apple Silicon 实现。** 宿主通过 purego 调用 AppKit，Go CPU Surface 渲染结果用 NSImageView 显示，鼠标与键盘事件交给 Go 控件。内嵌目录页只做能力浏览与启动操作。其他平台的 Studio 客户端适配仍未完成。

这是 Studio 内部实验宿主，尚不是正式的通用桌面驱动：固定尺寸、无 Retina 分辨率绘制、无系统 IME/屏幕阅读器/剪贴板、无复杂文字整形、无 GPU。默认字体不覆盖完整中文字符。每次绘制暂经 PNG 编码传给 AppKit；动画只在可见且需要帧时请求，仍需后续更换高效像素上传。此路径没有性能优势证明。

`native-select` 当前展示现有 Select 的 Go 内联实现，并非系统原生下拉菜单；完整下拉、滚动与无障碍仍在原审计的未完成范围。CodeBlock 的复制动作仅演示 Go 回调，不写系统剪贴板。各控件的完整限制见 `omniui/baseui/README.md`。

框架能力区目前是实现状态说明，尚不是生命周期、权限、更新、存储、终端等全部功能的交互实验。完整实现覆盖率仍为 0，Studio 的可演示数量不代表与参考框架完全对齐。
