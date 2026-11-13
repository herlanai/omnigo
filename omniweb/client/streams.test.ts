import {test} from "node:test";
import assert from "node:assert/strict";
import {OmniWebStreams,OmniWebError} from "./index.ts";
type API={count:{input:number;value:string;output:number}};
function fixture(ackStatus=204,onListenerError?:(error:unknown)=>void,ackWait?:()=>Promise<void>){
 const acknowledgments:number[]=[];let sequence=0;
 let c!:ReadableStreamDefaultController<Uint8Array>;
 let signal:AbortSignal|undefined;
 const response=new Response(new ReadableStream<Uint8Array>({start(controller){c=controller}}),{headers:{"Content-Type":"application/x-ndjson"}});
 const client=new OmniWebStreams<API>({onListenerError,fetch:(async(input,init)=>{if(String(input).endsWith("/ack")){acknowledgments.push(JSON.parse(String(init!.body)).seq);await ackWait?.();return new Response(null,{status:ackStatus});}signal=init!.signal!;return response}) as typeof fetch});
 return {client,acknowledgments,push:(frames:object[])=>c.enqueue(new TextEncoder().encode(frames.map(f=>JSON.stringify({v:1,id:"1",...(f["kind"]==="ready"?{token:"a".repeat(64)}:{}),...(f["kind"]==="value"?{seq:++sequence,ask:true}:{}),...f})).join("\n")+"\n")),signal:()=>signal};
}
test("values are consumed in order before final result",async()=>{
 const f=fixture(),call=f.client.call("count",2);
 f.push([{kind:"ready"},{kind:"value",data:"中文😀"},{kind:"value",data:"two"},{kind:"end"},{kind:"result",result:2}]);
 await call.ready;
 const values=[];for await(const value of call)values.push(value);
 assert.deepEqual(f.acknowledgments,[1,2]);assert.deepEqual(values,["中文😀","two"]);assert.equal(await call.result,2);
});
test("breaking iteration cancels request and final result",async()=>{
 const f=fixture(),call=f.client.call("count",2);
 f.push([{kind:"ready"},{kind:"value",data:"first"},{kind:"value",data:"second"}]);
 for await(const value of call){assert.equal(value,"first");break;}
 await assert.rejects(call.result,e=>e instanceof OmniWebError&&e.code==="canceled");
 assert.equal(f.signal()?.aborted,true);
});
test("server method error belongs to final result after end",async()=>{
 const f=fixture(),call=f.client.call("count",0);
 f.push([{kind:"ready"},{kind:"end"},{kind:"result",error:{code:"internal",message:"failed"}}]);
 const values=[];for await(const v of call)values.push(v);
 assert.deepEqual(values,[]);
 await assert.rejects(call.result,e=>e instanceof OmniWebError&&e.code==="internal");
});
test("correlation and null faults are rejected",async()=>{
 for(const frame of [{v:1,id:"other",kind:"ready"},{kind:"result",error:null}]){
  const f=fixture(),call=f.client.call("count",0);
  f.push([{kind:"ready"},{kind:"end"},frame]);
  await assert.rejects(call.result,e=>e instanceof OmniWebError&&e.code==="protocol");
 }
});
test("closing an unconsumed stream releases pending producer",async()=>{
 const f=fixture(),call=f.client.call("count",100);
 f.push([{kind:"ready"},{kind:"value",data:"pending"}]);
 await call.ready;f.client.close();
 await assert.rejects(call.result,e=>e instanceof OmniWebError&&e.code==="canceled");
});
test("multiple iterators share values in next-request order",async()=>{
 const f=fixture(),call=f.client.call("count",3);
 const a=call[Symbol.asyncIterator](),b=call[Symbol.asyncIterator]();
 const first=b.next(),second=a.next(),third=b.next(),end=a.next();
 f.push([{kind:"ready"},{kind:"value",data:"one"},{kind:"value",data:"two"},{kind:"value",data:"three"},{kind:"end"},{kind:"result",result:3}]);
 assert.deepEqual(await first,{done:false,value:"one"});
 assert.deepEqual(await second,{done:false,value:"two"});
 assert.deepEqual(await third,{done:false,value:"three"});
 assert.deepEqual(await end,{done:true,value:undefined});
 assert.deepEqual(await b.next(),{done:true,value:undefined});
 assert.equal(await call.result,3);
});


