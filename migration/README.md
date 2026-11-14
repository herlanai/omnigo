# 完整功能覆盖清单

当前非控件对齐任务见 [非控件能力审计](noncontrols/README.md)，保留独立的当前参考文件/API 快照、优先级、实现和验证证据；控件暂缓。

参考仓库：本地 `golet`，只读扫描。初始记录包含 **2,412 个源码/文档文件指纹、4,517 条 Go 导出声明和 16 个插件目录**。声明数量包含条件编译变体，并不等于独立功能数量。

`reference.json` 记录接口声明与源文件 SHA-256，不包含函数实现体，不导入旧框架。Go 导出清单排除 internal、cmd、examples 与测试文件；这些目录及前端的源码指纹仍记录在 files 中。**TypeScript 导出、CLI 行为、视觉状态与平台行为尚需进一步建立行为级清单。当前清单不能独自证明完整覆盖。**

初始参考快照中的声明保持 `pending`，不把它当作实时完成率。当前实现与差距分别记录在 [控件审计](controls/README.md)、[原生平台审计](platform/README.md) 和 [childwindow/clipboard/diff/e2e/GPU/多语言专项](features/README.md)。这些清单互补，不得只统计导出 API 而漏掉内部算法、资源生命周期和原生测试。Web UI 的 OmniWeb 协议与默认栈进度另见 [Web UI 行为覆盖](webui.md)。

## 必须覆盖的功能域

`backend/internal` 的 42 个目录另有[全目录范围与实现记录](internal/README.md)，保留原生源码、内部算法和测试索引。新增文本存储、快捷键、下载的真实实现及 Studio 演示在该记录中单独验收，目录覆盖不等于行为完成。

| 功能域 | 重写位置规划 | 当前状态 |
| --- | --- | --- |
| 生命周期、主线程调度、事件 | omniwindow / internal/runtime | 生命周期核心、就绪等待、单实例已有实现；参考行为尚未逐项验收 |
| 窗口、多窗口、屏幕、菜单、托盘、快捷键 | omniwindow / 原生驱动 | 已有实验 Cocoa 与 Win32/X11 软件窗口；父子窗、屏幕、菜单、托盘等仍缺，见平台与专项审计 |
| 系统 WebView、Web IPC、信任边界、资源协议 | omniwindow/host / omniweb | macOS WKWebView、RPC、事件及流已有实现与部分原生验收；其他平台及页面服务仍缺 |
| 原生 UI、布局、主题、控件、动画、焦点、IME、无障碍 | omniui/baseui | 已有基础 Go 行列布局、控件行为、焦点、文本缓冲、无障碍快照；系统 IME/无障碍待接入 |
| Metal、D3D、OpenGL、CPU、浏览器渲染 | omniui/baseui 绘制层 | Go CPU 绘制与可选 Metal 矩形 compute 渲染已实现；完整 GPU 控件绘制和其他后端仍待实现 |
| 图表、代码/富文本/Markdown 编辑器 | omniui/baseui / 可选插件 | 部分实现，DiffView 与独立行级 diff 已有；不代表高级编辑或完整算法覆盖 |
| Forma 与 AI 组件、加载与特效组件 | omniui/baseui | 多项局部实现，逐项差距见 controls；未宣称完整对齐 |
| 音频、视频、拖放、剪贴板、文档、国际化 | omniui / omniwindow | 剪贴板数据接口、Mac/Windows 部分原生通路及 i18n 已有实现；完整媒体、拖放、文档和平台覆盖仍缺 |
| TS 生成、CLI、热重载、项目模板、打包、更新 | cmd / tooling | init、平台查询 doctor、更新 keygen 已有；Studio 专用开发/打包脚本已有局部实现，通用生成、开发与发布流程未齐，见下方工具链复核 |
| 原有示例、文档、浏览器与桌面演示 | examples / examples/studio | 已有离屏画廊、Studio 与 native-window 示例；非全部参考示例覆盖 |
| 全部 16 个插件及前端客户端 | omniui/plugins；基础控件不导入插件集合 | 非控件插件已有局部实现，watch 等已接 OmniWeb；各项差异保留在插件审计，控件插件本轮暂缓 |

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

