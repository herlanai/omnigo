# OmniGo · OmniUI

面向 PC 桌面应用的新 Go 框架，当前处于**核心原型阶段**。OmniGo 管理应用与运行时；OmniUI 是控件、服务插件与渲染扩展共同使用的体系名称。

本仓库从空目录开始编写，没有依赖、复制或换名导入参考框架实现。Go module 路径为 `github.com/herlanai/omnigo`。

**目前不是可替换 golet 的完整桌面框架，尚未达到完整功能覆盖与性能领先目标。** 已有可选 Win32/X11 窗口、macOS Studio 宿主和局部 Metal 绘制实现，完整平台行为与 GPU 控件管线仍待补齐。`omniui/baseui` 提供 Go 控件、CPU 绘制与交互测试；OmniWeb 默认模板可构建浏览器界面，生成模板尚不打开原生桌面窗口。

**所有框架控件统一用 Go 实现，归属 `omniui/baseui`**。`ui`、`forma` 与控件插件的能力在此重写；Web 模板和 shadcn 配置不算 Go 控件移植成果。见 [baseui](omniui/baseui/README.md) 和 [控件审计](migration/controls/README.md)。

Web UI 统一通过 **OmniWeb v1** 协议构建，默认支持 **TanStack Start + shadcn**。创建和运行方式见 [OmniWeb 指南](docs/omniweb.md)，完整覆盖状态见 [Web UI 迁移表](migration/webui.md)。

## Studio 客户端

```sh
pnpm install --frozen-lockfile
pnpm dev       # 构建并打开内嵌 Mac 客户端
pnpm build     # 生成 build/studio/Omni Studio.app
pnpm start     # 运行构建结果
pnpm package   # 生成应用归档
```

Studio 主窗口内嵌 WKWebView，目录资源编译进 Go 可执行文件；不需要外部浏览器、单独 Web 服务或生产 Node 进程。34 个局部实现控件和 loading 插件可打开 Go 原生演示，全部 86 个控件项和 16 个插件保持可追踪。当前客户端仅支持 macOS Apple Silicon。详见 [Studio](studio/README.md)。

## 开始

需要 Go 1.26 或更新版本。原核心使用标准库；baseui 的字体与 CPU 绘制依赖纯 Go 的 `golang.org/x/image`。Web 模板验证另需 Node.js 24+。

```sh
go test ./...
go run ./examples/lifecycle
go test -race ./...
go test -run '^$' -bench . -benchmem ./omniui ./runtime/event
```

示例验证扩展启动与清理、类型化调用、界面局部更新与百万行数据的可见范围计算。

## 已实现

| 能力 | 当前行为 |
| --- | --- |
| OmniUI 扩展注册 | 控件、服务、渲染器共用描述协议；拒绝重名、缺失依赖、依赖环和缺失授权 |
| 应用生命周期 | 确定性启动顺序、失败逆序回滚、独立清理上下文、异常隔离、单次运行 |
| 主线程任务队列 | 有界容量、唤醒合并、每轮工作上限、取消与关闭、回调可再次提交任务 |
| 类型化调用 | 泛型请求/响应、严格 JSON 字段检查、权限校验、请求大小与并发限制 |
| 类型化事件 | 有界订阅队列、明确的丢弃计数、并发取消及关闭 |
| OmniUI 界面树 | 不可变节点、稳定兄弟键、子树共享、增量属性和结构补丁 |
| 虚拟列表算法 | 固定行高可见范围与 overscan 计算；不是完整列表控件 |
| 迁移清单 | 参考源码指纹、Go 导出声明、插件目录清单；全部保留待迁移状态 |
| OmniWeb | HTTP 请求/响应、批处理、TS SDK、取消、来源/权限/容量校验 |
| 默认 Web 模板 | `omni init` 生成 TanStack Start SPA + shadcn 配置 + Go 浏览器宿主 |

权限声明仅约束框架管理的调用，不是对任意 Go 插件的操作系统沙箱。扩展取消与停止是协作式的；超时上下文不能强行终止不遵守协议的 Go 函数。

## 目录

```text
apps/             应用生命周期与平台驱动契约
omniui/              统一扩展协议、不可变界面树、差分、虚拟化
omniui/baseui/       Go 控件、布局、输入、主题、CPU 绘制与控件插件
omniweb/             Web 协议、HTTP/静态资源宿主、唯一 TypeScript SDK
studio/              内嵌 Mac 客户端、Go 演示、目录与打包资源
cmd/omni/            项目生成 CLI
internal/scaffold/   默认 TanStack Start + shadcn 模板
runtime/dispatch/    主线程工作队列
runtime/event/       类型化事件
runtime/rpc/         类型化服务调用
runtime/download/    共享下载校验、暂存、重试与跨进程缓存
text/rope/           共享不可变文本版本与坐标算法
i18n/                语言目录和消息格式
examples/lifecycle/  无界面核心示例
tools/parity/        参考功能清单生成器
migration/           源码指纹、迁移状态与验收范围
docs/                新架构与性能验收规则
scripts/             可重复验证入口
```

原生窗口驱动归属 `apps/nativewindow`，系统服务按职责放在 `apps` 子包；控件、主题、编辑器、图表、绘制适配和可选控件能力统一归属 `omniui/baseui/`。共享算法与基础服务由上层复用，未实现的目录不提前填充空壳。具体依赖方向和复用边界见 [架构说明](docs/architecture.md)。

## 完整重写与验收

见 [迁移范围](migration/README.md)、[架构约束](docs/architecture.md) 和 [性能口径](docs/performance.md)。迁移过程中保持本地参考仓库只读。原有 API 用于追踪行为覆盖，不要求沿用旧名称与目录结构。

本次实际执行结果见 [验证记录](docs/validation.md)。

“100 倍”需要逐场景基准证明；不能把某一算法的复杂度改善、跨平台编译成功或无界面测试通过，描述成整套桌面框架的速度、兼容性与稳定性已经达标。
