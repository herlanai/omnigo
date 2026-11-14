import test from "node:test";
import assert from "node:assert/strict";
import { NativeTransport } from "./index.ts";
test("native transport exchanges protocol without HTTP", async () => {
 const t = new NativeTransport(async text => { const e=JSON.parse(text);assert.equal(e.kind,"call");return JSON.stringify({v:1,id:e.request.id,result:"中文"}); });
 assert.deepEqual(await t.exchange({v:1,id:"1",method:"echo",params:{}},new AbortController().signal),{v:1,id:"1",result:"中文"});
});
test("native abort sends cancellation and settles locally", async () => {
 const sent: string[]=[];
 const t = new NativeTransport(async text=>{sent.push(JSON.parse(text).kind);if(sent.length===1)return new Promise(()=>{});return "null";});
 const c=new AbortController();const result=t.exchange({v:1,id:"1",method:"wait",params:{}},c.signal);c.abort(new Error("stop"));await assert.rejects(result,/stop/);assert.deepEqual(sent,["call","cancel"]);
});