## 工具链复核与更新密钥生成（2026-11-13）

对照只读 `golet/backend/cmd/golet/main.go` 的命令表及更新密钥命令接口，当前命令覆盖如下。Studio 专用 devtool 不作为任意应用项目 CLI 已对齐的证据。

| 参考能力 | 当前实现与缺口 |
| --- | --- |
| init | `omni init` 生成 OmniWeb 浏览器宿主模板；原生桌面及完整模板选项仍缺 |
| doctor | 已加入基础开发工具检查及平台诊断，保留平台 JSON 接口；原生运行时和签名身份检查未齐 |
| keygen | 本轮新增 Ed25519 密钥对生成、输出目录与显式覆盖，见下文 |
| generate | 已有契约生成入口；具体输出与限制见 `cmd/omni/generate.go`，不代表全部消息目录访问器覆盖 |
| dev | 已有通用 macOS/Linux 监听、重建和重启；前端 HMR、Windows dev 未实现 |
| build / package | 已有通用配置构建及 macOS app/DMG 打包；其他平台安装包和生产发布签名未齐 |
| i18n check | 已有配置/显式目录入口和语法、消息键、参数名/类型诊断；复数类别与框架覆盖检查仍缺 |
| install-skills | 应用开发配套技能安装命令未实现 |
| version / 全局 help | 已有版本别名、全局/子命令帮助、构建版本与 revision 信息；帮助只列已实现命令 |
| vet / migrate-ui | 原生控件生命周期检查和 UI API 迁移命令仍缺，按当前用户要求暂缓控件工作 |

使用 `go run ./cmd/omni keygen -o /path/to/update-keys`，或安装后的 `omni keygen`。默认目录为 `os.UserConfigDir()/omnigo/update-keys`。生成 `omnigo-update.key`（64 字节 Ed25519 私钥的标准 Base64）和 `omnigo-update.pub`（32 字节公钥的标准 Base64），每个文件末尾有换行。Unix 新私钥文件权限 0600，公钥 0644（受 umask 限制）。输出不包含私钥内容。公钥解码后可直接交给现有 updater.Verify。

已有任一目标文件时默认拒绝；`-force` 显式轮换密钥对，拒绝符号链接、目录等非普通文件目标。`.omni-keygen.lock` 目录串行化同目录的 keygen 命令，旧文件在替换完成前保存在该私有目录；普通写入失败恢复旧文件。若其他写入者占用了恢复目标，不覆盖该文件，保留备份并报告恢复位置。进程崩溃或断电可能留下锁、备份及不完整的新密钥对；这是可恢复的两文件操作，不是断电原子事务。遇到遗留目录需核对并恢复原密钥后再移除锁，不自动删除可能唯一存活的私钥。Windows 的 Unix mode 不证明 owner-only ACL，此项与 Windows 实机权限验收仍需补齐。

验证：`cmd/omni/keygen_test.go` 覆盖生成密钥匹配、现有 updater 验签及篡改拒绝、无覆盖/显式轮换、Unix 私钥权限、写入失败回滚、并发互斥、恢复冲突保留备份、链接与非普通目标拒绝。真实 CLI 临时目录运行通过，随后删除本次探针密钥。`sh scripts/check.sh` 完整通过：Go tests/vet/race、37 SDK 用例、六桌面目标 CGO_ENABLED=0 构建及生命周期示例。

更新链仍未完整：当前 updater 签名对象为清单字节，参考的归档摘要签名、构建配置、发布清单生成、托管平台上传、安装替换与重启流程尚未全部对齐。本轮密钥命令不作为这些能力已完成的证据，控件未修改。

## doctor 开发环境检查（2026-11-13）

`omni doctor` 默认执行本机开发环境检查：Go >=1.26、Node >=24、npm 可执行且报告有效版本。版本要求来自当前 go.mod 与默认 OmniWeb 模板，ready 表示该模板的基础工具检查通过，不表示任意原生应用或完整发布链已可用。Go 查询设置 GOTOOLCHAIN=local，避免诊断触发工具链下载；每个工具运行上限 5 秒、输出上限 16 KiB，命令超时/失败或版本不满足会记录 fail 并让命令非零退出。不会安装软件。

