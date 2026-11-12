# 完整功能覆盖清单

参考仓库：本地 `golet`，只读扫描。初始记录包含 **2,412 个源码/文档文件指纹、4,517 条 Go 导出声明和 16 个插件目录**。声明数量包含条件编译变体，并不等于独立功能数量。

`reference.json` 记录接口声明与源文件 SHA-256，不包含函数实现体，不导入旧框架。Go 导出清单排除 internal、cmd、examples 与测试文件；这些目录及前端的源码指纹仍记录在 files 中。**TypeScript 导出、CLI 行为、视觉状态与平台行为尚需进一步建立行为级清单。当前清单不能独自证明完整覆盖。**

初始参考快照中的声明保持 `pending`，不把它当作实时完成率。当前实现与差距分别记录在 [控件审计](controls/README.md)、[原生平台审计](platform/README.md) 和 [childwindow/clipboard/diff/e2e/GPU/多语言专项](features/README.md)。这些清单互补，不得只统计导出 API 而漏掉内部算法、资源生命周期和原生测试。Web UI 的 OmniWeb 协议与默认栈进度另见 [Web UI 行为覆盖](webui.md)。

## 必须覆盖的功能域

`backend/internal` 的 42 个目录另有[全目录范围与实现记录](internal/README.md)，保留原生源码、内部算法和测试索引。新增文本存储、快捷键、下载的真实实现及 Studio 演示在该记录中单独验收，目录覆盖不等于行为完成。

| 功能域 | 重写位置规划 | 当前状态 |
| --- | --- | --- |
| 生命周期、主线程调度、事件 | desktop / runtime | 新核心已实现；参考行为尚未逐项验收 |
| 窗口、多窗口、屏幕、菜单、托盘、快捷键 | desktop / 原生驱动 | 已有实验 Cocoa 与 Win32/X11 软件窗口；父子窗、屏幕、菜单、托盘等仍缺，见平台与专项审计 |
| 系统 WebView、Web IPC、信任边界、资源协议 | platform / runtime | 仅类型化调用核心已实现 |
| 原生 UI、布局、主题、控件、动画、焦点、IME、无障碍 | omniui/baseui | 已有基础 Go 行列布局、控件行为、焦点、文本缓冲、无障碍快照；系统 IME/无障碍待接入 |
| Metal、D3D、OpenGL、CPU、浏览器渲染 | omniui/baseui 绘制层 | Go CPU 绘制与可选 Metal 矩形 compute 渲染已实现；完整 GPU 控件绘制和其他后端仍待实现 |
| 图表、代码/富文本/Markdown 编辑器 | omniui/baseui / 可选插件 | 部分实现，DiffView 与独立行级 diff 已有；不代表高级编辑或完整算法覆盖 |
| Forma 与 AI 组件、加载与特效组件 | omniui/baseui | 多项局部实现，逐项差距见 controls；未宣称完整对齐 |
| 音频、视频、拖放、剪贴板、文档、国际化 | omniui / desktop | 剪贴板仅编辑操作与 Studio Mac 文本通路；完整数据传输和多语言仍缺 |
| TS 生成、CLI、热重载、项目模板、打包、更新 | cmd / tooling | 待实现 |
| 原有示例、文档、浏览器与桌面演示 | examples / studio | 已有离屏画廊、Studio 与 native-window 示例；非全部参考示例覆盖 |
| 全部 16 个插件及前端客户端 | 控件插件归属 omniui/baseui；服务插件使用统一注册协议 | loading 已有 3 种新 Go 绘制，原 29 种及其他插件待完整重写 |

插件清单：automation、borderbeam、chromium、dsh-go、fetch、game、glass、loading、permissionflow、scene3d、sfsymbols、sqlite、terminal、updater、watch、websocket。

## 验收次序

1. 完善行为级清单，建立原参考测试与新 API 契约的映射；保留所有尚未实现项目。
2. 打通真实桌面垂直路径：应用启动 → 窗口 → 输入 → 布局 → 绘制 → 关闭，先验证当前 macOS，再实现 Windows/Linux。
3. 补齐系统 WebView 和 IPC 客户端，再建立基础控件的视觉、键盘、IME、无障碍契约。
4. 迁移高级控件与所有插件，同时验证资源释放、取消与平台支持矩阵。
5. 完成 CLI、模板、打包签名、开发工具和示例，使完整应用的创建与发布可复现。
6. 在同机同负载下比较性能、故障注入结果和开发流程耗时，逐项报告是否达到目标。

每一项标记完成前须提供新实现路径、行为测试、适用平台、已知差异及基准记录。旧实现中尚不完整的功能也要记录，不默认其文档描述就是已验证行为。

代码归属和依赖次序见 [专项整理](features/README.md#代码归属)。`i18n`、`omniwindow/clipboard`、`omniui/baseui/gpu` 和父子窗口代码已落地；各自的实现边界、剩余能力和原生证据分别记录，不能把部分实现当成全部参考覆盖。

## 更新参考快照

```sh
go run ./tools/parity -source /absolute/path/to/golet -out /tmp/omnigo-reference-next.json
```

先审查与 `reference.json` 的差异，再替换快照；不要把重新生成的 pending 状态当作迁移进度更新。参考文件扫描排除隐藏目录、node_modules、dist、build、vendor 和符号链接。

全新实现采用 OmniGo/OmniUI 命名。若未来决定复用第三方代码、字体、图标或素材，仍须保留相应许可证与来源；改品牌不能代替来源管理。
