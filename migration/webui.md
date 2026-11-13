# Web UI 行为覆盖

用户要求：Web UI 与原生 UI 并列支持；Web 构建统一使用 OmniWeb；默认 TanStack Start + shadcn；完整覆盖参考框架且性能更好。

本表依据本地参考的 `frontend/packages/runtime`、`frontend/packages/bridge`、Web 模板及核心 Go API 建立。它补充 Go 导出快照，不以 HTTP 桥接成功代替全部功能完成。

| 参考行为 | OmniWeb 位置 | 当前验收状态 |
| --- | --- | --- |
| Go 服务请求/响应 | omniweb + internal/runtime/rpc | HTTP 与 TS SDK 已实现；桌面 WebView 传输待实现 |
| 错误类型、请求关联、取消 | omniweb/client | 已实现基本协议及取消 |
| 类型安全客户端生成 | 模板显式 Services 契约 | 手写类型可用；自动生成待实现 |
| 前端事件 on/once/event | omniweb.Events + POST /omniweb/v1/events | Go HTTP NDJSON 事件传输、权限、队列上限、退出取消已实现；TS on/once/event 已接入；macOS ARM64 WKWebView 端到端已验收；其他平台与长期运行仍待验收 |
| Channel/AsyncIterable 流 | omniweb.Streams / BindStream / Channel | Go 请求级 NDJSON 流及 TS AsyncIterable 已实现；消费确认背压和原生全链路仍待完成 |
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


事件传输增量：`omniweb.NewEvents()` 创建应用拥有的 broker，Register 定义主题及
权限，Publish 快照序列化负载；`HTTPOptions.Events` 或根 `Options.Events` 接入
宿主。订阅 POST 请求为 `{"v":1,"topics":["progress"]}`，返回 NDJSON ready/event/error
帧。每条事件最多 256 KiB，每个订阅队列 64 条，每个 broker 最多 128 个订阅；溢出
明确关闭订阅并发送 overflow，发布方收到断开计数。没有重放或自动重连承诺。
应用 HTTP server 的 BaseContext 绑定窗口生命周期，退出先取消流再 Shutdown。
测试覆盖真实 HTTP ready/数据/结束、权限、快照和溢出；浏览器 SDK 与原生 WebView
全链路仍未验收。这不是 Channel/AsyncIterable 双向流能力的完成证据。


TypeScript 事件 API：`OmniWebEvents<{progress: number}>` 提供 `on`、`once` 和
`event(name)`；返回可调用的取消函数，其 `ready` 在服务端确认订阅后完成，`done`
报告结束或错误。可用 AbortSignal 或 client.close 清理。SDK 按字节流解析 UTF-8/
NDJSON，限制单帧 1 MiB、每客户端 64 个订阅；回调异常不丢失，once 在调用回调前
取消以阻止同批重复事件。无隐式重连、重放或跨标签页共享。Node 模拟流测试不等于
浏览器 WebView 原生全链路验收；事件类型仍是编译期契约，不是负载运行时 schema。


跨语言实测增量：`TestTypeScriptEventsAgainstGoHTTP` 用 Node 执行规范 SDK，连接
真实 Go HTTP server，经 RPC 触发事件，验证顺序、Unicode、once、权限拒绝、取消
和 broker 关闭；另有请求取消后订阅释放测试。`TestApplicationShutdownCancelsEventStreams`
验证公共应用退出时服务端结束活跃事件连接、清理订阅，并保留调用方拥有的 broker。
这些测试不用 sleep，不是模拟 fetch；但仍不是浏览器/WebView 原生端到端证据。
运行需 Node.js 24+，缺少 Node 时跨语言用例显式跳过，不能计作已通过。


macOS ARM64 原生事件验收：`examples/omniweb-events-check` 使用公共应用 API，加载
由规范 `omniweb/client/index.ts` 编译的 ES module，在真实 WKWebView 中验证顺序、
中文/Emoji、once、权限拒绝、AbortSignal 取消和 broker 关闭，再经 RPC 回报结果并
自动关闭窗口。2026-11-12 本机运行通过。编译产物位于临时目录，不复制或分叉 SDK。

可复现（需安装 TypeScript 编译器；`tsc` 来自工具链，不是 golet 依赖）：

```sh
sdk_dir=$(mktemp -d)
tsc --strict --target ES2022 --module ESNext --lib ES2022,DOM --skipLibCheck \
  --outDir "$sdk_dir" omniweb/client/index.ts
CGO_ENABLED=0 go run ./examples/omniweb-events-check -sdk "$sdk_dir/index.js"
```

探针有 30 秒截止时间；没有结果会失败，不把启动窗口本身计为通过。尚无 Intel Mac、
Windows/Linux WebView 的对应原生结果，也没有事件重放、自动重连或长期压力证据。


流式方法增量：`NewStreams` + `BindStream[Request, Value, Result]` 注册方法，
`HTTPOptions.Streams` / 根 `Options.Streams` 接入 `POST /omniweb/v1/stream`。
请求沿用 v/id/method/params，响应 ready/value/end/result 严格按序。Channel.Close
先结束值流，方法仍可继续计算最终结果；方法返回后自动结束值流。每值上限 256 KiB，
最终结果上限 1 MiB；默认并发 16，满时拒绝。写操作串行并有截止时间，断连/请求取消
传给方法 context，方法需协作退出；未引入后台无界消息队列。权限与参数在执行前校验，
panic 返回通用错误。页面消费确认窗口和 TS Channel 已由下述增量实现；
原生流式验收与其余差异仍保留，不以 HTTP/TCP 背压替代应用层消费确认。
参考 golet Channel 是服务端流值、页面确认或取消，并非任意双向用户消息通道。