接口兼容：`omni doctor --json`（及 `--all --json`）仍仅返回原平台能力数组，不运行环境工具；`--all` 单独使用也保留平台查询。`--check --json` 返回 `{ready, checks, platforms}`，失败时仍完整输出 JSON 再以非零状态退出；与 `--all` 组合仅扩展平台矩阵，环境检查始终针对当前机器，不能据此证明其他目标运行成功。

可选项包括 bun、pnpm、makensis、gh 及平台签名工具。PATH 中找到可执行文件不代表安装包、签名或上传已经验证，报告中明确保留这个区别。macOS 读取系统版本并查询 codesign 路径，不枚举/验证签名身份；Linux 检查 DISPLAY 是否设置及 loader cache 是否列出 libX11.so.6，匹配当前可选 X11 驱动，不把尚未实现的 WebKitGTK 后端算作可用。Windows 检查 npm.cmd（固定 cmd.exe 参数启动）及可选 signtool；WebView2 宿主与运行时检测仍缺失。平台实验性与未验证状态始终列为提醒。可选依赖缺失不把基础工具结果改成失败。

验证：environment_test.go 覆盖最低版本、缺失工具、无效/预发布版本、不支持的架构、Linux 依赖提示、取消与输出上限、默认人类可读报告、失败仍输出结构化 JSON，以及旧六平台 JSON 查询完全不执行工具。实际运行 `go run ./cmd/omni doctor --check --json` 通过，报告本机 Go 1.26.0、Node 24.18.0、npm 11.16.0。此项未在 Linux/Windows 原生执行，尚不覆盖开发证书、原生系统库加载/显示连接、项目配置诊断以及完整参考 doctor 的 WebView 运行时检测。控件未修改。

本轮 `sh scripts/check.sh` 完整通过：Go tests/vet/race、37 SDK 测试、六桌面目标 CGO_ENABLED=0 构建及生命周期示例。交叉构建不等于 Windows 命令启动和 Linux loader cache 在原生环境已验收。

## i18n check 入口与目录诊断（2026-11-13）

新增 `omni i18n check [project-dir]`，读取 `omnigo.json` 的 `i18n.dir/source`（默认 locales/en），支持 `--dir` / `--source` 覆盖与 `--json` 报告。项目目录通过 os.Root 约束，拒绝目录遍历与指向项目外的目录链接；配置大小上限 1 MiB，语言文件逐个限读 8 MiB。JSON 配置的其他顶层字段暂不解析，不等于通用应用构建配置已实现。

新 `i18n.Check` 与运行时共用消息解析器，收集多文件语法错误、重复键和归一化语言重复、源目录缺失、未知翻译键及嵌套参数名差异；未知参数为错误，未使用源参数与缺失翻译为警告，符合参考的失败/警告区分。无错误不改变运行时源语言回退策略。报告输出顺序稳定，JSON 显式声明 coverage；目前类型兼容、ARB 参数类型元数据、语言复数类别、库/框架命名空间覆盖、完整行号与强类型访问器生成仍未完成，不能将新入口视作 i18n 全面对齐。

证据：i18n/check_test.go 验证多项诊断、嵌套参数、确定排序、警告与回退、重复/错误文件及限读；cmd/omni/i18n_test.go 验证配置、显式参数、JSON 失败报告、警告不失败与项目目录隔离。真实 CLI 探针通过：英文源加德文缺失一项，仅输出 warning 并正常退出；探针文件随后清理。控件未修改。

本轮完整 `sh scripts/check.sh` 已通过：Go tests/vet/race、37 SDK 用例、六桌面目标 CGO_ENABLED=0 构建及生命周期示例。没有以命令入口、绿色检查或构建结果替代上述未完成的 i18n 行为。

## i18n 参数类型契约检查（2026-11-13）

