# OmniUI 可选插件

所有插件的规范路径为 `github.com/herlanai/omnigo/omniui/plugins/<name>`，按应用实际需要导入。该目录没有统一导入全部插件的注册器，不会因导入 `baseui` 而链接 SQLite、WebSocket、PTY 或 AppKit。用户指定的新目录覆盖此前仓库规则中的可选控件插件路径约定。

**16 个目录均有部分代码，但没有任何插件达到 golet 全量对齐。** 完整能力与缺口逐项保留在 [status.json](status.json)。Chromium 已按用户明确要求直接移植 Golet 的 CEF 浏览器与配套工具，保留 MIT 许可证和署名，macOS 实际浏览器测试已通过；Terminal 有真实 Unix PTY，但尚无完整 VT 模拟器和交互视口。这些边界不能由测试、目录数量或跨平台构建代替。

| 插件 | 本批实际能力 | 主要缺口 |
| --- | --- | --- |
| automation | 私有 socket、UI 调度、控件快照/输入/截图 | 多窗口、菜单/页面、settle、CLI |
| borderbeam | 五种色场/辉光模式、配色、暂停/隐藏、强度/周期/范围、Studio 同结构演示 | 参考色场精确系数、完整色相参数及逐像素/实机对比；见 [对齐记录](../../migration/controls/borderbeam.md) |
| chromium | CEF 原生浏览器、标签页、导航、主题、Chrome 扩展、CDP、助手、打包与 Studio 启动 | Windows/Intel 原生验收、Linux 宿主、布局内嵌视图 |
| dsh-go | Go Panel、omnix Agent、工具审批、会话历史、取消与预算 | 虚拟列表、原生设备验收、持久化恢复、完整 skills/subagent/plan 能力 |
| fetch | 有界流式 HTTP、重定向策略、取消 | OmniWeb 客户端和页面生命周期 |
| game | 固定步长、暂停、相机和整数缩放 | 输入累积、手柄、完整 View |
| glass | 真实 CPU 背景采样、模糊/折射、透明度回退 | 融合面板、GPU、原生材质、滚动边缘 |
| loading | Ring、Dots、Bars、Clock、Ripple、Orbit、Pulse | 其余样式及参考关键帧 |
| permissionflow | macOS 两类权限查询及屏幕录制请求 | 其他权限、引导面板、Windows |
| scene3d | 透视、六平面裁剪、深度测试 | 网格/实例、光照、OBJ、轨道控制、缓存 |
| sfsymbols | AppKit 系统符号转自有位图 | 完整样式/主题和实机验收 |
| [sqlite](sqlite/README.md) | GORM 模型迁移、泛型 CRUD、分页筛选、事务、原生 SQL、Studio 交互演示 | 前端桥、完整选项/资源策略 |
| terminal | macOS/Linux PTY、有限 VT 屏幕、颜色/光标/备用屏和原生输入视口 | 完整 VT、宽字符、选择/搜索/IME、Windows ConPTY |
| updater | 签名清单、校验下载与失败清理 | 更新窗口、安装/回滚/重启和偏好 |
| watch | 根目录隔离的轮询差分、重命名提示 | 原生监听、debounce、前端桥；瞬时变化可能漏报 |
| websocket | 文本/二进制连接、协议协商、限额/取消 | 浏览器式发送队列、前端桥和页面生命周期 |

## 使用和生命周期

网络、数据库、PTY 操作在应用 worker 中调用；返回结果通过 `internal/runtime/dispatch` 交回 UI。绘制插件和其状态仅在 UI goroutine 使用，不保留 Painter。所有服务由调用方显式启动/关闭；导入不启动服务、不打开网络、不请求系统权限、不下载原生代码。

Loading 已统一到 `omniui/baseui/loading`，实现和测试均在该目录，`omniui/plugins/loading` 已移除。插件清单保留参考能力审计，指向实际实现位置。本轮没有复制 golet 实现体，也没有导入 golet。

SQLite 使用可选的 modernc.org/sqlite 纯 Go 驱动；WebSocket 使用 coder/websocket；Unix PTY 使用 creack/pty。许可证见 [licenses](licenses)。它们与 golet 采用的依赖路线可能不同，完整参考行为仍需逐项验证。

SF Symbols 和权限 API 的原生调用仅限相应平台。没有用其他平台的空返回值冒充成功。SF Symbols 需要真实应用的主 UI 线程；测试仅覆盖参数校验，不宣称完成 AppKit 实机渲染验收。

Chromium 的 `Launch` 使用可选安装的 CEF 原生宿主。`pnpm setup:chromium` 构建宿主，`pnpm dev:browser` 启动；Studio 打包时可附带运行时。macOS E2E 覆盖网页、历史、标签页、JS/CDP、Chrome 扩展、应用模式和助手操作。详见 `chromium/README.md` 与 `chromium/PORT.md`。

运行 `CGO_ENABLED=0 go test ./omniui/plugins/...` 和仓库完整的 `sh scripts/check.sh`。Studio 新增 Borderbeam、Glass、Scene3D、Game 演示；目录状态保持 partial，服务插件不会误指向同名基础控件。

Markdown 阅读组件位于 `omniui/plugins/markdown`，通过可选 Goldmark 解析器组合 Go 原生控件。实现、测试和许可证均在该目录；Studio 保留 Markdown 演示入口，能力缺口继续由 `migration/controls/advanced.json` 审计。

新增独立服务演示 **OmniBrowser**（非 golet 原有 16 项之一）：核心位于 `omniui/plugins/omnibrowser`，提供纯 Go HTTP/DOM/JS 与抓取；可选 `omniui/plugins/omnibrowser/render` 提供 Go 语义预览，Studio 入口为 `plugin/omnibrowser`。它不是现有 Chromium 插件的替代，也没有完整浏览器兼容性；具体能力与限制见 `omniui/plugins/omnibrowser/README.md`。
