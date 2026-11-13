import assert from "node:assert/strict";
import {OmniWebStreams,OmniWebError} from "../client/index.ts";
const origin=process.argv[2];
const acknowledgments:number[]=[];
const client=new OmniWebStreams<{count:{input:number;value:string;output:number};private:{input:number;value:string;output:number}}>({
 fetch:((path,init)=>{if(String(path).endsWith("/ack"))acknowledgments.push(JSON.parse(String(init?.body)).seq);return fetch(new URL(String(path),origin),{...init,headers:{...init?.headers,Origin:origin}})}) as typeof fetch
});
const call=client.call("count",32);
await call.ready;
let n=0;
for await(const value of call){assert.equal(value, "中文😀".repeat(8192));n++;}
assert.equal(n,32);assert.equal(await call.result,32);
assert(acknowledgments.length>=4&&acknowledgments.length<=32);
assert(acknowledgments.every((seq,i)=>i===0||seq>acknowledgments[i-1]));
const callbackCall=client.call("count",2);
let callbackCount=0,callbackClosed=0;
callbackCall.onmessage=value=>{assert.equal(value,"中文😀".repeat(8192));callbackCount++};
callbackCall.onclose=()=>{assert.equal(callbackCall.closed,true);callbackClosed++};
assert.equal(await callbackCall.result,2);
assert.equal(callbackCount,2);assert.equal(callbackClosed,1);
const shared=client.call("count",3);
const left=shared[Symbol.asyncIterator](),right=shared[Symbol.asyncIterator]();
const split=await Promise.all([left.next(),right.next(),left.next(),right.next()]);
assert.deepEqual(split.map(item=>item.done),[false,false,false,true]);
for(const item of split.slice(0,3))assert.equal(item.value,"中文😀".repeat(8192));
assert.equal(await shared.result,3);
const denied=client.call("private",1);
await assert.rejects(denied.result,e=>e instanceof OmniWebError&&e.code==="http_403");
const canceled=client.call("count",1000);
for await(const _ of canceled){break;}
await assert.rejects(canceled.result,e=>e instanceof OmniWebError&&e.code==="canceled");
client.close();
console.log("stream SDK/Go HTTP: multi-window consumption, order, result, authorization, cancellation passed");
