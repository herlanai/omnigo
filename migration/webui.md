# Web UI 行为覆盖

用户要求：Web UI 与原生 UI 并列支持；Web 构建统一使用 OmniWeb；默认 TanStack Start + shadcn；完整覆盖参考框架且性能更好。

本表依据本地参考的 `frontend/packages/runtime`、`frontend/packages/bridge`、Web 模板及核心 Go API 建立。它补充 Go 导出快照，不以 HTTP 桥接成功代替全部功能完成。

| 参考行为 | OmniWeb 位置 | 当前验收状态 |
| --- | --- | --- |
| Go 服务请求/响应 | omniweb + internal/runtime/rpc | HTTP 与 TS SDK 已实现；桌面 WebView 传输待实现 |
| 错误类型、请求关联、取消 | omniweb/client | 已实现基本协议及取消 |
| 类型安全客户端生成 | 模板显式 Services 契约 | 手写类型可用；自动生成待实现 |
| 前端事件 on/once/event | 待规划事件传输 | 待实现，不能用 Go 内部 Topic 冒充 |
| Channel/AsyncIterable 流 | 待规划流协议及背压 | 待实现 |
| locale/direction/语言变更与消息格式化 | OmniUI 国际化与 OmniWeb 通知 | 待实现 |
| 主题偏好、窗口活跃状态 | 平台事件 + OmniWeb | 待实现 |
| currentWindow/窗口控制/拖拽标题栏 | 平台驱动 + OmniWeb | 待实现 |
| 文件拖放、文档状态与重载 | 平台驱动 + OmniWeb | 待实现 |
| 上下文菜单、按键转发、网页查找、内容尺寸 | WebView 适配层 | 待实现 |
| WebView 页面导航、脚本、cookies、内容规则、安全域 | 平台驱动 | 待实现 |
| 插件前端客户端 | OmniUI 扩展 + OmniWeb 服务 | 全部待逐项重写 |
| TanStack Start SPA/shadcn 默认模板 | internal/scaffold + cmd/omni | 生成、npm ci、类型检查、生产构建及浏览器到 Go 调用通过 |
| 静态资源与 SPA 深链接 | omniweb.Assets | 已实现契约测试 |
| 桌面热重载、类型生成、打包、签名、更新 | CLI 与平台工具链 | 仅项目生成已实现 |
| 浏览器和三平台系统 WebView | HTTP 浏览器宿主 + 后续适配器 | 浏览器路径可构建；系统 WebView 尚未接通 |

更新任何条目时，需要实现、测试和验证环境证据；性能列必须独立于功能列验收。不要求沿用原协议、包名或目录，但不得无说明删除原有能力。
