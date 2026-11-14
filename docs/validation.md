# 初始验证记录

> 本文保留按日期的历史过程与证据；旧条目中的“当前/尚未”仅指当批次。最新能力以 [当前状态](status.md) 为准，历史缺口的关闭需要后续证据。


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

## Windows/Linux 原生 CI 配置（2026-11-14）

新增 `.github/workflows/native-desktop.yml`，在 Windows runner 与 Linux Xvfb 执行现有原生软件窗口和父子窗口测试。结果以 JSON artifact 保留，`scripts/ci/require_native_tests.py` 拒绝缺失/跳过/失败的必需测试。检查器的两个单元测试本地通过。

工作流尚未在远程运行，不能计作 Windows/Linux 原生验收，更不代表 WebView2/WebKitGTK 已实现。完整升级边界见 `migration/wails-v3-upgrade.md`。

## 更新器显式恢复验证（2026-11-14）

新增持久事务日志、保存的发布签名、OS 进程锁及 RecoverInstallation。
真实子进程安装后未确认即退出，父进程重新接管并分别确认/回滚通过；
活跃所有者拒绝、签名损坏拒绝、备份篡改拒绝及 Close 后交接均通过。
独立外部模块的签名安装 → Close → RecoverInstallation → Rollback 验证通过。
最终 scripts/check.sh 通过，日志 /tmp/omnigo-recovery-final-check.log。

仅为单文件可执行程序更新事务的显式恢复。Windows 锁实现有交叉构建证据，
未执行 Windows 原生进程测试；目录 app 安装、进程退出等待、自动重启健康确认
和断电持久性不在本轮验收范围内。日志不完整时保留文件并返回错误。


## .app 安装与原生 Transport 验证（2026-11-14，本批）

- 独立外部模块 `/tmp/omnigo-bundle-acceptance`：ad-hoc 签名 bundle，发布签名 ZIP
  安装，真实原生窗口启动，自动健康握手和备份清理通过。不是公证分发验收。
- `examples/native-rpc-smoke`：WKWebView 原生 message/reply 的 Unicode RPC 和退出通过。
- Bundle 单元/进程测试覆盖归档越界拒绝、确认/回滚、恢复时拒绝活跃所有者、
  丢失可执行文件、真实子进程确认与确认前退出。SDK 测试覆盖 WebView2 回包关联和取消清理。
- Windows WebView2 是可选实验后端；用户没有 Windows 环境。Windows/Linux workflow
  尚未执行，不能宣称原生通过。完整状态以 migration/wails-v3-upgrade.md 为准。

最终 `sh scripts/check.sh` 通过：Go tests、vet、race、40 个 SDK 测试、六个默认
桌面目标，以及 Windows WebView2/Linux WebKitGTK 各两种架构的可选后端构建。
日志：`/tmp/omnigo-final-upgrade-check.log`。最后的更新子进程清理调整另行通过
updater race/子进程测试；没有新增 Windows/Linux 原生运行证据。

## Golet 后续对齐验证（2026-11-14）

- `sh scripts/check.sh` 通过，日志 `/tmp/omnigo-golet-alignment-check.log`：Go tests、vet、race、SDK 和默认/可选桌面后端跨编译。
- 最后补充的 doctor 库名检查另行通过 `go test -race ./cmd/omni` 与 vet；库存在不代表原生运行成功。
- 外部模块 `/tmp/omnigo-golet-alignment-external` 实际消费 keygen 公钥、解析 Base64 签名并验签，使用公开 CodeEditor API 重建和绘制通过。
- `/tmp/omnigo-golet-alignment-gallery` 生成编辑器、树、树表六张 2× 明暗截图；抽查编辑器亮色及树表暗色未见明显裁切，不代表参考像素一致或系统输入法验收。
- 缓存失效、旧快照不可变、屏幕外文本不测量、标签按视口构建、组合文本失焦/卸载清理均有专项测试。热构建基准范围见 performance.md。
- 本批没有新增 Windows/Linux 实机、macOS NSTextInputClient 或系统读屏证据。


## 四项原生/增量实现验证（2026-11-14）

- `sh scripts/check.sh` 最终通过，日志 `/tmp/omnigo-native-context-final-check.log`。
  后续高亮字符串所有权调整另行通过 baseui/host race 测试；没有将构建当成原生验收。
- `CGO_ENABLED=0 go run ./examples/studio/cmd/inputcheck` 在当前 macOS arm64
  真实 AppKit 窗口通过：marked text、代理对范围、候选框坐标、提交/撤销、原生
  NSEvent 输入、AX 角色/操作/选择、AppKit 无障碍子对象遍历及关闭后回调失效。
  探针曾发现默认 NSImageView 的 inputContext 为空；已改为显式拥有并释放
  NSTextInputContext，真实键盘路径复测通过。
- 单元测试覆盖随机编辑时增量/全量词法结果对照、与整文档 Go scanner 的有效
  示例对照、单行编辑仅重扫一行、树展开与完整索引一致、局部增删重排不读取
  无关分支、只读/禁用操作拒绝、UTF-16 选择和组合文本一次撤销。
- 独立外部模块 `/tmp/omnigo-four-capabilities-external` 使用公开原生文本 API、
  增量树选项和局部 Invalidate 完成构建与绘制。
- 原始增量基准：`docs/benchmarks/incremental-2026-11-14.txt`。编辑时索引复制、
  展开映射比较及最坏词法状态传播成本仍存在，未声明全流程常数时间或 Golet 倍率。
- 新增 macOS 原生协议 CI job，尚无远程执行结果。本地没有开启 VoiceOver、
  请求系统辅助功能授权或遍历所有输入源；人工朗读/导航、Intel 实机和 Windows
  UIA/Linux AT-SPI 仍未验收。

## P0 收敛验证（2026-11-14）

- 当前状态汇总迁至 `docs/status.md`；首页、架构、CLI 和历史记录均指向该入口。
- `sh scripts/check.sh` 通过，日志 `/tmp/omnigo-p0-check.log`：Go tests、vet、race、
  40 个 SDK 测试、六个默认目标和四个可选 Web 后端构建。新增 scaffold 测试在
  独立模块中 tidy 并以只读模块模式依据生成配置构建 Go/WebView 桌面项目。
- 独立 Go/WebView 项目真实生成 ad-hoc 签名 .app 和含 Applications 快捷方式的 DMG，
  codesign、Mach-O arm64 架构及 hdiutil 校验通过。复制至独立安装目录后从 `/`
  启动，CoreGraphics 窗口列表确认两个可见窗口；WebView 嵌入页面请求通过。
  同样的探针已保存为 `scripts/ci/desktop_project.py` 与 `visible_window.swift`。
- 浏览器项目使用锁文件执行 npm ci，统一 CLI 完成 Vite、TypeScript 与 Go 构建；
  `scripts/ci/browser_project.mjs` 验证真实 HTML、编译资源及标准 SDK 的 Unicode RPC。
  Node 测试客户端显式设置浏览器同源 Origin；未设置时服务器正确返回 403。
- 本地交付样本：`/tmp/omnigo-p0-go/releases/0.1.0`、
  `/tmp/omnigo-p0-webview/releases/0.1.0`。源码保存在对应项目目录。
- 新 CI 配置完成 YAML 解析；仓库尚未提交/推送执行，远程 CI 和分支保护未验收。
  窗口探针不覆盖完整鼠标/键盘交互、人工读屏或公证发布；本次无 Windows/Linux
  实机证据。项目自身许可证仍需所有者决定，没有代为选用开源授权条款。
