# Wails v3 对照升级工作清单

基线：Wails v3.1.0 全项目能力范围，非仅发布增量。OmniGo 工作区既有未提交变更予以保留。

本表是实施清单，不是完成声明。六项尚未全部完成。

| 项目 | 当前状态 | 验收条件 |
| --- | --- | --- |
| 1. 统一窗口、生命周期和前端桌面 SDK | 已有显式权限桌面查询/对话框 RPC，统一窗口与其余 SDK 待实现 | 稳定窗口 ID；操作能力查询；关闭拦截；销毁后拒绝调用；前端 Window/Dialog/Clipboard/Screen 接入；UI 回调无同步等待死锁 |
| 2. 通用 dev/build/package 和类型生成联动 | build、macOS/Linux dev 重建重启及 macOS package 已实现，完整 HMR/Windows dev/签名联动待实现 | 独立外部项目启动；Go/前端重建；子进程退出；生成失败保留旧文件；应用配置；临时 staging 和原子发布；macOS app/DMG 验证 |
| 3. Windows/Linux WebView 与原生 CI | GTK4/WebKitGTK 实验宿主及页面 CI 已接入，原生待验证；WebView2 待实现 | 下述平台专项矩阵全部有执行证据；交叉编译不计原生通过 |
| 4. 更新安装、恢复与重启 | 单文件签名安装/确认/回滚及显式崩溃恢复已实现，完整应用更新仍待实现 | 签名校验；同卷暂存；等待退出；原子替换；失败恢复；重启确认；并发安装和取消测试 |
| 5. 原生 Transport 与性能比较 | 待实现 | 与 HTTP 共享权限/取消/流控语义；导航来源隔离；二进制请求；窗口关闭清理；同负载延迟和内存记录 |
| 6. Go 控件输入、无障碍和大数据 | 已有上一轮局部改进，未完成 | 原生 IME、辅助技术、焦点与选择；大数据视口构建；长期生命周期与等价绘制性能验收 |

## Windows / Linux 分阶段验收

### 已加入的 CI

`.github/workflows/native-desktop.yml` 在 windows-2022 与 ubuntu-24.04 执行 `OMNIGO_NATIVE_TEST=1` 的窗口创建/绘制/退出和父子窗口测试。Linux 使用 Xvfb（真实 X11 服务，非 Wayland）；Windows 使用 runner 桌面环境。JSON 测试输出保留为 artifact，脚本拒绝 skipped/missing/failed 测试。

本地仅验证证据检查器，不代表远程 workflow 已运行通过。托管 Windows runner 的交互桌面能力也以实际执行结果为准。

### Windows WebView2

- 使用 CGO=0 COM 调用；明确 STA 线程和 callback 所有权，运行时缺失报可诊断错误。
- 环境/控制器异步创建，窗口关闭时取消并释放迟到回调。
- 导航、resize、DPI、焦点、二进制请求、桥接来源限制和 WebView2 Runtime 检测。
- 在真正 Windows runner 验证页面 JS → Go → JS 回路、取消、文件选择和关闭；不能以 Win32 软件窗口测试替代。

### Linux WebKitGTK

- 先选定 GTK4/WebKitGTK 6.0 最低版本和动态符号清单，缺依赖明确报告；保持 CGO=0。
- GTK 主循环、引用计数、信号回调、WebKit 请求取消与页面关闭必须成对清理。
- Xvfb 下验证 WebView 加载与桥接；另设 Wayland compositor 验证，不能把 Xvfb 结果写为 Wayland 支持。
- 不把测试环境可能需要的 sandbox 例外写入生产默认配置。

两平台共同验收：默认入口实际创建 WebView；资源可用；缺失资源 404；Unicode 与二进制 RPC；事件/流取消；导航权限；窗口退出后无活动服务或子进程。功能未实现前保留 Platform 的 unavailable 状态。

## 通用 build 首批实现

omni build -config omnigo.json 支持显式目标、配置目录相对路径、前置参数数组命令及同目录暂存发布。编译失败、缺失输出及取消保留旧可执行文件；不是完整 app/DMG 打包。详见 internal/appbuild/README.md。

## macOS package 首批实现

同一 omnigo.json 可配置 bundle 名称、ID、版本、ICNS 图标、许可证清单和新发布目录。omni package 构建并检查架构，创建 app、ad-hoc 签名、创建和校验 DMG；已有发布目录拒绝覆盖。临时独立模块生成 app/DMG、签名校验、复制后执行及 arm64 检查通过；完整 scripts/check.sh 通过。此测试模块没有 GUI，不代表窗口交互或 Dock/Finder 图标实测。Developer ID、公证、CEF 运行时打包、Windows/Linux 安装包仍待实现。

## dev 首批运行流程

显式源码路径内容哈希监听、串行重建、失败保留旧进程、成功后进程组重启、SIGINT 清理。可通过 before 串联既有类型生成。watch/args 修改需重启会话，生成输出必须排除；不是前端 HMR。Windows Job Object 与脱离会话的子进程管理仍缺失。