TypeScript 流式 API 增量：`OmniWebStreams<API>.call(method, params)` 返回支持共享消费的
AsyncIterable，附带 ready、result、close。正常消费值流后等待 result；提前 break、
显式 close、AbortSignal 或 client.close 取消网络请求并拒绝最终结果。客户端每次只
保留一个待消费值，消费确认后推进解析。end 结束迭代，之后的方法
错误由 result 报告。严格校验版本/请求 ID/帧顺序与最终结果，限制帧大小和 64 个并发。
新增 Node 用例覆盖正常顺序、Unicode、提前退出、未消费关闭、方法错误及协议异常；
与 Go 服务的跨语言验证及 macOS ARM64 WKWebView 验证见下；其他平台
仍待验收，不沿用普通事件的验收结论。


### Channel 页面消费确认（2026-11-12）

重新核对参考 Channel 的消费窗口语义，新增每个活跃调用的随机 token、递增 seq
及同源 POST /omniweb/v1/stream/ack。未确认值按 JSON 字节数加每帧 256 字节
元数据预算计入 1 MiB 窗口，达到上限后 Send 等待确认或 context 取消。
重复确认不重复释放额度，未来序号拒绝；ACK 复用 Host/Origin/权限检查。
方法返回后按序发送 end/result 并清理会话；SDK 消费此前值之后才能取得结果。
已结束会话的 ACK 返回 410，SDK 继续解析有限尾部数据，仍必须验证最终结果帧。

规范 SDK 在迭代器取值时确认，再交付该值；确认失败、提前退出和 AbortSignal
终止请求。服务端处理的是消费额度，不是任意双向用户消息。当前每值一次 HTTP
确认；参考按半窗口批量确认，吞吐与延迟尚未等价验证，不能宣称性能对齐。
单值 256 KiB 限额、UI 线程调用约束、onmessage 风格接口和其他平台原生窗口
验收仍须继续核对，不将本批标记为 Channel 全量完成。

新增窗口/重复和未来确认/取消/权限测试；真实 Node SDK → Go HTTP 用例传输
32 个 Unicode 大值（总量超过两倍窗口），验证顺序、最终结果、权限拒绝、
提前退出以及会话和并发槽回收。没有 sleep，也不以模拟 fetch 替代集成结果。
本批 sh scripts/check.sh 全部通过：Go 测试、vet、race、21 个 SDK 测试、
六个 CGO_ENABLED=0 桌面目标构建。跨编译不代表对应平台原生验收。


### WKWebView 流式验收与尾部停滞修复（2026-11-12）

新增 examples/omniweb-streams-check，加载规范 SDK 的编译产物，在真实 macOS
ARM64 WKWebView 中验证 32 个大 Unicode 值（超过两倍消费窗口）、顺序/结果、
权限拒绝、提前 break、AbortSignal、页面 reload 取消旧方法、应用关闭取消活跃
方法。探针有阶段日志与 45 秒截止时间，Go 端确认方法退出；本机运行通过。

首次原生运行暴露了 Node 未发现的停滞：服务端已发送全部值，WKWebView 仍缓冲
部分尾部数据，而服务端等待最后 ACK 才结束响应，导致互相等待。现改为生产结束
即写最终帧并结束响应，不延长已经完成的方法来等待消费确认。生产期间仍有
1 MiB 未确认预算，尾部有界；410 仅表示会话已结束，不替代正常结果验证。
新增 SDK 回归覆盖已结束会话的尾部消费及非法最终帧拒绝；Go 回归覆盖无 ACK
时生产完成立即回收，以及宿主关闭时取消未消费的生产者。

复现：按上节方式编译 SDK，然后执行
CGO_ENABLED=0 go run ./examples/omniweb-streams-check -sdk "$sdk_dir/index.js"。
这只证明本机 ARM64 的 WKWebView 路径，不外推到 Intel Mac、Windows 或 Linux。

修复后的完整检查再次通过：Go 测试/vet/race、23 个 SDK 测试、六个无 CGO
桌面目标构建；原生探针单独运行通过。仍未完成的批量确认、onmessage 接口、
单值大小与 UI 调用约束差异及等价性能验证继续保留。


### Channel 回调消费与关闭状态（2026-11-12）

核对只读参考 frontend/packages/bridge/src/runtime.ts 的 Channel 消费行为，
规范 SDK 的 StreamChannel 新增 onmessage、onclose 和只读 closed。回调按值顺序
消费；赋值 null 停止回调消费，后续可转异步迭代；晚注册回调可取走已等待的值。
onclose 在值流结束或取消时通知一次，closed 在通知前可见；值流结束后 close
是幂等操作，不取消尚未返回的方法结果。整个 client.close 或调用 AbortSignal
仍可取消该方法。同步回调异常经可配置 onListenerError 报告，默认走浏览器
reportError（无此 API 则 console.error），不篡改 Go 方法结果；接口不等待异步
回调返回的 Promise，与参考的同步回调契约一致。

