# 创建、运行和交付应用

Go 1.26+。从本地 OmniGo 仓库安装 CLI（确保 Go 的 bin 目录在 PATH 中）：

```sh
go install ./cmd/omni
```

选择一种模板；以下命令在框架仓库根目录执行，目标必须为空或不存在：

```sh
omni init --template go --framework "$PWD" --module example.com/notes \
  --name "Notes" --id com.example.notes /tmp/notes
# 或 --template webview：嵌入静态页面的桌面工作区
# 或 --template browser：TanStack + React + TypeScript 浏览器应用
```

默认模板是 go（旧默认浏览器模板现在需显式 --template browser）。
Go 与静态 WebView 模板不需要 Node；浏览器模板需要 Node 24+ 与 npm。
模板使用本地 go.mod replace，移动框架目录后需更新替换路径。

## Go / WebView 桌面

```sh
cd /tmp/notes
go mod tidy
omni doctor
omni dev
# 退出开发进程后：
omni build
omni package
```

同一个 `omnigo.json` 配置工具链、构建、监视目录与 bundle。默认 darwin/arm64；
`dev` 要求宿主与目标相同，`package` 要求 macOS 宿主。
输出 `releases/0.1.0/Notes.app` 与 `Notes-darwin-arm64.dmg`。再次打包需更换
bundle.destination，旧发布不会被覆盖。图标为按应用名称生成的起步图案，正式
产品应替换 `app/assets/icon.png` 和 `AppIcon.icns`。

.app 嵌入全部运行资源，不依赖 Node、Go 或源码目录。复制到独立位置后验证启动、
图标、窗口、输入、关闭与清理。签名为 ad-hoc；没有 Developer ID 或公证。

WebView 模板直接编辑 `app/web/index.html`，示例为本地便笺交互。
若采用 React/TypeScript 构建，把输出放到该嵌入目录，在 `before` 中添加构建命令，
并将 `toolchain` 设为 `web`。RPC 必须使用框架的标准 SDK，参见 [OmniWeb](omniweb.md)。

## 浏览器

```sh
cd /tmp/browser-app
go mod tidy
npm --prefix ui ci
omni doctor
omni dev
```

访问 http://127.0.0.1:7331。`omni dev` 会重新构建前端及 Go 并重启；需要 Vite HMR
时使用生成 README 中的双终端命令。`omni build` 输出可执行文件；部署时同时携带
`ui/dist/client`，或用 `-assets` 指向该静态目录。不需要 Node 生产服务器。
浏览器模板没有 bundle 配置，`omni package` 不将它冒充桌面应用。

## 工具检查

`toolchain` 支持 `go`、`web`、`studio`。未配置时按 `go` 检查；旧配置或自定义
构建步骤若使用 Node，应显式设为 `web`。`studio` 检查 Node 24+ 与 pnpm 10.33+。

```sh
omni doctor --config /path/to/omnigo.json
omni doctor --profile web --check --json
omni doctor --profile studio
omni doctor --all --json
```

最后一个命令仅列出当前 CLI 构建的后端，不执行工具检查。[当前能力矩阵](status.md)
区分构建、原生探针与人工验收；命令成功不自动提升平台支持等级。