本轮补齐上一节保留的源语言 ARB placeholder 类型元数据与跨语言类型兼容检查。支持 String、int、double、num、DateTime 和 Object；未声明的数值/复数/序数参数推断为 int，其他参数推断为 String。普通插值接受任意合法声明类型，select 要求 String 或 Object，number/plural/selectordinal 要求数值类型或 Object。递归检查所有嵌套分支中的用法，同名参数的不同用法都要满足契约。

源语言声明是翻译的类型依据，翻译元数据不能扩大它。未在源消息中使用的已声明 placeholder 仍属于契约并产生警告，翻译可以使用该参数；源元数据描述不存在的消息时警告。未知类型、重复 placeholder、非对象声明、非法 type/description 字段以及不兼容用法汇总为错误。JSON coverage 新增 argument-types，CLI 摘要同步更新。运行时的 Catalog、Format 和消息回退行为未修改；DateTime 普通插值可校验，不代表日期/时间 skeleton 已实现。

证据：check_types_test.go 覆盖隐式类型、显式 double/Object/DateTime、数值与 select 不兼容、嵌套分支、翻译不能覆写类型、未使用声明、重复/非法元数据及孤立元数据；CLI 测试检查 JSON 与退出状态。真实 CLI 探针以源 String / 翻译 number 验证结构化错误及非零退出，临时项目随后删除。完整复数类别、库命名空间、错误行号和访问器生成仍未完成，控件未修改。

本轮最终 `sh scripts/check.sh` 完整通过：Go tests/vet/race、37 SDK 测试、六桌面目标 CGO_ENABLED=0 构建及生命周期示例。类型检查通过不代表前述日期格式、复数类别和生成链路已经实现。

## i18n 诊断行号（2026-11-13）

本轮为 Diagnostic 增加可选的一基 `line`，CLI 文本输出 `locales/de.json:3`。消息语法、参数名和类型兼容问题指向消息键所在行；ARB 元数据结构、类型声明和未使用声明问题指向 `@message` 所在行；重复键指向重复出现的位置，JSON 语法错误指向原始文档的错误位置。没有源行的缺失翻译、缺失文件或读取失败不填写 line，不人为指向文件开头。

行索引按原始字节构建，再查找对应行，处理 Unicode、CRLF 和转义键名；不对每个消息反复扫描整个文件前缀。encoding/json 混用 Token 与 Decode 的错误偏移经测试不能直接当作原始文件偏移，语法错误时使用原始 JSON 校验定位。运行时 Load 的错误文本和 Unwrap 保留原错误，不改变消息编译或回退语义。

证据：locations_test.go 覆盖重复键、非字符串值、非法 JSON、意外 EOF、消息错误、元数据、尾随 JSON、Unicode/CRLF/转义键及缺失翻译不伪造行号；CLI 测试检查文本 `file:line` 与 JSON line。复数类别、库/框架消息覆盖、日期格式和访问器生成仍未完成，控件未修改。

本轮最终 `sh scripts/check.sh` 完整通过：Go tests/vet/race、37 SDK 测试、六桌面目标 CGO_ENABLED=0 构建与生命周期示例。行号定位的完成不改变上述其他非控件缺口的未完成状态。

## i18n 文件内多错误汇总（2026-11-13）

对照参考目录加载器确认：可读取 JSON 对象中的语义错误会逐项报告，不能只报第一个错误；同时参考的字符串解码也接受 null 并产生空消息，不能借对齐任务悄悄改成拒绝 null。本轮为目录检查接入可继续读取的诊断回调，汇总同一文件内的非字符串值、重复键与 ARB locale 声明错误，保留各项实际行号。结构性 JSON 错误仍结束该文件读取，随后可检查其他文件。运行时 Load 继续使用遇错即停止路径。

重复消息和元数据字段均保留首次声明；后续重复值仍报错，但不能覆盖首次消息位置或把 String 改成 Object 来改变后续类型诊断。元数据索引与消息解码遵循同一规则。

证据：check_recovery_test.go 在同一文件中验证五项错误同时出现、运行时拒绝首个错误、重复元数据不改变原契约、非法 locale 不遮蔽后续错误，以及 null/空字符串的检查与运行时一致性。真实 CLI 探针输出两项非字符串错误、正确行号和非零退出，临时项目已清理。复数类别、日期格式、库命名空间和访问器生成仍未完成，控件未修改。

