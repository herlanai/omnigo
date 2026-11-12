# OmniBrowser

参考 [Obscura](https://github.com/h4ckf0r0day/obscura) 的独立纯 Go 无头文档引擎。**当前是功能子集，不是 Obscura 的完整重写，也不兼容完整 Chromium / Puppeteer / Playwright。** 不调用 Rust Obscura、不启动 Chromium、不通过 WebView 执行网页。

实现分层：

- `omnibrowser`：Go HTML5 解析、CSS 选择器、Goja JavaScript、DOM、HTTP 会话与并发抓取。不导入 OmniUI 或原生宿主。
- `omnibrowser/render`：可选 Go 原生语义 HTML 预览和 PNG 图像，使用 OmniUI 排版。不是 CSS 像素等价截图。
- `omnibrowser/cdp`：可选单目标 CDP 子集，使用已有 coder/websocket。
- `cmd/omnibrowser`：fetch / html / scrape / serve 命令。
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

`SetContent` 执行内联经典脚本，不下载资源。`Navigate` 支持 HTTP(S)、最多十次重定向、Cookie 和同源外部经典脚本；外部脚本按文档顺序执行，不模拟 async/defer。跨源脚本和跨源脚本重定向明确失败。页面脚本异常记录在 `Snapshot.ScriptErrors`，默认像普通浏览器一样继续加载；CLI 将异常输出到 stderr，Studio 展示错误提示。`StrictScripts: true` 可选择遇到异常立即失败；超时/取消总是失败。已经发生的 DOM 修改不回滚。

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

并发抓取保持输入顺序，每个 URL 独立会话，最大并发 32，逐项报告错误。CDP 服务仅允许绑定 loopback IP，默认生成随机 Bearer token，启动时写到 stderr。HTTP/WebSocket 请求均需 `Authorization: Bearer ...`。路径为 `/json/version` 与 `/devtools/page/1`。

已支持 CDP：`Browser.getVersion`、`Page.navigate`、`Page.setDocumentContent`、`Runtime.evaluate`（对象需 `returnByValue`）、`Page.captureScreenshot`（1024×768 PNG 语义预览）。其他方法明确返回协议错误。多目标、对象句柄、生命周期事件、完整 DOM/Network/Input 域与客户端兼容尚未实现。

## Studio 与绘制

演示初始页面为内置 HTML，加载时真实执行 JavaScript，按钮操作改变 DOM。URL 导航与 JS 执行由 native host 后台任务完成，结果通过 `runtime/dispatch` 返回 UI；关闭演示窗口关闭 Page。离屏测试使用同步执行器，导航入口禁用以避免测试依赖公网。

语义预览支持标题、正文、列表内容、按钮、输入、链接文本和分隔线；按钮/输入有 ID 时可以与引擎交互。最多生成 500 个节点；图片显示 alt 文本。CSS 样式表、flex/grid/table、精确 inline flow、Web 字体、资源图片、Canvas、PDF、screencast 和完整页面截图尚未实现。原生预览是受限阅读/交互视图，不应作为网站像素验收工具。

## 来源、许可证与验证

Obscura README/Cargo 元数据使用 Apache-2.0，作为功能与分层参考；没有复制实现体或资产。新增实现使用 Goja（MIT）、Cascadia（BSD-2-Clause）、x/net（BSD-3-Clause）；传递依赖许可证副本位于 `licenses/`。共享 OmniUI/字体与 WebSocket 许可证沿用仓库原有记录。

`capabilities.json` 保留完整参考领域与缺口。没有移用 Obscura 的内存/速度数据来证明本实现的性能。

```sh
CGO_ENABLED=0 go test ./omnibrowser/...
go test -race ./omnibrowser/...
sh scripts/check.sh
```

测试覆盖真实本地 HTTP Cookie/重定向/外部脚本、DOM/JS 交互、超时恢复、资源配额、并发抓取、CDP 认证与失败、Go 图像输出，以及 Studio 工作线程与 UI 结果应用分离。

实际公网检查：`fetch --eval 'document.title' https://example.com` 返回 `"Example Domain"`，同时报告了该次响应中一段脚本访问未支持属性的异常。此结果只证明 HTTP/DOM/求值链路可用，不代表该页面全部脚本或渲染已兼容。
