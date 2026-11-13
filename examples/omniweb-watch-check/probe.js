import {OmniWebClient,OmniWebStreams,OmniWebError} from "/sdk.js";
const rpc=new OmniWebClient(),streams=new OmniWebStreams();
function assert(value,message){if(!value)throw new Error(message)}
async function rejects(promise,code){
 try{await promise}catch(error){assert(error instanceof OmniWebError&&error.code===code,`expected ${code}: ${error}`);return}
 throw new Error(`missing ${code} failure`);
}
async function opened(path,root="project"){
 const call=streams.call("watch.observe",{root,path});
 const iterator=call[Symbol.asyncIterator]();
 const batch=await iterator.next();
 assert(!batch.done&&batch.value.ready===true&&batch.value.events.length===0,"watch snapshot not ready");
 return {call,iterator};
}
try{
 if(sessionStorage.getItem("watch-navigation")==="pending"){
  sessionStorage.removeItem("watch-navigation");
  assert(await rpc.call("stopped","navigation"),"navigation context retained");
  await rpc.call("stage","navigation canceled old watch");
  await opened("shutdown");
  document.querySelector("#status").textContent="File checks passed; verifying application shutdown";
  await rpc.call("report",{passed:true,detail:"all watch checks passed"});
 }else{
  await rpc.call("stage","module loaded");
  const privateWatch=streams.call("watch.private",{root:"project",path:"normal"});
  await rejects(privateWatch.result,"http_403");
  for(const request of [{root:"unknown"},{root:"project",path:"../outside"},{root:"shallow",path:"normal",recursive:true},{root:"shallow",path:"normal"}]){
   const denied=streams.call("watch.observe",request);
   assert((await denied.result).code==="denied","watch denial was not classified");
  }
  const {call,iterator}=await opened("normal");
  await rpc.call("stage","snapshot ready");
  const expected=[
   {op:"create",path:"文件.txt",step:"create"},
   {op:"write",path:"文件.txt",step:"write"},
   {op:"rename",path:"moved.txt",oldPath:"文件.txt",step:"rename"},
   {op:"remove",path:"moved.txt",step:"remove"},
  ];
  for(const want of expected){
   assert(await rpc.call("mutate",want.step),"mutation failed");
   const batch=await iterator.next();
   assert(!batch.done&&!batch.value.ready&&batch.value.events.length===1,"unexpected event batch/ignored file leaked");
   const event=batch.value.events[0];
   assert(event.op===want.op&&event.path===want.path&&event.oldPath===want.oldPath&&event.isDir===false,`incorrect ${want.step}: ${JSON.stringify(event)}`);
   await rpc.call("stage",`${want.step} event checked`);
  }
  await iterator.return();
  await rejects(call.result,"canceled");
  assert(await rpc.call("stopped","normal"),"page close did not cancel watch");
  const single=await opened(".","single");
  for(const step of ["write","remove"]){
   assert(await rpc.call("mutate",`file-${step}`),"single-file mutation failed");
   const batch=await single.iterator.next();
   assert(!batch.done&&batch.value.events.length===1,"missing file-root event");
   const event=batch.value.events[0];
   assert(event.op===step&&event.path==="."&&event.isDir===false,"incorrect file-root event");
  }
  assert((await single.iterator.next()).done,"removed file stream did not end");
  assert(Object.keys(await single.call.result).length===0,"file stream did not complete successfully");
  assert(await rpc.call("stopped","single"),"completed file watch context retained");
  await rpc.call("stage","file root write/removal/completion checked");
  await opened("navigation");
  sessionStorage.setItem("watch-navigation","pending");
  location.reload();
 }
}catch(error){
 document.querySelector("#status").textContent=String(error);
 await rpc.call("report",{passed:false,detail:String(error)}).catch(()=>{});
}
