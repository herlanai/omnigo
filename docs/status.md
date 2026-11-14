# OmniGo 当前能力与支持状态

更新：2026-11-14。本文是当前能力状态的唯一汇总入口；migration 和 validation
中的带日期记录是历史证据，不覆盖本页。实现变化时同时更新本页与对应验收记录。
CLI `doctor` 输出的是当前构建启用的后端和本机工具检查，不等同于以下实机验收。

OmniGo 为 developer preview，API 尚未冻结。主验收平台为 macOS arm64。
“已实现”“构建通过”“原生探针通过”“人工验收”是不同层级，不能互相替代。

## 平台与界面

| 能力 | macOS arm64 | macOS amd64 | Windows amd64/arm64 | Linux amd64/arm64 |
| --- | --- | --- | --- | --- |
| CGO_ENABLED=0 默认目标 | 已构建 | 已构建 | 已构建 | 已构建 |
| 根包 Go 内容窗口 | AppKit，原生运行证据 | 已实现，未实机验证 | 未接入根包 GoWindow | 未接入根包 GoWindow |
| 底层软件窗口 | AppKit | AppKit，未实机验证 | 可选 Win32，未实机验证 | 可选 X11，未实机验证 |
| Web 主窗口 | WKWebView，原生探针通过 | WKWebView，未实机验证 | 可选 WebView2，已跨编译，未实机验证 | 可选 GTK4/WebKitGTK6，已跨编译，未实机验证 |
| Go 布局内 WebView | WKWebView | 已实现，未实机验证 | 未实现 | 未实现 |
| 原生 unary/batch RPC | WK message/reply，探针通过 | 已实现，未实机验证 | WebView2 message/reply，未实机验证 | 未实现，使用 HTTP |
| Go 控件原生 IME | NSTextInputClient，真实事件探针通过 | 未实机验证 | 未接入根包 Go 控件 | 未接入根包 Go 控件 |
| Go 控件系统读屏 | NSAccessibility，协议探针通过；人工 VoiceOver 待验收 | 未实机验证 | UIA 未实现 | AT-SPI 未实现 |

Windows 后端需 `omnigo_webview2` 构建标签、匹配架构的 WebView2Loader.dll 和
Evergreen Runtime。Linux 后端需 `omnigo_webkitgtk` 标签、GTK4/WebKitGTK6 库及显示环境。
这两种 Web 主窗口不代表 Go 内容窗口、菜单、系统服务或标题栏完整支持。
原生 RPC 不包括资源、事件订阅或流；这些仍使用 HTTP。

## 移动端（新增实验路径）

| 目标 | 构建产物 | 本地运行证据 |
| --- | --- | --- |
| iOS arm64 模拟器 | ad-hoc .app | iOS 26.5 模拟器首帧与截图通过 |
| iOS arm64 真机 | 未签名 .app | 未做设备签名/安装验收 |
| Android arm64 | SDK 35 清单、开发签名 APK | 构建、ABI、签名、对齐校验；未做设备运行验收 |
| Android amd64 | 开发签名模拟器 APK，已构建/校验 | 原生模拟器 CI 已配置，尚未运行 |

使用 `omni init --template mobile`、`omnigo.RunMobile` 和 `omni mobile build`。
仅移动原生宿主启用 cgo，桌面依然 CGO_ENABLED=0。Go 界面先落地；移动 WebView、
IME、系统读屏和完整手机交互仍未实现，不继承桌面的能力等级。见 [移动端指南](mobile.md)。

## 控件与性能

- Go 控件采用 CPU 绘制；macOS 主路径直接提交 RGBA。GPU 仍为可选实验能力。
- 86 个参考目录项维持 partial；这是参考能力对齐状态，不是生产质量认证。
- 文本已有持久化 rope、编辑历史、组合输入、Go 增量词法高亮。
  编辑仍物化全文字符串，高亮仍复制行索引；复杂整形与字素簇导航尚未完成。
- 树支持可选增量索引与局部 Invalidate；结构修改仍复制扁平数组。
  固定行高虚拟化不代表可变行高、滚动锚定或全流程常数复杂度。
- 系统输入/读屏探针不替代所有输入源、VoiceOver 人工导航及每个控件行为验收。

专项缺口保留在 [控件审计](../migration/controls/README.md)；测量范围见
[性能记录](performance.md)，原生证据见 [验证记录](validation.md)。

## 应用开发与交付

| 流程 | 当前可用 | 边界 |
| --- | --- | --- |
| `omni init --template go`（默认） | Go 内容窗口、独立状态、图标、omnigo.json | 默认 macOS arm64，不需要 Node |
| `omni init --template webview` | 嵌入静态 HTML 的桌面工作区、图标、omnigo.json | 默认 macOS arm64；示例本地交互，无 RPC；不需要 Node |
| `omni init --template browser` | TanStack/React/TypeScript、标准 SDK、Go HTTP 服务 | Node 24+、npm；浏览器应用，不创建桌面窗口 |
| `omni doctor` | 自动读取配置的 toolchain；也可显式选择 go/web/studio | 检查工具/库存在，不启动原生窗口；无配置默认 go |
| `omni dev` | 依据同一配置重建、失败保留旧进程、成功重启 | macOS/Linux；不是前端 HMR；Windows 未实现 |
| `omni build` | CGO_ENABLED=0、准备步骤、暂存后发布可执行文件 | 跨编译不代表目标平台可运行 |
| `omni package` | macOS .app、图标、许可证、ad-hoc 签名、带 Applications 快捷方式的 DMG | macOS 宿主；目标目录必须不存在；不含 Developer ID、公证或 CEF 打包 |
| 更新器 | 签名验证、单文件/.app 安装、恢复/回滚、重启确认 | .app 本地探针通过；需要应用组织 helper 与更新入口；就绪确认不是业务健康检查 |

完整起步流程见 [应用开发](getting-started.md)。当前生成项目通过本地 replace 引用框架；
尚未承诺稳定的已发布模块升级渠道。Windows/Linux 生成桌面应用并不属于模板验收范围。

## 工程质量与发布边界

- `scripts/check.sh` 是完整本地检查入口：Go 测试、vet、race、SDK 测试、六个默认
  目标与四个可选 Web 后端目标构建。scaffold 测试包含独立模块消费/构建验证。
- `.github/workflows/check.yml` 自动执行完整检查、真实浏览器模板构建、两个桌面
  模板的 macOS .app/DMG 打包及复制后可见窗口/嵌入资源探针。`native-desktop.yml` 单独执行原生探针。
- 新增工作流尚未取得远程执行结果，不能记录为 CI 已通过或 Windows/Linux 已验收。
  仓库分支保护是否要求这些检查由托管平台配置决定，本地文件不建立分支保护。
- 本项目自身的根目录许可证、API 兼容/弃用政策和正式发布流程仍待明确；
  第三方许可不能替代项目自身授权。当前打包用于本地验证。