新增单元验证覆盖回调顺序、末尾通知、回调移除后转迭代、晚注册、异常报告、
回调内关闭、onclose 内重复关闭以及值结束后客户端退出；真实 Node→Go HTTP
和 macOS ARM64 WKWebView 探针加入回调消费路径。尚未对齐的差异仍包括批量
确认、任意多迭代器的等待队列、参考的大值及 UI 线程策略、其他平台原生验收
和等价性能证据，不将回调入口新增视为整个 Channel 已完全对齐。

本批最终验证：sh scripts/check.sh 通过（Go 测试/vet/race、30 个 SDK 用例、
六个 CGO_ENABLED=0 目标构建）；严格 TypeScript 编译通过；更新后的 WKWebView
探针实测通过，包含 onclose 内重复 close 后仍取得正常方法结果。


### Channel 多迭代器与并发 next（2026-11-12）

依据参考 Channel 的 waiting 队列语义，将 SDK 的单迭代器限制替换为共享 FIFO
等待队列。同一个迭代器可并发 next，多个迭代器按请求先后取走不同值，值不会
广播或重复交付。onmessage 优先于等待者；移除回调后原有等待者继续消费。
迭代器 return 或显式 close 关闭整个通道，等待中的 next 以 done 结束，方法
result 继续报告取消。确认请求进行中关闭时，不把已预留但尚未交付的值交给
消费者。协议错误仍显式拒绝等待者，保留可观察的异常。

新增覆盖 FIFO 分配、同迭代器并发 next、所有等待者结束、回调优先与切换、
确认期间关闭。Node SDK→真实 Go HTTP 和 macOS ARM64 WKWebView 均新增
多迭代器路径并运行通过。sh scripts/check.sh 完整通过（33 个 SDK 用例、
Go 测试/vet/race、六个 CGO_ENABLED=0 目标构建），严格 TS 编译通过。

剩余差异仍包括按半窗口批量确认、参考的大值与 UI 线程策略、值流结束通知
与尚未消费尾部的时序、外部 AbortSignal 的迭代错误表现、其他平台原生验收及
等价性能证据。保留这些细节，不能仅凭多迭代器入口升级为完整 Channel 对齐。

### 半窗口批量确认与 HTTP 读取进度（2026-11-12）

参考 backend/channel.go 每半窗口请求确认的策略，服务端现在在累计未请求确认
字节达到 512 KiB 时，给 value 帧添加 ask 标志。客户端消费此值后累计确认此前
所有值；不再强制对每个值单独发 HTTP ACK。短流在同一块内结束时无需额外 ACK。
Go 测试验证确认点、整批释放额度、未消费时窗口约束与取消；SDK 验证仅确认标记、
非法标记拒绝和短流无需确认。

原生实测揭示传输差异：只依赖半窗口标志时，Node 通过，但 WKWebView 可能缓存
下一个确认点所在的数据，生产者又在等待额度，形成中途停滞。因此 SDK 在消费
完当前网络数据块、准备继续读取之前，也发送尚未确认的累计进度。相同序号不重复
确认；帧未消费时不确认。没有定时轮询。新增确定性用例将下一数据块的产生依赖于
前一块的确认，验证此路径能继续推进。

修复后的 macOS ARM64 原生探针通过，32 个 Unicode 大值本次用了 10 次累计
HTTP 确认，回调、多迭代器、取消、导航与关闭回归通过。次数取决于网络分块，
不承诺固定为 4 次、不据此宣称吞吐或延迟领先。参考原生消息队列没有同样的 HTTP
缓冲路径，不能把确认请求数量当作完全性能对齐证据。大值/主线程策略、未消费
尾部与结束通知时序、其他平台原生验收和等价性能测量仍待继续。

本批最终 sh scripts/check.sh 全部通过：Go 测试/vet/race、37 个 SDK 测试、
六个无 CGO 桌面目标构建；严格 TypeScript 编译与原生探针另行通过。

### watch 非控件插件接入 OmniWeb（2026-11-13）

重新核对只读参考 backend/plugins/watch 的命名根目录、递归授权、配额和页面
生命周期语义。此前本项目 watch 仅有 Go 轮询扫描器；本批新增
omniui/plugins/watch.Service，向 OmniWeb Streams 注册持续事件方法。

Go 配置示例（应用持有并关闭 service；不导入此插件的应用不增加插件依赖）：

```go
service, err := watch.NewService(watch.ServiceOptions{
    Roots: map[string]watch.Root{
        "project": {Path: absoluteProjectPath, Recursive: true},
    },
    MaxWatches: 128,
})
if err != nil { return err }
defer service.Close()
streams := omniweb.NewStreams(128)
if err := service.Bind(streams, "watch.observe", "files.watch"); err != nil {
    return err
}
// 将 streams 传给 omnigo.Options.Streams，并在服务端授予 files.watch。
```

页面使用规范 SDK：

