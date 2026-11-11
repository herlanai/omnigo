# OmniWeb 应用

默认使用 TanStack Start + React + TypeScript + shadcn 配置 + Tailwind CSS。Go 业务通过 OmniWeb v1 调用。当前宿主运行在浏览器，尚不包含原生桌面窗口适配。

需要 Go 1.26+、Node.js 24+。

```sh
npm --prefix ui ci
npm run build:web
npm start
```

打开 http://127.0.0.1:7331。只托管 `ui/dist/client`，不需要 Node 生产服务器。

开发时开两个终端，分别运行 `npm run dev:host` 与 `npm run dev:web`，访问 http://127.0.0.1:5173。Vite 将 OmniWeb 请求代理到 Go。不要混用 localhost 与 127.0.0.1；来源校验采用精确匹配。

添加 shadcn 组件：在 `ui` 目录运行 `npx shadcn@latest add dialog`。当前内置 Button 与 Input 是采用 shadcn 目录、主题和变体约定编写的基础组件；新增官方组件由 shadcn CLI 生成。

业务方法在 `app/main.go` 注册，前端契约在 `ui/src/lib/services.ts` 定义。自动 Go → TypeScript 代码生成尚未实现，修改接口时需同步两处。

`ui/src/lib/omniweb.ts` 来自框架的唯一 SDK 源码；默认有超时、取消、并发上限和协议校验。框架升级后需要同步该文件。模板包含已验证的 `ui/package-lock.json`，使用 `npm ci` 安装；添加组件后请一起提交依赖和锁文件的变更。

不要将 TanStack server functions 或 `dist/server` 作为桌面业务后端。不要把敏感权限写入客户端请求。当前示例仅监听 loopback；部署远程多用户服务需要独立认证与授权设计。
