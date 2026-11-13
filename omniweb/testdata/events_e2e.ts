// Executed by the Go integration test against a real loopback HTTP server.
import assert from "node:assert/strict";
import { HTTPTransport, OmniWebClient, OmniWebEvents, OmniWebError } from "../client/index.ts";

const origin = process.argv[2];
const realFetch: typeof fetch = (input, init) => {
  const headers = new Headers(init?.headers);
  headers.set("Origin", origin);
  return fetch(new URL(String(input), origin), { ...init, headers });
};
const rpc = new OmniWebClient<{
  emit: {input: {}; output: number};
  stop: {input: {}; output: boolean};
}>(new HTTPTransport({fetch: realFetch}));
const events = new OmniWebEvents<{progress: string; private: string}>({fetch: realFetch});
const all: string[] = [];
const first: string[] = [];
try {
  const on = events.on("progress", value => { all.push(value); if (all.length === 3) on(); });
  const once = events.event("progress").once(value => first.push(value));
  await Promise.all([on.ready, once.ready]);
  assert.equal(await rpc.call("emit", {}), 3);
  await Promise.all([on.done, once.done]);
  assert.deepEqual(all, ["中文😀", "line\nbreak", "last"]);
  assert.deepEqual(first, ["中文😀"]);

  const denied = events.on("private", () => assert.fail("unauthorized event"));
  await assert.rejects(denied.ready, e => e instanceof OmniWebError && e.code === "http_403");
  await assert.rejects(denied.done, e => e instanceof OmniWebError && e.code === "http_403");

  const abort = new AbortController();
  const canceled = events.on("progress", () => assert.fail("event after cancellation"), {signal: abort.signal});
  await canceled.ready; abort.abort(); await canceled.done;

  const closing = events.on("progress", () => assert.fail("unexpected event"));
  await closing.ready;
  assert.equal(await rpc.call("stop", {}), true);
  await assert.rejects(closing.done, e => e instanceof OmniWebError && e.code === "closed");
  console.log("Go/TypeScript events: order, Unicode, once, permissions, cancellation, broker close passed");
} finally {
  events.close();
}