```ts
type API = {
  "watch.observe": {
    input: {root: string; path?: string; recursive?: boolean};
    value: {ready?: boolean; events: {op: string; path: string; oldPath?: string; isDir: boolean}[]};
    output: {code?: "denied" | "limit" | "not-found" | "unsupported" | "path-encoding" | "io"; message?: string};
  };
};
const streams = new OmniWebStreams<API>();
const changes = streams.call("watch.observe", {root: "project", path: ".", recursive: true});
for await (const batch of changes) {
  if (batch.ready) {
    // 初始快照已经完成，现在开始写入可避免启动竞态。
  } else {
    // 使用 batch.events 更新文件列表；路径相对于请求目录。
  }
}
// 传输成功也需要检查业务结束码；客户端取消仍会拒绝 result。
const outcome = await changes.result;
if (outcome.code) throw new Error(outcome.message ?? outcome.code);
```

注意 Channel.ready 只证明流式传输建立；首个 ready 批次才证明 watch 初始扫描
已完成。服务端持有根目录句柄并以 os.Root.OpenRoot 打开子目录，拒绝绝对路径、
父目录遍历、反斜杠路径和指向根外的符号链接。Roots 配置映射不保留调用方可变
引用；Allow 可进一步拒绝请求，不能授予未配置根。递归必须由根配置允许，扫描
数量使用既有上限。全服务配额默认 128；Channel 的并发配额另行生效。
新增 Event JSON 字段 op/path/oldPath，与页面数据形状对应。

页面请求取消、导航及服务关闭均让扫描退出并释放子目录句柄和配额。Channel
新增 SendContext，让服务关闭可以取消等待发送锁/消费额度的操作，且不取消
整个方法的后续发送；网络写入仍受既有 10 秒写截止时间约束。应用必须在自己的
关闭流程调用 Service.Close，不能把绑定注册误认为自动管理服务生命周期。

确定性测试通过显式扫描触发验证初始快照、真实文件创建事件、请求取消、服务
关闭、配额、授权、遍历/符号链接拒绝和清理；不使用 sleep。通过真实 HTTP
处理器测试完整请求→BindStream→初始批次→事件批次→取消链路，使用内存响应
记录器而非浏览器，尚不代表 watch 插件的原生页面验收。

仍未对齐：原生文件事件后端、文件型根、动态 Resolve、Ignore、按页面配额、
50ms 静默去抖、平台重命名/溢出语义、插件专用 TS 包和原生页面端到端测试。
当前是目录轮询，可能合并或漏掉两次扫描之间的变化；大事件批次仍受 Channel
单值限制，不将这个接入点标记为完整 watch 对齐。

本批 sh scripts/check.sh 全部通过：Go 测试、vet、race、37 个 SDK 用例和
六个 CGO_ENABLED=0 桌面目标构建。watch 原生后端和实机页面验收仍不在此结论内。

### watch Ignore 与目录剪枝（2026-11-13）

对照 backend/plugins/watch/watch.go 与 backend/watch.go 的过滤契约，新增
watch.Options.Ignore 和命名 Root.Ignore。扫描过滤器收到相对于所订阅目录的
斜杠路径；Root.Ignore 始终收到相对于命名根的路径，包括请求子目录的前缀。
同时配置时任一过滤器命中即可排除。忽略目录会在读取其子项前剪枝，忽略项目
不加入快照、不计入 MaxEntries、不产生事件；忽略请求目录自身则快照为空。
过滤器应在监听期间保持规则稳定，不可回调同一个 Watcher；不同订阅可能并发
调用根过滤器，应用需自行保护其共享状态。

新增测试验证目录不被遍历、忽略文件不发事件、可见文件移入忽略区域只发 remove、
忽略文件移入可见区域只发 create，不泄露被忽略端的路径。应用过滤器 panic
转为通用错误，初始扫描失败释放目录句柄，后续扫描失败保留上一成功快照。
子目录订阅使用根相对路径的映射也有确定性验证，没有 sleep。

本批 sh scripts/check.sh 全部通过（Go 测试/vet/race、37 个 SDK 用例、六个
CGO_ENABLED=0 目标构建）。仍需补齐文件型根、动态 Resolve、去抖、按页面配额、
原生事件后端和页面原生验收；再次核对还发现参考 Event 的 isDir 与仅内容写入
语义尚未对齐，当前扫描会报告部分目录/权限元数据变化，保留为后续修正项。

### watch 事件类型与内容变化（2026-11-13）

对照参考 backend/watch.go 的 FileEvent 与 watch_test.go 的原子保存/类型替换
契约，Event 新增 isDir（包括 false），remove 使用删除前类型，create 使用
新增类型，rename 保留原类型。普通目录的时间/尺寸变化与权限变更不再生成
write；非目录仍用内容尺寸/修改时间判断，普通文件换身份时无论尺寸和时间
是否相同均生成 write，以覆盖编辑器原子保存。

同路径从文件变目录、目录变文件或其他类型/目录身份替换，改为 remove 后
create；重命名同时发生内容变化时，先 rename 再 write。对硬链接的身份判定
检查完整前后快照，存在保留硬链接时不擅自推断 rename。测试使用真实临时文件、
显式修改时间和 Poll，没有依赖 sleep；覆盖类型字段、事件顺序、原子保存、
权限/目录元数据、重命名伴随内容变化及保留硬链接歧义。

剩余包括目录整体移动时抑制子项重复 rename、平台稳定身份采集及索引性能、
同尺寸同修改时间写入的原生事件提示、根自身删除、文件型根和其余既有缺口。
当前仍是快照轮询，不声称所有瞬时写入都可检测，也没有性能优于参考的证据。

本批完整检查通过：Go 测试/vet/race、37 个 SDK 测试和六个无 CGO 平台构建。