test("failed consumption acknowledgment cancels producer",async()=>{
 const f=fixture(403),call=f.client.call("count",1);
 f.push([{kind:"ready"},{kind:"value",data:"one"}]);
 await assert.rejects(async()=>{for await(const _ of call){}},e=>e instanceof OmniWebError&&e.code==="stream_ack");
 await assert.rejects(call.result);assert.equal(f.signal()?.aborted,true);
});
test("sequence gaps cannot be acknowledged",async()=>{
 const f=fixture(),call=f.client.call("count",1);
 f.push([{kind:"ready"},{kind:"value",data:"one",seq:2}]);
 await assert.rejects(call.result,e=>e instanceof OmniWebError&&e.code==="protocol");
 assert.deepEqual(f.acknowledgments,[]);
});

test("retired producer permits consumption of buffered tail before result",async()=>{
 const f=fixture(410),call=f.client.call("count",1);
 f.push([{kind:"ready"},{kind:"value",data:"tail"},{kind:"end"},{kind:"result",result:1}]);
 const values=[];for await(const v of call)values.push(v);
 assert.deepEqual(values,["tail"]);assert.equal(await call.result,1);
});
test("retired session does not turn missing final result into success",async()=>{
 const f=fixture(410),call=f.client.call("count",1);
 f.push([{kind:"ready"},{kind:"value",data:"tail"},{kind:"result",result:1}]);
 await assert.rejects(async()=>{for await(const _ of call){}},e=>e instanceof OmniWebError&&e.code==="protocol");
 await assert.rejects(call.result,e=>e instanceof OmniWebError&&e.code==="protocol");
});


test("onmessage drains ordered values and onclose precedes result",async()=>{
 const f=fixture(),call=f.client.call("count",2),seen:string[]=[];
 call.onmessage=value=>seen.push(value);
 call.onclose=()=>{assert.equal(call.closed,true);seen.push("closed");};
 f.push([{kind:"ready"},{kind:"value",data:"one"},{kind:"value",data:"two"},{kind:"end"},{kind:"result",result:2}]);
 assert.equal(await call.result,2);seen.push("result");
 assert.deepEqual(seen,["one","two","closed","result"]);
 assert.deepEqual(f.acknowledgments,[1,2]);
 call.close();assert.equal(seen.filter(v=>v==="closed").length,1);
});
test("callback removal transfers later values to iteration without duplication",async()=>{
 const f=fixture(),call=f.client.call("count",2),seen:string[]=[];
 let first!:()=>void;const consumed=new Promise<void>(resolve=>first=resolve);
 call.onmessage=value=>{seen.push(value);call.onmessage=null;first();};
 f.push([{kind:"ready"},{kind:"value",data:"one"},{kind:"value",data:"two"},{kind:"end"},{kind:"result",result:2}]);
 await consumed;
 for await(const v of call)seen.push(v);
 assert.equal(await call.result,2);assert.deepEqual(seen,["one","two"]);
});
test("listener exceptions are reported without failing the Go result",async()=>{
 const errors:unknown[]=[];const f=fixture(204,e=>errors.push(e)),call=f.client.call("count",2);
 const messageError=new Error("listener"),closeError=new Error("close");
 let n=0;call.onmessage=()=>{n++;throw messageError};call.onclose=()=>{throw closeError};
 f.push([{kind:"ready"},{kind:"value",data:"one"},{kind:"value",data:"two"},{kind:"end"},{kind:"result",result:2}]);
 assert.equal(await call.result,2);assert.equal(n,2);
 assert.deepEqual(errors,[messageError,messageError,closeError]);
});
test("closing in callback stops buffered messages and notifies once",async()=>{
 const f=fixture(),call=f.client.call("count",2);let values=0,closes=0;
 call.onmessage=()=>{values++;call.close();call.close()};
 call.onclose=()=>{closes++;assert.equal(call.closed,true)};
 f.push([{kind:"ready"},{kind:"value",data:"one"},{kind:"value",data:"two"},{kind:"end"},{kind:"result",result:2}]);
 await assert.rejects(call.result,e=>e instanceof OmniWebError&&e.code==="canceled");
 assert.equal(values,1);assert.equal(closes,1);assert.equal(f.signal()?.aborted,true);
});
test("late callback attaches to the waiting value",async()=>{
 const f=fixture(),call=f.client.call("count",1);const seen:string[]=[];
 f.push([{kind:"ready"},{kind:"value",data:"buffered"},{kind:"end"},{kind:"result",result:1}]);
 await call.ready;
 call.onmessage=value=>seen.push(value);
 assert.equal(await call.result,1);assert.deepEqual(seen,["buffered"]);
});