最终 `sh scripts/check.sh` 完整通过：Go tests/vet/race、37 SDK 测试、六桌面目标 CGO_ENABLED=0 构建及生命周期示例。本轮仅补齐文件内错误汇总，不宣称国际化或整体非控件能力已经全面对齐。

## CLI 帮助、版本与退出状态（2026-11-13）

新增 `omni help` / `-h` / `--help`、`omni help <command>` 与 `omni version` / `-v` / `--version`。帮助只列当前可执行的 init、doctor、keygen、i18n check 和查询命令，明确通用 dev/build、客户端生成、发布尚未实现。`omni help i18n check` 及 `omni i18n --help` 均可查询目录检查用法。

版本优先使用构建时 `-ldflags '-X main.version=vX.Y.Z'` 的发布标识，否则读取 Go module 构建版本；源码构建无发布版本时显示 devel，并在存在时附带 vcs.revision 和 modified。不会借用 golet 的版本号或凭空声明发布版本。

帮助写 stdout，返回 0；缺少/未知命令及版本参数误用返回 2，其他运行/解析错误返回 1 并写 stderr。FlagSet 的输出先缓冲，避免重复打印解析错误或把失败诊断混入 stdout 的机器报告。帮助和版本输出流写入失败会返回失败状态。

验证：usage_test.go 覆盖全局/各子命令帮助、版本别名、模块版本/显式版本/开发工作树标识、退出状态及输出失败；真实 CGO_ENABLED=0 构建后的二进制验证帮助、显式版本标识、stdout/stderr 和未知命令退出码，临时二进制已清理。该命令层进展不替代通用开发、生成、打包发布与其他非控件缺口，控件未修改。

本轮最终 `sh scripts/check.sh` 完整通过：Go tests/vet/race、37 SDK 测试、六桌面目标 CGO_ENABLED=0 构建与生命周期示例。Linux/Windows 上实际启动二进制的验收仍不能由交叉构建替代。

## CLI 输出失败与已完成操作（2026-11-13）

复核新增命令输出路径后，修复 doctor 文本平台列表和 init 成功提示忽略写入错误的问题。doctor 的平台列表、环境文本和 JSON 输出失败均返回错误，CLI 返回非零状态。init 若项目已创建但成功提示写出失败，会明确报告“project created at …, but status output failed”，保留完整项目，不因提示失败删除用户结果；重试仍由原有目的目录保护阻止覆盖。

证据：environment_test.go 覆盖 doctor 各输出格式失败；usage_test.go 检查实际 CLI 状态以及 init 写出失败后的项目保留、重试拒绝和内容不变。这是已有实现的可靠性修复，不计作通用开发、生成、打包发布等未完成能力。控件未修改。

本轮最终 `sh scripts/check.sh` 通过：Go tests/vet/race、37 SDK 用例、六桌面目标无 CGO 构建与生命周期示例。整体非控件对齐仍未完成。

## init 空目录与独占写入（2026-11-13）

对照参考 newProject 的“空目录或不存在”条件，修复 OmniGo 原先拒绝所有已存在目录的差异。init 现在允许预先创建的空目录，也能创建缺失的多级目录；非空目录（包括只有隐藏文件的目录）仍拒绝。模板、SDK 和 go.mod 均通过保留的 os.Root 句柄写入，O_EXCL 独占创建避免覆盖其他写入者建立的文件；临时目录锁使同一目标的协作初始化互斥，完成后移除。

写入失败仍保留部分项目，SDK/go.mod 阶段的错误也注明保留位置；不因失败删除用户目录。若初始化被进程终止而遗留文件或锁，后续请求会因非空而拒绝，需先检查已有结果，不自动清理。此修改不代表默认模块名、完整模块路径验证、模板选项、依赖安装或原生应用模板已全部对齐。

证据：scaffold_test.go 覆盖预建空目录、多级目标、隐藏文件保护、两次并发仅一次成功及生成 SDK/模块完整性；真实 CLI 探针验证空目录成功、非空重试拒绝且文件保留，临时项目已清理。控件未修改。