### watch 目录移动事件归并（2026-11-13）

依据参考目录整体重命名只报告一次的契约，先生成完整目录移动映射，再去掉
路径变化完全由祖先移动解释的子项 rename。归并不依赖 Go map 遍历顺序；按
路径组件查找祖先，不把 a 与 ab 当成父子关系。只去掉重复 rename，不吞掉
文件 write、create、remove，也保留与祖先移动目的地不同的独立重命名。

新增真实临时目录测试覆盖多层目录整体移动、移动后下一次写入的新路径、同批
子目录独立改名/文件改名/内容变化/新增/删除，以及相似前缀边界。另在差异配对
外层检查取消，取消不更新已保存快照。所有行为通过显式 Poll 验证，没有 sleep。
仍待平台稳定文件身份、配对索引性能、原生事件后端及其余已有 watch 缺口；
归并不等于原生文件事件完整对齐。

本批完整检查通过：Go 测试/vet/race、37 个 SDK 用例、六个无 CGO 目标构建。

### watch 动态 Resolve 与授权生命周期（2026-11-13）

新增 ServiceOptions.Resolve(ctx, name)，只在名称未出现在固定 Roots 中时调用；
每次订阅重新解析，不缓存动态根。应用可据当前状态批准已选择的目录。返回目录
必须为绝对路径；动态根仍遵守递归配置、Ignore、Allow 和 os.Root 子路径隔离。
固定 Roots 优先，Allow 只能进一步拒绝解析后的根，不能绕过解析或路径限制。

配额在解析前预留，避免并发解析绕过 MaxWatches；失败、拒绝、取消和正常结束
均归还。Resolver 使用合并页面与服务生命周期的 context，服务关闭会通知协作式
Resolver 退出。回调在锁外执行，Allow 内关闭服务不会死锁或继续启动监听。
动态目录句柄仅属于当前订阅，所有退出路径关闭它；应用自行管理回调共享状态。
Resolve 返回 false 只拒绝新的订阅，不自动撤销已开始的订阅；要停止已有监听，
需取消其页面请求或关闭服务。不声称能强制中断忽略 context 的应用回调。

新增确定性测试覆盖授权后撤销对新请求生效、固定根优先、Allow 拒绝、解析计入
配额、关闭唤醒解析、取消后不启动、递归及符号链接隔离、非法返回路径和关闭
回调重入。文件型根、原生后端、去抖、按页面配额及原生页面验收仍未完成。

本批 sh scripts/check.sh 全部通过：Go 测试/vet/race、37 个 SDK 用例、六个
CGO_ENABLED=0 目标构建；这不替代动态根在各平台原生页面中的端到端验收。

### watch 静默去抖与最长等待（2026-11-13）

对照参考 FileWatchOptions 的 Debounce/MaxDelay，新增默认 50ms 静默期与默认
500ms 最长等待，MaxDelay 不小于 Debounce，负 Debounce 禁用去抖。Next 根据
当前观察快照判断是否重置静默截止，最终从上一次已交付快照计算净变化，不将
中间事件数组简单拼接；因此连续写入合并、短暂创建后删除不产生多余事件。
持续变化达到 MaxDelay 仍交付当前净变化。等待下一次扫描的定时器会考虑更早的
静默/最长等待截止，关闭和 context 取消仍立即唤醒。

显式 Poll 保留立即扫描语义，并提交当前快照、清除待合并状态；Next 才执行
去抖。扫描失败和取消不丢弃已等待批次。页面 Request 新增 debounceMs：0 沿用
服务 Scan 设置/默认值，负数禁用，正数不超过 60000ms。该字段只控制去抖，不
改变服务端根授权、扫描上限或递归权限。

新测试使用显式时间戳验证静默截止前后、持续变化最长等待、空净批次、过滤器
失败与取消恢复，以及负值/显式 Poll；没有 sleep。当前仍是轮询检测，默认扫描
间隔 100ms，静默期从检测到变化开始计算，不能将 50ms 配置宣称为文件系统变化
到页面通知的总延迟。原生事件后端、文件型根、按页面配额和原生验收仍待完成。

本批 sh scripts/check.sh 全部通过：Go 测试/vet/race、37 个 SDK 用例和六个
CGO_ENABLED=0 目标构建。

### watch 真实 WKWebView 页面验收（2026-11-13）

新增 examples/omniweb-watch-check，使用公共应用 API、规范 SDK 编译产物与
watch.Service，在独立临时目录运行真实文件操作；退出自动清理。页面先等待
首个 ready 批次，再逐步请求 Go 创建、写入、重命名和删除文件，每一步等待
相应事件才执行下一步。验证中文路径、op/oldPath/isDir、忽略文件不进入事件
批次、权限拒绝、未知根、路径越界及未授权递归拒绝。

同一探针验证迭代 return 取消、页面 reload 取消旧订阅及应用关闭取消活跃
订阅。服务端 Allow 注册 context 结束信号，页面经独立 RPC 等待取消信号，
避免把前端 Promise 拒绝本身当作后端已停止证据。句柄/配额回收仍由已有 Go
单元测试验证，本探针不直接统计 OS 句柄。总截止时间 45 秒，没有靠 sleep
等待变化或用模拟事件替代真实文件；2026-11-13 macOS ARM64 实机通过。

