import { createRootRoute, HeadContent, Outlet, Scripts } from "@tanstack/react-router";
import type { ReactNode } from "react";
import styles from "@/styles.css?url";

export const Route = createRootRoute({
  head: () => ({
    meta: [{ charSet: "utf-8" }, { name: "viewport", content: "width=device-width, initial-scale=1" }, { title: "OmniWeb" }],
    links: [{ rel: "stylesheet", href: styles }],
  }),
  component: () => <Outlet />,
  shellComponent: ({ children }: { children: ReactNode }) => (
    <html lang="zh-CN"><head><HeadContent /></head><body>{children}<Scripts /></body></html>
  ),
  notFoundComponent: () => <main className="p-12"><h1>页面不存在</h1><a href="/">返回首页</a></main>,
});