本轮最终 `sh scripts/check.sh` 完整通过：Go tests/vet/race、37 SDK 测试、六桌面目标 CGO_ENABLED=0 构建及生命周期示例。整体非控件对齐仍未完成。

## init 模块路径与框架声明解析（2026-11-13）

移除旧模块路径正则，使用 golang.org/x/mod 的 CheckImportPath 与 SplitPathVersion，并拒绝 go/toolchain 保留模块名，匹配当前 Go 初始化的校验步骤。合法单段模块名（如 my-app）现在可用；Windows 保留文件名、路径尾随点、注入字符、非法主版本后缀等在创建目标目录前被拒绝。没有把“路径元素内连续点号”额外禁止：本机 Go 1.26 的 go mod init 实测接受它，测试依据实际规则修正。

框架 go.mod 改用正式 modfile.Parse，接受合法引号、CRLF 和行尾注释，只认真实 module 指令；注释中的模块名、缺失或重复 module 指令不能伪装框架身份。这只证明模块声明匹配，不等于整个框架 checkout 的内容完整性验证。使用 x/mod v0.41.0，其许可证保留于 omniui/plugins/licenses/golang.org_x_mod@v0.41.0/LICENSE；未复制参考实现。

证据：module_test.go 覆盖合法/非法路径、拒绝前不创建目录、各类 go.mod 语法与身份判断；真实 CLI 使用 --module my-app 创建项目，随后 go list -m 确认模块名，临时项目已清理。目录派生默认模块名、名称罗马化、模板选项等仍未完成，本轮不把它们计入完成，控件未修改。

验证结果：最终 `sh scripts/check.sh` 完整通过（Go tests/vet/race、37 SDK 测试、六桌面目标 CGO_ENABLED=0 构建、生命周期示例）。真实 CLI 探针明确使用已缓存的 Go 1.26.0 工具链，验证单段模块名以及连续点号路径规则；基础 PATH 上的较旧 Go 配合 GOTOOLCHAIN=local 无法构建本仓库，不将该环境失败误判为模板失败。


## 2026-11-14：Golet 后续对齐状态

本轮以本地只读 Golet `031db49` 为参考；没有运行参考性能基准。
历史章节描述当时状态，以本节和当前实现为准。

| 优先级 | 项目 | 本轮结果与后续边界 |
| --- | --- | --- |
| P0 | 更新工具格式兼容 | `omni-update` 可直接读取 keygen 的 Base64 公钥，签名支持 raw/Base64；签名、篡改拒绝及外部模块验证覆盖。生产发布配置、签名和更新 UX 仍归应用负责。 |
| P1 | 原生输入法 | 修复焦点转移、移除、禁用、只读、替换 buffer、关闭时残留组合文本；未实现 macOS NSTextInputClient，也未宣称真实输入法验收。 |
| P1 | 编辑器 | 按不可变文档快照和语言缓存词法结果、行数、行宽；普通文本绘制仅读取视口附近行。修改时仍需全文词法分析，长行、语言服务和复杂文字整形仍缺。TextBuffer 已有 rope，不重复新增文本容器。 |
| P1 | 无障碍 | 仍有 Surface 语义快照，系统读屏适配未完成；状态保持 partial。 |
| P2 | 树形列表 | 只为可见行生成标签，typeahead 按需获取全量标签；展开索引和导航选项仍为 O(N)，未宣称增量树索引。 |
| P2 | 绘制性能 | 加入屏幕外文本测量回归和编辑器热构建基准；脏区域、GPU、端到端呈现对照仍待实现。macOS 已走像素提交，PNG 不是当前主路径。 |
| P2 | 桌面工具链 | doctor 增加可选 GTK/WebKitGTK/GLib/GObject loader 清单检查；存在库名不等于加载或运行成功。原生模板和 Windows Runtime 验收仍缺。 |

相关证据：`alignment_cache_test.go`、`encoding_test.go`、`keygen_test.go`、
`environment_test.go`。全量能力分母保留，不提高完整对齐数量。