复现（先编译规范 SDK，输出位置由调用方指定）：

```sh
sdk_dir=$(mktemp -d)
tsc --strict --target ES2022 --module ESNext --lib ES2022,DOM --skipLibCheck \
  --outDir "$sdk_dir" omniweb/client/index.ts
CGO_ENABLED=0 go run ./examples/omniweb-watch-check -sdk "$sdk_dir/index.js"
```

本次使用当前轮询与静默去抖实现，不代表原生文件事件后端、性能或其他平台已
验收；文件型根、按页面配额、平台身份采集、溢出/原生提示等缺口仍然保留。

加入探针后的完整检查通过：Go 测试/vet/race、37 个 SDK 测试和六个无 CGO
目标构建；原生探针及严格 TypeScript 编译另行实测通过。

### OmniWeb 服务端来源与 watch 宿主配额（2026-11-13）

复核发现当前 HTTP 传输没有参考的页面身份。新增 RequestSource 与 Source(ctx)：
每个 NewHTTP 实例持有服务端生成的独立不透明标识，在公共校验之后注入请求
context，客户端 JSON、请求头、Cookie 或查询参数均不能选择它。它只代表服务端
HTTP 宿主，不是认证用户，也不是浏览器文档身份。桌面窗口独立创建 HTTP 宿主
时独立计数；同一浏览器宿主下的多个标签页共享来源。

watch.Service 新增 MaxWatchesPerSource，默认 8，另受全服务 MaxWatches 限制。
额度在 Resolve 前预留，所有退出路径归还，来源计数归零时删除条目；没有 HTTP
来源的直接 Go 调用共享一个计数组。多个宿主共享一个 Streams/Service 时仍能
分别限制，伪造页面标识请求头不能绕过配额。新增完整 HTTP 处理器用例验证来源
隔离、头/Cookie 伪造无效、全局额度、取消后重订阅和来源计数释放。

这是桌面宿主级限制，并非参考每个页面文档生命周期的完整对齐。共享 HTTP 宿主
的标签页分离、导航代际身份及立即退还已取消但仍未退出的处理器额度仍需继续，
不能以 MaxWatchesPerSource 替代这些缺口。完整检查已通过（Go 测试/vet/race、
37 个 SDK 用例及六个 CGO_ENABLED=0 目标构建）。

来源配额接入后的 macOS ARM64 WKWebView watch 探针回归通过，覆盖文件事件、
过滤/拒绝、主动取消、重载及应用关闭；这仍不提供多标签页身份隔离的证据。

### watch 取消与配额退还时机（2026-11-13）

再次核对参考 active 的计数方式：已取消请求不占页面配额，但尚未退出的处理器
仍占全局 MaxWatches。本批对来源配额采用同一规则。取消回调退还来源计数；新
请求准入时也同步检查旧 reservation 的 context，避免依赖取消回调调度时机。
处理器真正退出时才退还全局槽，重复清理不会扣掉后续请求的额度。全局上限仍
限制忽略取消、迟迟不返回的应用回调，不能通过取消后不断重开绕过它。

新增确定性用例让旧 Resolver 在取消后继续等待：同来源替代订阅可启动，第三
请求仍被全局上限拒绝；旧请求最终退出不影响新订阅计数，全部退出后来源与
reservation 记录归零。测试用显式通道控制顺序，没有 sleep。文档级页面身份与
多标签页隔离仍未实现，来源配额仍不等于完整 MaxWatchesPerPage 对齐。

本批完整检查通过：Go 测试/vet/race、37 个 SDK 测试、六个 CGO_ENABLED=0
平台构建。取消配额的新增证据来自确定性并发测试，不外推为页面身份已完成。

### watch 文件型根与正常结束（2026-11-13）

Open、固定 Root 和动态 Resolve 现在可指定普通文件；目录根的请求也可选择
其内部普通文件。应用提供的根路径先解析初始符号链接，持有父目录句柄与文件
名；后续观察使用 Lstat，不跟随文件被替换后的符号链接。文件型根只接受请求
路径 "."，当前拒绝递归请求，不能借它观察兄弟路径。文件事件均使用 path="."
和 isDir=false，Ignore 仍收到对应根相对路径。

原子保存换文件身份时，即使大小与修改时间相同仍报告 write。文件消失或被换成
非普通文件时，立即交付 remove（未被 Ignore 时）、结束监听并关闭其目录句柄；
后续 Poll/Next 返回 io.EOF，不因文件重新出现而复活。服务把 EOF 转为正常方法
返回，页面可先收到 remove，再看到迭代完成和 result=true。首次目标不存在或
不支持的目标类型仍拒绝。显式 Close 在已结束状态下幂等。

新增确定性测试覆盖原子保存、兄弟文件隔离、删除/重新出现、替换为根外符号
链接、固定及动态文件根、目录内文件请求、Ignore 和正常配额回收。真实 macOS
ARM64 WKWebView 探针新增文件写入→删除→迭代结束→成功结果链路，实机通过，
同时回归此前目录事件、过滤、拒绝、主动取消、导航及关闭。

本批完整检查通过：Go 测试/vet/race、37 个 SDK 用例及六个 CGO_ENABLED=0
目标构建；规范 SDK 严格编译通过。仍待根/父目录路径整体移动后的结束语义、
平台稳定文件身份、原生事件后端、特殊文件/符号链接请求与递归选项的完整参考
边界，以及其他平台原生证据；本批不标记整个 watch 完成。

