# 完整功能覆盖清单

参考仓库：本地 `golet`，只读扫描。初始记录包含 **2,412 个源码/文档文件指纹、4,517 条 Go 导出声明和 16 个插件目录**。声明数量包含条件编译变体，并不等于独立功能数量。

`reference.json` 记录接口声明与源文件 SHA-256，不包含函数实现体，不导入旧框架。Go 导出清单排除 internal、cmd、examples 与测试文件；这些目录及前端的源码指纹仍记录在 files 中。**TypeScript 导出、CLI 行为、视觉状态与平台行为尚需进一步建立行为级清单。当前清单不能独自证明完整覆盖。**

所有参考声明的状态为 `pending`。已经写出的新核心是独立基础，不因为名称或类型接近就自动计为兼容实现。Web UI 的 OmniWeb 协议与默认栈进度另见 [Web UI 行为覆盖](webui.md)。

## 必须覆盖的功能域

| 功能域 | 重写位置规划 | 当前状态 |
| --- | --- | --- |
| 生命周期、主线程调度、事件 | desktop / runtime | 新核心已实现；参考行为尚未逐项验收 |
| 窗口、多窗口、屏幕、菜单、托盘、快捷键 | platform / desktop | 待实现 |
| 系统 WebView、Web IPC、信任边界、资源协议 | platform / runtime | 仅类型化调用核心已实现 |
| 原生 UI、布局、主题、控件、动画、焦点、IME、无障碍 | omniui/baseui | 已有基础 Go 行列布局、控件行为、焦点、文本缓冲、无障碍快照；系统 IME/无障碍待接入 |
| Metal、D3D、OpenGL、CPU、浏览器渲染 | omniui/baseui 绘制层 | 基础 Go CPU 绘制已实现，其余待实现 |
| 图表、代码/富文本/Markdown 编辑器 | omniui | 待实现 |
| Forma 与 AI 组件、加载与特效组件 | omniui | 待实现 |
| 音频、视频、拖放、剪贴板、文档、国际化 | omniui / platform | 待实现 |
| TS 生成、CLI、热重载、项目模板、打包、更新 | cmd / tooling | 待实现 |
| 原有示例、文档、浏览器与桌面演示 | examples / docs | 只有新核心无界面示例 |
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

## 更新参考快照

```sh
go run ./tools/parity -source /absolute/path/to/golet -out /tmp/omnigo-reference-next.json
```

先审查与 `reference.json` 的差异，再替换快照；不要把重新生成的 pending 状态当作迁移进度更新。参考文件扫描排除隐藏目录、node_modules、dist、build、vendor 和符号链接。

全新实现采用 OmniGo/OmniUI 命名。若未来决定复用第三方代码、字体、图标或素材，仍须保留相应许可证与来源；改品牌不能代替来源管理。
