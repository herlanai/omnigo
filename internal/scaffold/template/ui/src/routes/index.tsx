import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { services } from "@/lib/services";

export const Route = createFileRoute("/")({ component: Home });

function Home() {
  const [name, setName] = useState("OmniWeb");
  const [reply, setReply] = useState("准备好后，向 Go 发送第一条消息。");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);
  async function submit(event: FormEvent) {
    event.preventDefault();
    active.current?.abort();
    const controller = new AbortController();
    active.current = controller;
    setPending(true); setError("");
    try {
      const result = await services.call("app.greet", { name }, { signal: controller.signal });
      if (!controller.signal.aborted) setReply(result.message);
    } catch (e) {
      if (!controller.signal.aborted) setError(e instanceof Error ? e.message : "调用失败");
    } finally {
      if (active.current === controller) { active.current = null; setPending(false); }
    }
  }
  return (
    <main className="mx-auto flex min-h-screen max-w-5xl flex-col justify-center px-6 py-16">
      <p className="mb-6 text-xs font-medium uppercase tracking-[.3em] text-muted-foreground">OmniGo / OmniUI</p>
      <h1 className="text-5xl font-semibold tracking-tight sm:text-7xl">Hello, OmniWeb.</h1>
      <p className="mt-6 max-w-xl text-lg leading-8 text-muted-foreground">让 Web 界面与 Go 服务，从同一个协议开始。</p>
      <section className="mt-12 max-w-xl rounded-2xl border border-border bg-card p-6 shadow-sm" aria-labelledby="demo-title">
        <h2 id="demo-title" className="text-lg font-semibold">连接你的应用</h2>
        <form className="mt-6 space-y-4" onSubmit={submit}>
          <label htmlFor="name" className="block text-sm font-medium">你的名字</label>
          <Input id="name" value={name} onChange={e => setName(e.target.value)} maxLength={100} autoComplete="name" />
          <div className="flex gap-3">
            <Button type="submit" disabled={pending}>{pending ? "正在发送…" : "发送问候"}</Button>
            {pending && <Button variant="outline" onClick={() => active.current?.abort()}>取消</Button>}
          </div>
        </form>
        <p role="status" className="mt-6 rounded-lg bg-muted p-4 text-sm leading-6">{reply}</p>
        {error && <p role="alert" className="mt-4 text-sm text-destructive">{error}</p>}
      </section>
      <p className="mt-8 text-xs text-muted-foreground">TanStack Start · shadcn · OmniWeb v1</p>
    </main>
  );
}