### watch 根路径消失、替换与父目录移动（2026-11-13）

对照参考 watch_scan.go 的根目录 gone/goneEvents 语义，打开监听时记录解析后
绝对目录路径及从实际目录句柄读取的身份，扫描前后重新核对路径。保留句柄在
目录移动后仍可读取，但现在不会将其继续当成原路径的监听。根路径不存在、被
新目录或其他类型替换时，目录监听只交付 remove(".", isDir=true)，单文件监听
交付 remove(".", isDir=false)，随后关闭资源并返回 EOF。服务转为正常完成。
不会把旧树全部子项展开成删除列表，也不会自动接管同路径后来出现的新树。

新增真实文件系统、显式 Poll 测试覆盖目录移动、同路径替换、文件父目录移动、
扫描回调期间发生移动，以及通过服务监听子目录后删除并正常结束。创建监听时
也核对路径与句柄一致性；查询错误不伪装成正常删除。此处仅补根目录身份检查，
不代表所有条目的跨平台稳定身份采集或原生事件后端已完成。

保留限制：轮询仍无法保证检测两次扫描之间发生又恢复的路径变动；原生提示、
溢出恢复、条目身份索引及其他平台实机验收仍待完成。

本批完整检查通过：Go 测试/vet/race、37 个 SDK 用例及六个 CGO_ENABLED=0
目标构建；根路径变动的新证据来自当前 macOS 的真实文件系统测试。

### watch 重命名身份索引（2026-11-13）

此前每个删除项都遍历新增项及完整前后快照。现在 macOS/Linux 使用 Lstat 已
捕获的设备号、inode 和文件类型建立身份计数索引，一次汇总前后及新增集合，
仅唯一身份配对；文件类型参与键，避免不同类型的 inode 重用被误判。未知身份
不建立部分索引，而是整批回退保守比较，防止漏掉可能的别名。Windows 等平台
暂不使用索引，稳定句柄身份采集仍待完成。

新增测试将真实临时文件身份的索引结果与保守比较逐项对照，包含保留硬链接的
歧义、未知身份回退与取消；既有原子保存、目录归并和类型替换测试继续有效。
新增 BenchmarkSnapshotDiffRenames，只测预先捕获的文件元数据集合的 diff，
不包含目录扫描、系统事件或页面交付，不是与 golet 的等价性能比较。

本机 Apple M2 Max / darwin arm64 的三次迭代初步记录：1,000 个逻辑重命名约
0.516 ms/op、654,872 B/op；4,000 个约 2.314 ms/op、2,915,621 B/op。样本少，
仅供复现入口，不据此宣称速度提升或优于参考。运行方式：

```sh
CGO_ENABLED=0 go test ./omniui/plugins/watch -run '^$' \
  -bench BenchmarkSnapshotDiffRenames -benchtime=3x
```

原生事件后端、Windows 稳定身份、完整页面身份、特殊文件及平台实机证据仍待
继续，不把这次索引改动计为 watch 完整对齐。

本批完整检查通过：Go 测试/vet/race、37 个 SDK 用例及六个无 CGO 平台构建。
Linux 索引有交叉构建证据；本次实际执行的文件身份测试和基准为 macOS ARM64。

### watch 路径编码完整性（2026-11-13）

重新核对只读参考 `backend/plugins/watch/watch.go` 的事件发送：参考在发送前检查整批 Path / OldPath 是否为 UTF-8。OmniGo 之前直接交给 encoding/json，会把非法字节替换为 U+FFFD，导致页面收到另一个路径。本次 Service 对整批事件先做编码检查，失败返回 ErrEncoding，不发送有效前缀，也不泄露原路径；本地 Go Watcher 保留原始路径字节。正常中文、emoji 及合法 U+FFFD 不受影响。

证据：`omniui/plugins/watch/encoding_test.go` 覆盖 create/rename 的两个路径字段、整批拒绝、watcher 关闭、全局/来源配额释放及重新订阅；`service_http_test.go` 覆盖实际 HTTP handler 输出 ready/value/end/result(error)，验证无变形路径泄露。当前通用流错误仍映射为 internal，尚未提供参考插件的结构化 encoding outcome。真实非法字节文件名测试在本机 macOS 被文件系统拒绝创建后跳过，不能算 Linux 文件系统实测通过。

复核保留缺口：单个事件批次仍受 Channel 256 KiB 消息上限约束；不通过直接拆批把参考的单批次语义改写后声称完成。原生事件后端、Windows 稳定文件身份、真正的页面身份及其他平台原生验证继续未完成。本轮不修改控件。

验证结果：本轮 `sh scripts/check.sh` 通过（Go tests/vet/race、37 个 SDK 测试、六个桌面目标 CGO_ENABLED=0 构建及生命周期示例）。交叉构建不代表对应平台的原生窗口或文件系统行为已验证。

### watch 结束结果与非递归根权限（2026-11-13）

对照参考 service 的 run/codeFor 和请求授权条件，补齐两项差异：

