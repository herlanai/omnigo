import { test } from "node:test";
import assert from "node:assert/strict";
import { OmniWebEvents, OmniWebError } from "./index.ts";
const ready = '{"v":1,"kind":"ready"}\n';
const event = (data: unknown) => JSON.stringify({v:1,kind:"event",topic:"test",data})+"\n";
function fixture() {
  let controller!: ReadableStreamDefaultController<Uint8Array>;
  const stream = new ReadableStream<Uint8Array>({ start(c) { controller=c; } });
  const fetcher = (async () => new Response(stream,{headers:{"Content-Type":"application/x-ndjson"}})) as typeof fetch;
  return { client:new OmniWebEvents<{test:string}>({fetch:fetcher}), push:(s:string)=>controller.enqueue(new TextEncoder().encode(s)), bytes:(b:Uint8Array)=>controller.enqueue(b) };
}
test("once receives one event across byte-split UTF8 frames",async()=>{
 const f=fixture();const got:string[]=[];
 const sub=f.client.event("test").once(v=>got.push(v));
 const raw=new TextEncoder().encode(ready+event("中文😀")+event("ignored"));
 for(const byte of raw)f.bytes(new Uint8Array([byte]));
 await sub.ready;await sub.done;
 assert.deepEqual(got,["中文😀"]);
});
test("unsubscribe during callback prevents further same-chunk delivery",async()=>{
 const f=fixture();const got:string[]=[];
 const sub=f.client.on("test",v=>{got.push(v);sub();});
 f.push(ready+event("one")+event("two"));await sub.done;
 assert.deepEqual(got,["one"]);f.client.close();
 assert.throws(()=>f.client.on("test",()=>{}));
});
test("server overflow and malformed ordering reject readiness/done",async()=>{
 for(const body of ['{"v":1,"kind":"error","code":"overflow"}\n',event("early")]){
  const f=fixture();const sub=f.client.on("test",()=>assert.fail("unexpected event"));f.push(body);
  await assert.rejects(sub.done,OmniWebError);await assert.rejects(sub.ready,OmniWebError);
 }
});
test("listener failure is observable, even for once",async()=>{
 const f=fixture();const failure=new Error("listener failed");
 const sub=f.client.once("test",()=>{throw failure});f.push(ready+event("x"));
 await assert.rejects(sub.done,e=>e===failure);
});
test("abort before ready releases subscription and rejects ready",async()=>{
 const f=fixture();const c=new AbortController();
 const sub=f.client.on("test",()=>{}, {signal:c.signal});c.abort();
 await sub.done;await assert.rejects(sub.ready);f.client.close();
});
test("oversized frame terminates with a bounded parser error",async()=>{
 const f=fixture();const sub=f.client.on("test",()=>{});
 f.bytes(new Uint8Array(1024*1024+1).fill(32));
 await assert.rejects(sub.done,e=>e instanceof OmniWebError&&e.code==="too_large");
});
test("once does not swallow listener AbortError",async()=>{
 const f=fixture();const failure=new DOMException("user callback","AbortError");
 const sub=f.client.once("test",()=>{throw failure});f.push(ready+event("x"));
 await assert.rejects(sub.done,e=>e===failure);
});
test("abort stops a waiting reader after ready",async()=>{
 const f=fixture();const c=new AbortController();const sub=f.client.on("test",()=>{}, {signal:c.signal});
 f.push(ready);await sub.ready;c.abort();await sub.done;
});
test("same-origin validation and active subscription limit",async()=>{
 assert.throws(()=>new OmniWebEvents({base:"https://other.test"}));
 const client=new OmniWebEvents<{test:string}>({fetch:(async(_input,init)=>new Promise((_resolve,reject)=>{
  const signal=init!.signal!;if(signal.aborted)reject(signal.reason);else signal.addEventListener("abort",()=>reject(signal.reason),{once:true});
 })) as typeof fetch});
 const subs=Array.from({length:64},()=>client.on("test",()=>{}));
 assert.throws(()=>client.on("test",()=>{}),e=>e instanceof OmniWebError&&e.code==="busy");
 client.close();await Promise.all(subs.map(s=>s.done));
});
