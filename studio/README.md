# Omni Studio

OmniGo 的开发实验室。TanStack Start + shadcn 风格目录壳，通过 OmniWeb 调用 Go 宿主；所有被演示的控件、状态、布局、绘制仍由 `omniui/baseui` 实现。没有复制参考项目的实现，没有将 Go 控件改写成 React。

## 运行与打包

需要 Go 1.26+、Node 24+、pnpm 10.33。仓库根目录执行：

```sh
pnpm install --frozen-lockfile
pnpm dev          # 编译 Go 宿主，启动 Vite，打开浏览器；Ctrl+C 停止两者
pnpm build        # 类型检查、前端构建、Go 编译、复制静态资源
pnpm start        # 运行构建后的 Studio 并打开目录页
pnpm package      # 重新构建并生成当前平台归档
pnpm check:studio # Go 演示/目录检查与前端类型检查（先 build 生成路由）
```

开发目录页：`http://127.0.0.1:5173`，Go 宿主：`http://127.0.0.1:7331`；生产只使用后者。端口占用会明确报错退出。开发启动使用已编译 Go 可执行文件，避免 `go run` 子进程残留。关闭全部演示窗口不会停止服务，使用终端 Ctrl+C。

macOS 输出 `build/studio/Omni Studio.app`，归档 `build/studio/omnigo-studio-darwin-arm64.zip`。可执行文件与网页资源一起打包，无需 Node 运行生产包。应用为本地未签名版本，不包含签名、公证或自动更新。Linux/Windows 生成相同资源结构的可执行目录与 tar.gz；跨编译检查不代表已验证这些系统上的运行行为。

## 目录与演示

- 86 个参考控件目录项、16 个插件，以及 8 项框架能力说明。
- 当前 34 个局部实现控件均有 Go 演示，另有 loading 插件的 Ring/Dots/Bars。待重写项完整保留，不显示虚假的启动按钮。
- 名称/API 搜索、类别筛选、仅看可演示、明暗主题、实现路径、迁移状态与连接错误重试。
- 选择控件后打开独立 Go 原生演示；同一控件与主题复用窗口，关闭后重开重置状态。最多 12 个并存窗口。
- 目录数据来自现有迁移审计；`pnpm studio:catalog` 重新生成。测试检查参考目录、插件与状态没有漏项。

## 实际平台边界

**原生演示当前只在 macOS Apple Silicon 实现。** 宿主通过 purego 调用 AppKit，Go CPU Surface 渲染结果用 NSImageView 显示，鼠标与键盘事件交给 Go 控件。网页只做目录与启动操作。其他平台仍可运行浏览器目录页，但不能打开原生演示。

这是 Studio 内部实验宿主，尚不是正式的通用桌面驱动：固定尺寸、无 Retina 分辨率绘制、无系统 IME/屏幕阅读器/剪贴板、无复杂文字整形、无 GPU。默认字体不覆盖完整中文字符。每次绘制暂经 PNG 编码传给 AppKit；动画只在可见且需要帧时请求，仍需后续更换高效像素上传。此路径没有性能优势证明。

`native-select` 当前展示现有 Select 的 Go 内联实现，并非系统原生下拉菜单；完整下拉、滚动与无障碍仍在原审计的未完成范围。CodeBlock 的复制动作仅演示 Go 回调，不写系统剪贴板。各控件的完整限制见 `omniui/baseui/README.md`。

框架能力区目前是实现状态说明，尚不是生命周期、权限、更新、存储、终端等全部功能的交互实验。完整实现覆盖率仍为 0，Studio 的可演示数量不代表与参考框架完全对齐。