- Bind 的最终结果由 boolean 改为 `Outcome{code?, message?}`。正常结束返回空对象；错误分为 denied、limit、not-found、unsupported、path-encoding、io。消息使用固定描述，不返回操作系统原始错误或文件路径。配额和快照条目上限、过大的 debounceMs 都归入 limit；目标不存在可归入 not-found。客户端取消和 HTTP 授权失败仍由传输协议处理。调用方需更新结果类型并检查 code，本文件示例及原生验收已同步更新。
- Root.Recursive=false 时只能选择该根本身和直属普通文件；订阅子目录或更深层文件返回 denied，Allow 返回 true 也不能扩大权限。应用明确授予 Recursive 后可订阅子目录，即使该次扫描不递归。静态根和 Resolve 动态根均覆盖。

验证：outcome_test.go 通过实际 HTTP handler 检查正常结束和六类结果，包含真实扫描超限、缺失文件、敏感路径不外泄及资源释放；shallow_test.go 检查动态/静态授权和允许的直属文件。完整 `sh scripts/check.sh` 通过（tests/vet/race、37 SDK 用例、六桌面目标无 CGO 构建）。更新后的 macOS WKWebView 原生验收通过：denied 业务结果、HTTP 403、子目录拒绝、创建/写入/重命名/移除、ignore、文件根正常空结果、取消、重载、应用退出释放。

这取代上一节“watch 错误统一映射为 internal”的状态，不代表插件全部完成。原生事件后端、256 KiB 以上完整批次传输、真正页面身份、Windows 文件身份和 Linux/Windows 原生实测仍缺失。请求符号链接文件及文件根 recursive 参数的参考语义尚待补齐；目前没有用这些限制声称完整覆盖。

### watch 请求路径组件与文件递归选项（2026-11-13）

重新阅读参考 resolve/validateRoot 及 TestPaths，纠正前文“符号链接请求待支持”的表述：应用指定的根路径允许在配置/动态解析时规范化符号链接，但页面请求的每一个路径组件必须是普通目录，终点必须为普通目录或普通文件。即使指向同一个授权根内部，页面链接也应被拒绝。

本轮新实现 `target.go` 逐级持有、打开目录句柄，拒绝目录链接、文件链接及中间路径链接。打开子目录后重新检查目录项类型、身份及打开对象身份，再释放父句柄；失败路径释放已取得的句柄。Allow 回调执行后才取得目标，因此回调将普通目录替换为链接不能使该订阅启动。仍以 os.Root 保证访问范围，不使用未受限的客户端路径拼接。初始应用根规范化行为保留。

同时去掉文件目标额外拒绝 recursive 请求的条件：若应用根已授予 Recursive，文件根或目录中的普通文件可带 recursive=true，仍观察该文件并在删除后正常结束；未授予递归权限的根仍拒绝该参数。`target_test.go` 覆盖内部/外部/中间/文件链接拒绝、正常多级目录与文件、静态/动态应用根链接规范化、Allow 中目录替换、文件递归请求的删除事件及完成。

完整 `sh scripts/check.sh` 通过（Go tests/vet/race、37 SDK 测试、六桌面目标 CGO_ENABLED=0 构建）。该证据不代表 Windows junction/重解析点与并发路径替换的所有原生行为已验证；Windows 稳定文件身份仍是缺口。原生事件后端、大批次传输、真实页面身份等未完成项继续保留，控件未修改。

本轮亦重跑 macOS WKWebView watch 原生验收通过，验证正常子目录事件、文件根完成、请求拒绝、取消、页面重载及应用退出清理没有被新路径解析破坏。新增链接和 recursive=true 文件场景的证据来自确定事件 Go 测试，尚未扩展至原生页面验收脚本。

### watch 新订阅重新验证规范根路径（2026-11-13）

发现 captureRootPath 在应用配置根已经规范化后又调用 EvalSymlinks：若配置后原根被移走、原位置改成指向旧目录的链接，订阅会继续采用旧目录。参考插件在新请求中要求规范路径仍由普通目录构成，不重新授予这个链接路径。本轮取消第二次符号链接解析；新订阅建立时验证路径各级都是普通目录，并保持目录句柄和路径身份一致性检查。根目录、祖先目录或文件父目录被替换成链接时不发送 ready，失败后释放订阅配额。

新订阅检查与已有监听分开：已有监听仍按所监听目录自身的路径/身份判断消失；祖先路径变成链接但所监听目录身份不变时，不额外终止，这与参考 scan 的检查范围一致。目录自身被替换成链接时仍发送单个根 remove 后结束。普通文件原子保存可以改变文件身份，新订阅仍允许启动；不能把目录身份固定规则套用到文件。

证据：`root_revalidation_test.go` 覆盖上述三种新订阅链接替换、既有目录/文件父目录替换后的 remove/EOF、祖先链接改变时保留既有监听、文件原子保存后重新订阅。测试使用真实临时文件系统及显式 Poll，无 sleep。Windows 稳定身份、junction/重解析点与所有并发路径替换情况仍无完整原生验证，原生事件后端、大批次传输及真正页面身份仍未完成。本轮没有修改控件。

最终实现的 `sh scripts/check.sh` 已通过：tests/vet/race、37 个 SDK 用例、六桌面目标无 CGO 构建及生命周期示例。最终版本的 macOS WKWebView 原生回归亦通过：文件事件、文件根完成、拒绝、取消、重载和应用退出；新增路径替换场景仍以 Go 文件系统测试为证据，未据此宣称所有平台原生对齐。
