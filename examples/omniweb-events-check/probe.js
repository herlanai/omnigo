import { OmniWebClient, OmniWebEvents, OmniWebError } from "/sdk.js";
const rpc = new OmniWebClient();
const events = new OmniWebEvents();
function assert(value, message) { if (!value) throw new Error(message); }
async function rejects(promise, code) {
  try { await promise; } catch (error) { assert(error instanceof OmniWebError && error.code === code, `expected ${code}, got ${error}`); return; }
  throw new Error(`missing ${code} failure`);
}
try {
  const all = [], first = [];
  const on = events.on("progress", value => { all.push(value); if (all.length === 3) on(); });
  const once = events.event("progress").once(value => first.push(value));
  await Promise.all([on.ready, once.ready]);
  assert(await rpc.call("emit", {}) === 3, "publish result");
  await Promise.all([on.done, once.done]);
  assert(JSON.stringify(all) === JSON.stringify(["中文😀", "line\nbreak", "last"]), "ordered Unicode payloads");
  assert(JSON.stringify(first) === JSON.stringify(["中文😀"]), "once repeated or missing");
  const denied = events.on("private", () => { throw new Error("unauthorized event"); });
  await rejects(denied.ready, "http_403"); await rejects(denied.done, "http_403");
  const abort = new AbortController();
  const canceled = events.on("progress", () => {}, {signal: abort.signal});
  await canceled.ready; abort.abort(); await canceled.done;
  const closing = events.on("progress", () => {}); await closing.ready;
  assert(await rpc.call("stop", {}) === true, "stop result");
  await rejects(closing.done, "closed");
  document.querySelector("#status").textContent = "All live WebView event checks passed";
  await rpc.call("report", {passed:true,detail:"all checks passed"});
} catch (error) {
  document.querySelector("#status").textContent = String(error);
  await rpc.call("report", {passed:false,detail:String(error)}).catch(() => {});
} finally { events.close(); }
