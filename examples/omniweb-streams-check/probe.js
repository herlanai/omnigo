import { OmniWebClient, OmniWebStreams, OmniWebError } from "/sdk.js";
const rpc = new OmniWebClient();
const acknowledgments=[];
const streams = new OmniWebStreams({fetch:(path,options)=>{
  if(path.endsWith("/ack"))acknowledgments.push(JSON.parse(options.body).seq);
  return fetch(path,options);
}});
function assert(value, message) { if (!value) throw new Error(message); }
async function rejects(promise, code) {
  try { await promise; } catch (error) { assert(error instanceof OmniWebError && error.code === code, `expected ${code}, got ${error}`); return; }
  throw new Error(`missing ${code} failure`);
}
try {
  if (sessionStorage.getItem("stream-probe-navigation") === "pending") {
    sessionStorage.removeItem("stream-probe-navigation");
    assert(await rpc.call("navigationStopped", {}) === true, "navigation did not cancel old page stream");
    await rpc.call("stage", "navigation checked");
    // Keep a server method active through application/window shutdown. Go verifies
    // its context was canceled and the handler returned before Run completes.
    const holding = streams.call("hold", {});
    await holding.ready;
    document.querySelector("#status").textContent = "Live WebView stream checks passed; checking shutdown";
    await rpc.call("report", {passed:true,detail:"all page checks passed"});
  } else {
    await rpc.call("stage","module loaded");
    const call = streams.call("count", 32);
    await call.ready;
    await rpc.call("stage","stream ready");
    let count = 0;
    for await (const value of call) {
      assert(value === "中文😀".repeat(8192), "Unicode value mismatch");
      count++;
      if(count%8===0)await rpc.call("stage",`consumed ${count}`);
    }
    assert(count === 32 && await call.result === 32, "value count / final result");
    assert(acknowledgments.length>=4&&acknowledgments.length<=32,"bounded cumulative acknowledgments");
    assert(acknowledgments.every((seq,i)=>i===0||seq>acknowledgments[i-1]),"monotonic acknowledgments");
    await rpc.call("stage",`result received; ${acknowledgments.length} cumulative acknowledgments`);
    const callbackCall=streams.call("count",2);
    let callbackCount=0,callbackClosed=0;
    callbackCall.onmessage=value=>{assert(value==="中文😀".repeat(8192),"callback Unicode");callbackCount++};
    callbackCall.onclose=()=>{assert(callbackCall.closed,"closed state");callbackCall.close();callbackClosed++};
    assert(await callbackCall.result===2&&callbackCount===2&&callbackClosed===1,"callback delivery");
    await rpc.call("stage","callbacks checked");
    const shared=streams.call("count",3);
    const left=shared[Symbol.asyncIterator](),right=shared[Symbol.asyncIterator]();
    const split=await Promise.all([left.next(),right.next(),left.next(),right.next()]);
    assert(JSON.stringify(split.map(item=>item.done))==="[false,false,false,true]","shared iterator completion");
    assert(split.slice(0,3).every(item=>item.value==="中文😀".repeat(8192)),"shared iterator values");
    assert(await shared.result===3,"shared iterator result");
    await rpc.call("stage","multiple iterators checked");
    const denied = streams.call("private", 1);
    await rejects(denied.ready, "http_403");
    await rejects(denied.result, "http_403");
    await rpc.call("stage","permission checked");
    const early = streams.call("count", 1000);
    for await (const value of early) { assert(value.length > 0, "missing first value"); break; }
    await rejects(early.result, "canceled");
    await rpc.call("stage","early exit checked");
    const abort = new AbortController();
    const canceled = streams.call("count", 1000, {signal: abort.signal});
    await canceled.ready; abort.abort(); await rejects(canceled.result, "canceled");
    await rpc.call("stage","abort checked");
    const navigating = streams.call("navigate", {});
    await navigating.ready;
    assert(await rpc.call("navigationEntered", {}) === true, "navigation handler not entered");
    sessionStorage.setItem("stream-probe-navigation", "pending");
    location.reload();
  }
} catch (error) {
  document.querySelector("#status").textContent = String(error);
  await rpc.call("report", {passed:false,detail:String(error)}).catch(() => {});
}
