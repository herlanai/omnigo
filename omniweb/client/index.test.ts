import { test } from "node:test";
import assert from "node:assert/strict";
import { HTTPTransport, OmniWebClient, OmniWebError } from "./index.ts";
import type { Request, Transport } from "./index.ts";

type API = { "demo.echo": { input: { text: string }; output: string } };
const fault = (code: string) => (e: unknown) => e instanceof OmniWebError && e.code === code;

test("typed calls validate correlation and recover slots", async () => {
  let bad = true;
  const transport: Transport = { async exchange(request) {
    const r = request as Request;
    return { v: 1, id: bad ? "wrong" : r.id, result: "hello" };
  } };
  const client = new OmniWebClient<API>(transport, 1);
  await assert.rejects(client.call("demo.echo", { text: "hi" }), fault("protocol"));
  bad = false;
  assert.equal(await client.call("demo.echo", { text: "hi" }), "hello");
});

test("abort and pending limits propagate to the transport", async () => {
  const transport: Transport = { exchange(_, signal) {
    return new Promise((_, reject) => signal.addEventListener("abort", () => reject(signal.reason), { once: true }));
  } };
  const client = new OmniWebClient<API>(transport, 1);
  const controller = new AbortController();
  const pending = client.call("demo.echo", { text: "hi" }, { signal: controller.signal });
  await assert.rejects(client.call("demo.echo", { text: "hi" }), fault("busy"));
  controller.abort(new Error("stopped"));
  await assert.rejects(pending, /stopped/);
});

test("errors and malformed success/error combinations are rejected", async () => {
  const client = new OmniWebClient<API>({ async exchange(req) {
    return { v: 1, id: (req as Request).id, error: { code: "forbidden", message: "Denied" } };
  } });
  await assert.rejects(client.call("demo.echo", { text: "hi" }), fault("forbidden"));
  const malformed = new OmniWebClient<API>({ async exchange(req) {
    return { v: 1, id: (req as Request).id, result: null, error: null };
  } });
  await assert.rejects(malformed.call("demo.echo", { text: "hi" }), fault("protocol"));
});

test("HTTP is same-origin, redirects forbidden, response bytes bounded", async () => {
  const request: Request = { v: 1, id: "1", method: "demo.echo", params: {} };
  const transport = new HTTPTransport({ maxResponseBytes: 8, fetch: (async (url, options) => {
    assert.equal(url, "/omniweb/v1/call");
    assert.equal(options?.redirect, "error");
    return new Response('{"result":"too large"}', { headers: { "Content-Type": "application/json" } });
  }) as typeof fetch });
  await assert.rejects(transport.exchange(request, new AbortController().signal), fault("too_large"));
  assert.throws(() => new HTTPTransport({ base: "https://example.com" }), fault("configuration"));
});
