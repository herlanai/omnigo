# OmniWeb v1

OmniWeb 是 OmniGo 的 Web UI 协议。OmniUI 保持统一的控件、主题与扩展体系；Web 界面通过 OmniWeb 访问 Go 服务，通过公开的 `omniweb.NewRouter`、`omniweb.Bind` 和 `omniweb.NewHTTP` 组合宿主，内部复用原生界面的 RPC 实现。外部应用不需要也不能导入 `internal/runtime`。

当前实现覆盖 HTTP 请求/响应与批处理、TypeScript 客户端、SPA 静态资源宿主和默认项目生成。尚未实现系统 WebView 适配、事件订阅、流式 Channel、自动类型生成与完整系统 API。协议版本为 1；本阶段仍是开发协议，未承诺稳定发布。

## 默认构建方式

默认栈是 **TanStack Start + React + TypeScript + shadcn 配置 + Tailwind CSS**。Start 采用 SPA 模式生成 `ui/dist/client/index.html`；Go 只托管 client 目录，业务逻辑由 Go 执行，生产运行不启动 Node 服务。

```sh
go run ./cmd/omni init \
  --framework "$PWD" \
  --module example.com/myapp \
  /tmp/my-omniweb-app
cd /tmp/my-omniweb-app
npm --prefix ui ci
npm run build:web
npm start
```

打开 `http://127.0.0.1:7331`。当前这是浏览器宿主，不是原生桌面窗口。目标路径必须不存在；CLI 不覆盖已有项目。`--framework` 是本地替换路径，发布前不能省略。

开发时分别执行 `npm run dev:host` 和 `npm run dev:web`，访问 `http://127.0.0.1:5173`。Vite 将 `/omniweb/` 代理到 Go，浏览器仍使用同源请求。默认没有时钟或状态轮询。

shadcn 目录别名、CSS 主题变量与 `components.json` 已配置，模板包含按相同约定编写的 Button/Input。可以在 `ui` 目录执行 `npx shadcn@latest add dialog` 添加官方组件。这不表示整个 shadcn 控件库已被内置，也不表示 Web 与原生控件已经视觉一致。

配置参考：[TanStack Start SPA 模式](https://tanstack.com/start/latest/docs/framework/react/guide/spa-mode)、[shadcn TanStack Start 安装](https://ui.shadcn.com/docs/installation/tanstack)。

## 线协议

单次调用：`POST /omniweb/v1/call`，Content-Type 为 `application/json`。

```json
{"v":1,"id":"42","method":"app.greet","params":{"name":"OmniWeb"}}
```

成功：

```json
{"v":1,"id":"42","result":{"message":"你好，OmniWeb。"}}
```

失败：

```json
{"v":1,"id":"42","error":{"code":"forbidden","message":"Permission denied"}}
```

`result` 与 `error` 必须且只能出现一个；`result: null` 是合法成功。客户端必须校验版本、请求 ID 和结果形状。契约泛型只提供编译期结果类型，不会自动进行业务数据的运行时 schema 校验。

批处理：`POST /omniweb/v1/batch`，请求体是请求数组，响应按输入顺序返回。批量内 ID 不得重复。全部信封在执行前校验；方法本身逐个执行，失败不会回滚已经发生的业务修改。批处理不是事务，也没有自动重试。

错误码：`invalid_params`、`not_found`、`forbidden`、`busy`、`too_large`、`canceled`、`deadline`、`internal`。普通业务错误与 panic 不向浏览器泄露原始错误文本。需要公开业务错误详情的应用应将其设计成显式响应类型。

信封或 HTTP 不合法时返回非 2xx 状态及 `{code,message}`，不执行服务。合法调用中的方法错误返回 200 和带 ID 的 error 响应。

## 容量与安全边界

- 默认请求上限 1 MiB、响应发送上限 4 MiB、批处理上限 64。响应上限在编码后检查，**不能阻止业务函数自己分配过大的结果**。
- Go Router 默认最多 64 个正在执行的调用；TS 客户端默认最多 64 个在途调用。过载返回错误，不无限排队。
- 默认客户端超时 30 秒，允许 0–600 秒之间的正数；取消通过 AbortSignal → fetch → HTTP 请求上下文传播。Go 服务与自定义 Transport 必须协作处理取消。
- 宿主精确匹配 Host 与 Origin，拒绝缺失/不可信来源、错误媒体类型与压缩请求体；不提供通配 CORS。Vite 代理仅用于开发，不放宽生产来源。
- 权限只从宿主初始化配置传入；请求不接受 grants 字段。权限属于进程内服务策略，不是任意 Go 代码的 OS 沙箱。
- 默认生成程序只绑定 loopback。Origin 校验用于浏览器来源隔离，**不等同于用户身份认证**；本机任意进程可主动构造 HTTP 请求。远程部署需另行实现身份认证和逐用户授权。
- 静态宿主只能接收可信的 client 文件系统；不应指向项目源码或含敏感文件的目录。导航请求可回退 SPA shell；不存在的脚本、隐藏路径、协议路径不会回退。

## 性能口径

批处理减少 HTTP 往返次数，复用 Go 路由避免第二套业务分发器。SDK 不引入第三方运行时依赖，空闲时没有周期定时器。以上是结构特征，不是相对参考框架更快的实测结论。

后续基准必须比较同一业务负载的 IPC 延迟、吞吐、序列化分配、首次可交互时间、开发冷/热构建和空闲内存。当前没有可证明“优于 golet”的等价对照数据。
