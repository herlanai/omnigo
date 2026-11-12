# OmniBrowser

参考 [Obscura](https://github.com/h4ckf0r0day/obscura) 的独立纯 Go 无头文档引擎。**当前是功能子集，不是 Obscura 的完整重写，也不兼容完整 Chromium / Puppeteer / Playwright。** 不调用 Rust Obscura、不启动 Chromium、不通过 WebView 执行网页。

实现分层：

- `omniui/plugins/omnibrowser`：Go HTML5 解析、CSS 选择器、Goja JavaScript、DOM、HTTP 会话与并发抓取。不导入 OmniUI 或原生宿主。
- `omniui/plugins/omnibrowser/render`：可选 Go CSS 子集 PNG/PDF 渲染，以及使用 OmniUI 排版的独立语义交互预览。
- `omniui/plugins/omnibrowser/cdp`：可选多目标、会话和隔离 JS 上下文的 CDP 子集，使用已有 coder/websocket。
- `cmd/omnibrowser`：fetch / html / scrape / serve / mcp 命令。
- `omniui/plugins/omnibrowser/css`：选择器级联、继承、inline 与 important 优先级。
- `omniui/plugins/omnibrowser/mcp`：MCP 2025-06-18 stdio 工具协议。
- Studio → **Plugins → OmniBrowser → Go 原生演示**：预览、编辑 HTML、运行 JS、提取 DOM、捕获图像，以及 URL 导航。

## Go API

```go
page := omnibrowser.New(omnibrowser.Options{})
defer page.Close()
ctx := context.Background()
if err := page.Navigate(ctx, "https://example.com"); err != nil {
    return err
}
result, err := page.Eval(ctx, `document.title`) // owned JSON bytes
matches, err := page.Query(ctx, "a[href]")
snapshot, err := page.Snapshot(ctx) // owned HTML/text/title/log snapshot
```

`SetContent` 执行内联经典脚本，不下载资源。`Navigate` 支持 HTTP(S)、最多十次重定向、Cookie、外部 CSS 和同源外部经典脚本；外部脚本按文档顺序执行，不模拟 async/defer。跨源脚本记录为 ScriptErrors 并跳过；StrictScripts 下立即失败。脚本重定向仍限制同源。页面脚本异常记录在 `Snapshot.ScriptErrors`，默认像普通浏览器一样继续加载；CLI 将异常输出到 stderr，Studio 展示错误提示。`StrictScripts: true` 可选择遇到异常立即失败；超时/取消总是失败。已经发生的 DOM 修改不回滚。

DOM 子集：`querySelector(All)`、`getElementById`、`createElement/createTextNode`、`textContent/innerHTML`、常用属性、子节点插入/删除、`addEventListener`、单目标 click/input 与 DOMContentLoaded 事件。`Fill` 拒绝 disabled/readonly；`Click` 拒绝 disabled。没有事件冒泡、表单提交、链接默认导航或完整 DOM/Web API。

Page 串行化调用；每个页面有独立 VM 和 Cookie jar。`Close` 会取消正在执行的脚本/网络操作。默认每次操作 2 秒、文档/脚本/响应 2 MiB、最多 50,000 个分配节点、64 个文档脚本；脚本超时后 VM 可继续使用。节点配额到下次导航才重置。JS 堆内存没有硬上限，Goja 也不是恶意代码的进程隔离沙箱。

网络默认禁用代理环境变量，并在实际连接前解析、检查 IP，阻止 loopback、private、link-local 和共享地址；连接使用已检查 IP。访问本地开发服务需显式 `AllowPrivateNetwork: true`。尚无完整浏览器 CORS/CSP、缓存、TLS 指纹模拟、代理和持久化会话。

## CLI

所有选项置于 URL/文件参数之前：

```sh
CGO_ENABLED=0 go run ./cmd/omnibrowser fetch --eval 'document.title' https://example.com
CGO_ENABLED=0 go run ./cmd/omnibrowser fetch --selector 'a[href]' https://example.com
CGO_ENABLED=0 go run ./cmd/omnibrowser html --screenshot /tmp/page.png ./page.html
CGO_ENABLED=0 go run ./cmd/omnibrowser scrape --concurrency 4 --eval 'document.title' https://example.com https://example.org
CGO_ENABLED=0 go run ./cmd/omnibrowser serve --addr 127.0.0.1:9222
```

并发抓取保持输入顺序，每个 URL 独立会话，最大并发 32，逐项报告错误。CDP 服务仅允许绑定 loopback IP，默认生成随机 Bearer token，启动时写到 stderr。HTTP/WebSocket 请求均需 `Authorization: Bearer ...`。发现路径为 `/json/version`、`/json/list`；浏览器入口为 `/devtools/browser/omni`，保留 `/devtools/page/1`。

CDP 支持目标创建/关闭、上下文登记/销毁、扁平会话、导航生命周期事件、Runtime 求值/函数调用/对象句柄与释放、共享 DOM 的隔离 JS 全局上下文、PNG 截图与单页 PDF。导航使旧句柄与隔离上下文失效。上下文内页面暂不共享 Cookie/storage；每个连接维护自己的目标列表，初始页面由宿主提供。

`Page.printToPDF` 目前仅支持 `ReturnAsBase64`。截图固定 1024×768，PDF 固定 794×1123；不支持完整页面、裁剪、设备仿真或分页打印。Network 域发送实际 HTTP 请求的 requestWillBeSent、responseReceived、loadingFinished/loadingFailed，并支持 getResponseBody；事件在当前操作完成后发送，不是实时流。普通导航响应及正文已通过真实客户端测试，重定向链的完整 CDP 语义、拦截、ExtraInfo、WebSocket 与 SSE 尚未对齐。Log/Inspector/Audits/Performance 的 enable/disable 仅用于客户端初始化，没有完整事件流或指标。未知方法返回错误。新文档脚本在文档脚本之后执行，尚不符合 Chromium 的注入时序；隔离世界之间不共享事件监听器。

### 真实客户端兼容验证

`compat/probe.mjs` 固定验证 Puppeteer Core **25.17.0** 和 Playwright Core **1.65.0**：连接、创建/关闭页面、HTTP 导航、标题、DOM 求值与 JS click/input、对象句柄传参与释放、Promise/timer、CDP PNG/PDF、request/response 事件、导航响应状态及正文读取。它不是完整客户端测试集；尚未支持高层 locator/actionability、键鼠 Input 域、下载、拦截、浏览器上下文共享会话、默认 viewport 仿真或高层 `page.pdf()`。

Puppeteer 使用 `defaultViewport: null`；Playwright 使用 `noDefaults: true`，通过 `browser.contexts()[0].newPage()` 创建页面。两者均携带 Bearer token。客户端依赖仅用于测试，不是 Go 引擎运行依赖，不下载 Chromium。

```sh
CGO_ENABLED=0 go build -o /tmp/omnibrowser ./cmd/omnibrowser
npm --prefix omniui/plugins/omnibrowser/compat ci --ignore-scripts
OMNIBROWSER_BINARY=/tmp/omnibrowser npm --prefix omniui/plugins/omnibrowser/compat test
```

## Web API 与 MCP

新增 `element.style`、`getComputedStyle`、同源 `fetch`、`Headers`、文本/JSON `Response`、`URL`/`URLSearchParams` 基础操作、按页面及来源隔离的内存 storage、Promise、`setTimeout/clearTimeout` 与 microtask。fetch 使用受限 HTTP 客户端、Cookie jar、超时和字节配额；拒绝跨源请求/重定向和 Cookie 等受限请求头。

这些是 Web API 子集：fetch 传输同步发生在当前页面操作内，Promise 不代表并行 I/O；timer 在等待 Promise 时推进；文档加载结束还会推进最多 64 个即将到期的 timer/animation-frame 回调，以支持流式 HTML 正文装配。仍无持续后台事件循环。URL 属性与 searchParams 尚未实现双向更新，非完整 WHATWG URL；base64 API 暂按 UTF-8 字节处理；storage 不持久化、不跨页面共享；没有 XHR、Worker、WebSocket JS API、流、完整 AbortSignal 或模块加载。`NewRealm` 只共享 DOM/网络/storage，不共享 JS 全局，主页面导航和关闭会使其失效。

```sh
CGO_ENABLED=0 go run ./cmd/omnibrowser mcp
CGO_ENABLED=0 go run ./cmd/omnibrowser html --pdf /tmp/page.pdf ./page.html
```

MCP 使用逐行 JSON-RPC stdio，依次 initialize → notifications/initialized → tools/list 或 tools/call。工具包括 navigate、set_content、evaluate、query、click、fill、screenshot、pdf。PNG 返回 image 内容，PDF 返回嵌入资源；支持取消、重复请求 ID 检查、8 个活动工具上限与 2 MiB 输入上限。仅实现 stdio tools，未实现 HTTP/SSE、资源订阅、提示词或完整 MCP conformance。

## Studio 与绘制

演示初始页面为内置 HTML，加载时真实执行 JavaScript，按钮操作改变 DOM。URL 导航与 JS 执行由 native host 后台任务完成，结果通过 `runtime/dispatch` 返回 UI；关闭演示窗口关闭 Page。离屏测试使用同步执行器，导航入口禁用以避免测试依赖公网。

Studio 导航后显示 **Page** CSS 图像预览，原有可交互 Go 控件保留在 **DOM controls** 页。原生语义预览支持标题、正文、列表、按钮、输入、链接文本和分隔线；有 ID 的按钮/输入可交互。最多生成 500 个节点，图片显示 alt 文本。它与 CSS 截图是两个明确不同的渲染入口。

`render.Screenshot` 支持内联/外部样式表级联、继承、CSS 变量、px/rem 宽高媒体查询（含范围比较）、CSS layer 常规优先级、px/rem/em/vw/vh 与 calc/min/max/clamp、四边 margin/padding、max-width、块布局、flex 分配/换行、固定/fr grid 列、绝对/固定定位、aspect-ratio 与 translate/rotate/scale、圆角/椭圆边框、线性/径向渐变、RGB/HEX 颜色、表单占位文本，以及路径/基础图形 SVG。网络样式表最多 64 个、单表 512 KiB、累计 1 MiB；失败写入 Logs，资源请求遵守原网络限制。没有实现 @import。

字体使用纯 Go 解析与栅格化，渲染模块内嵌 Inter Regular/Bold 与 Noto Sans SC Regular/Bold（SIL OFL 1.1），常用简繁中文不再依赖宿主系统字体。中文/拉丁字符共用回退字体链及行高；font-weight 支持常规/粗体。`OMNIBROWSER_FONT_PATH` 可指定优先 TTF/OTF/TTC，TTC 最多读取 8 个 face。默认字体增加约 16 MiB 可选渲染模块资源；只导入核心 Page 的程序不包含字体。来源、文件哈希与许可证见 `render/fonts/`。尚未实现完整 font-family 匹配、WOFF/WOFF2、斜体、多语言 shaping 和全部 Unicode 字形。

截图最多 8 百万像素、文档 2 MiB、布局 10,000 节点/128 层。已补基础行内文本/inline-block 排版、父容器百分比高度、content-box/border-box、分角圆角、矩形 overflow 裁剪以及 PNG/JPEG/GIF 首帧/WebP 图片资源；图片最多 32 次请求、累计 8 MiB，解码限制 8 百万像素。仍缺完整 flex/grid/inline formatting、表格、堆叠与圆角内容裁剪、动画图片、WOFF/WOFF2 Web 字体、完整动画/事件循环、Canvas 与复杂 SVG；CSS 完整一致性尚未达成。

### so.qq.com 实测（2026-11-12）

该站当前返回流式 Next 页面。已修复外部 CSS 未加载、中文缺字、流式 DOM 节点未移入正文、fixed/absolute 被计入 flex 列宽等问题；纯 Go 截图可显示中文提示、元宝搜索 SVG 标识和搜索框边框。命令：

```sh
go run ./cmd/omnibrowser fetch --timeout 20s --screenshot /tmp/so-qq.png https://so.qq.com
```

这不是完整站点验收：CDN 脚本被跳过、WOFF 图标缺失、布局仍有差异，React hydration 和实际搜索流程未通过验证。没有把截图可见当作整站可用。

### baidu.com 展示回归

2026-11-12 实际页面检查修复了：`type=hidden` 被绘制、导航 inline-block 纵排、绝对定位 auto 宽度撑满、百分比高度按视口计算、搜索输入高度未计入 padding/border、Logo 图片未加载。当前截图可显示横向导航、右侧登录、Logo 和完整搜索框。测试覆盖隐藏字段不产生像素、inline-block 同行、父容器百分比高度、盒模型、overflow、图片资源快照所有权、中文常规与粗体实际字形。

这不是完整 Chromium 像素一致性验收；未验证百度的动态交互、登录与搜索结果页。纯 Go 局部 CSS 渲染器仍不能宣称完整网站兼容。

### omniorbit.si 展示回归

2026-11-12 实际页面修复：Tailwind CSS `@layer`、宽度范围媒体查询、逻辑 margin/padding、clamp 字号、inline-flex/匿名文本项、space-between/center 对齐、fr Grid 列、比例容器高度、SVG 浅色主题 invert。主题初始化补充 classList、document.currentScript、元素相邻节点和静态 matchMedia（1024×768、light，不提供视口变化事件）。当前可显示中文、主标题、操作按钮与 SVG 星球轨道。另有通用渐变和二维 rotate/scale 的像素回归测试。

仍不是像素级复刻：复杂选择器与生成内容、嵌套层完整级联规则、CSS filter 链、复杂 SVG 文字排版、完整动画/脚本模块、字体字距和完整 Flex/Grid 算法仍有限制。没有验证该站 React hydration、菜单及应用导航。`@supports` 仅识别 display:grid/flex；其他条件会记录诊断。

### SVG 文字、伪元素与动画采样

新增简单 SVG text 字形轮廓（含中文、anchor、字距）、SVG 属性级联/变量、before/after 独立渲染盒及字符串/attr() 内容。`fit-content` 与隐藏 Flex 子项修复了 omniorbit.si 顶部控件测量。

```sh
go run ./cmd/omnibrowser fetch --timeout 25s --animation-time 10s --screenshot /tmp/orbit-10s.png https://omniorbit.si
```

Go API：`render.ScreenshotAt(snapshot, width, height, dark, elapsed)`。当前采样首个 CSS transform 动画，支持 linear/常用 ease、延迟、循环、反向、填充和初始 paused 状态；SVG 组也使用采样变换。默认 Screenshot 取 0 时刻。此 API 不驱动 JS timer/rAF，Studio 图像预览仍是静态图。

具体范围、未实现项目与 Obscura 源码依据见 [渲染对照审阅](OBSCURA_RENDER_REVIEW.md)。

### Go 高层 Locator

```go
err := page.Locator("#query").Fill(ctx, "中文")
err = page.Locator("button").FilterText("搜索").Click(ctx)
text, err := page.Locator(".result").Nth(0).TextContent(ctx)
```

支持 Count、Nth、FilterText、GetAttribute、TextContent、WaitFor(attached/detached/visible/hidden)、Click、Fill。每次重新查询 DOM，动作严格要求单一匹配，并等待可见/启用/可编辑状态；支持取消与超时。尚无遮挡命中、滚动、真实键鼠或自动导航。

**Go Locator 不等于 Puppeteer/Playwright locator 兼容。** 实测 Puppeteer 25.17.0 的注入代码需要 Goja 尚未实现的 async generator；Playwright 1.65.0 的注入代码首先失败于缺失 MutationObserver。没有绕过这些检查返回假成功，客户端 locator 仍列为未完成。

`render.PDF` 将一个 CSS 视口嵌为无损 RGB 图像，生成 PDF 1.4 单页文件；文字不可选择，无分页和打印样式。测试检查 PDF 交叉引用偏移、页尺寸及非法尺寸；独立 `pdfinfo` 已验证导出文件可解析。

## 来源、许可证与验证

Obscura README/Cargo 元数据使用 Apache-2.0，作为功能与分层参考；没有复制实现体或资产。新增实现使用 Goja（MIT）、Cascadia（BSD-2-Clause）、x/net（BSD-3-Clause）；传递依赖许可证副本位于 `licenses/`。共享 OmniUI/字体与 WebSocket 许可证沿用仓库原有记录。

`capabilities.json` 保留完整参考领域与缺口。没有移用 Obscura 的内存/速度数据来证明本实现的性能。

```sh
CGO_ENABLED=0 go test ./omniui/plugins/omnibrowser/...
go test -race ./omniui/plugins/omnibrowser/...
sh scripts/check.sh
```

测试覆盖真实本地 HTTP Cookie/重定向/外部脚本、DOM/JS 交互、超时恢复、资源配额、并发抓取、CDP 认证与失败、Go 图像输出，以及 Studio 工作线程与 UI 结果应用分离。

实际公网检查：`fetch --eval 'document.title' https://example.com` 返回 `"Example Domain"`，同时报告了该次响应中一段脚本访问未支持属性的异常。此结果只证明 HTTP/DOM/求值链路可用，不代表该页面全部脚本或渲染已兼容。