test("close in onclose preserves the pending method result",async()=>{
 const f=fixture(),call=f.client.call("count",0);
 let closed!:()=>void;const ended=new Promise<void>(resolve=>closed=resolve);
 call.onclose=()=>{call.close();closed()};
 f.push([{kind:"ready"},{kind:"end"}]);await ended;
 assert.equal(f.signal()?.aborted,false);
 f.push([{kind:"result",result:0}]);assert.equal(await call.result,0);
});
test("client shutdown still cancels a method after values have ended",async()=>{
 const f=fixture(),call=f.client.call("count",0);
 let closed!:()=>void;const ended=new Promise<void>(resolve=>closed=resolve);
 call.onclose=closed;
 f.push([{kind:"ready"},{kind:"end"}]);await ended;f.client.close();
 await assert.rejects(call.result,e=>e instanceof OmniWebError&&e.code==="canceled");
});


test("return on an iterator with pending next cancels all consumers",async()=>{
 const f=fixture(),call=f.client.call("count",1);
 const a=call[Symbol.asyncIterator](),b=call[Symbol.asyncIterator]();
 const all=Promise.allSettled([a.next(),b.next(),call.result]);
 await a.return!();
 const settled=await all;
 assert.deepEqual(settled.slice(0,2),[{status:"fulfilled",value:{done:true,value:undefined}},{status:"fulfilled",value:{done:true,value:undefined}}]);
 assert.equal(settled[2].status,"rejected");
 assert.equal(call.closed,true);
});
test("callback priority does not lose waiting iterator requests",async()=>{
 const f=fixture(),call=f.client.call("count",2),seen:string[]=[];
 const iterator=call[Symbol.asyncIterator](),next=iterator.next();
 call.onmessage=value=>{seen.push(value);call.onmessage=null};
 f.push([{kind:"ready"},{kind:"value",data:"callback"},{kind:"value",data:"iterator"},{kind:"end"},{kind:"result",result:2}]);
 assert.deepEqual(await next,{done:false,value:"iterator"});
 assert.deepEqual(seen,["callback"]);assert.equal(await call.result,2);
});


test("close during acknowledgment completes the reserved iterator without delivery",async()=>{
 let entered!:()=>void,release!:()=>void;
 const started=new Promise<void>(resolve=>entered=resolve),gate=new Promise<void>(resolve=>release=resolve);
 const f=fixture(204,undefined,()=>{entered();return gate}),call=f.client.call("count",1);
 const iterator=call[Symbol.asyncIterator](),next=iterator.next();
 f.push([{kind:"ready"},{kind:"value",data:"reserved"}]);
 await started;await iterator.return!();release();
 assert.deepEqual(await next,{done:true,value:undefined});
 await assert.rejects(call.result,e=>e instanceof OmniWebError&&e.code==="canceled");
});


test("only marked values acknowledge all preceding consumption",async()=>{
 const f=fixture(),call=f.client.call("count",4),values:string[]=[];
 call.onmessage=value=>values.push(value);
 f.push([{kind:"ready"},{kind:"value",data:"one",ask:false},{kind:"value",data:"two",ask:true},{kind:"value",data:"three",ask:false},{kind:"value",data:"four",ask:true},{kind:"end"},{kind:"result",result:4}]);
 assert.equal(await call.result,4);
 assert.deepEqual(values,["one","two","three","four"]);
 assert.deepEqual(f.acknowledgments,[2,4]);
});
test("short completed stream requires no acknowledgment request",async()=>{
 const f=fixture(),call=f.client.call("count",1);
 call.onmessage=()=>{};
 f.push([{kind:"ready"},{kind:"value",data:"tail",ask:false},{kind:"end"},{kind:"result",result:1}]);
 assert.equal(await call.result,1);assert.deepEqual(f.acknowledgments,[]);
});
test("invalid acknowledgment marker fails the stream",async()=>{
 const f=fixture(),call=f.client.call("count",1);
 f.push([{kind:"ready"},{kind:"value",data:"bad",ask:"yes"}]);
 await assert.rejects(call.result,e=>e instanceof OmniWebError&&e.code==="protocol");
});

test("network boundary confirms a consumed batch before waiting for more bytes",async()=>{
 const f=fixture(204,undefined,async()=>{
  // Model a producer that cannot send the next network chunk until progress
  // arrives, even though the last consumed frame did not request an ACK.
  f.push([{kind:"end"},{kind:"result",result:2}]);
 });
 const call=f.client.call("count",2),values:string[]=[];
 call.onmessage=value=>values.push(value);
 f.push([{kind:"ready"},{kind:"value",data:"one",ask:false},{kind:"value",data:"two",ask:false}]);
 assert.equal(await call.result,2);
 assert.deepEqual(values,["one","two"]);assert.deepEqual(f.acknowledgments,[2]);
});
