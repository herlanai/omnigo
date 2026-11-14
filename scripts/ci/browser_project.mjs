import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {setTimeout as delay} from 'node:timers/promises';
import {resolve,join} from 'node:path';
import {pathToFileURL} from 'node:url';
const project=resolve(process.argv[2]);
const {OmniWebClient,HTTPTransport}=await import(pathToFileURL(join(project,'ui/src/lib/omniweb.ts')));
const server=spawn(join(project,'build/app'),[],{cwd:project,stdio:['ignore','pipe','pipe']});
const stopped=once(server,'exit');
let logs='';server.stderr.on('data',b=>logs+=b);
try {
 const deadline=Date.now()+15000;
 let body;
 while(Date.now()<deadline){
  if(server.exitCode!==null)throw Error(logs);
  try {const r=await fetch('http://127.0.0.1:7331/');if(!r.ok)throw Error(r.status);body=await r.text();break;}catch(e){await delay(50);}
 }
 if(!body?.includes('<html'))throw Error('No rendered HTML');
 const asset=body.match(/(?:src|href)="(\/assets\/[^" ]+\.(?:js|css))"/);
 if(!asset)throw Error('No compiled asset reference');
 const response=await fetch('http://127.0.0.1:7331'+asset[1]);
 if(!response.ok || (await response.arrayBuffer()).byteLength===0)throw Error('Missing compiled asset');
 const client=new OmniWebClient(new HTTPTransport({fetch:(input,init)=>fetch(new URL(input,'http://127.0.0.1:7331'),{...init,headers:{...init.headers,Origin:'http://127.0.0.1:7331'}})}));
 const result=await client.call('app.greet',{name:'P0 😀'});
 if(!result.message.includes('P0 😀'))throw Error(JSON.stringify(result));
 console.log('PASS: independent browser HTML, compiled assets and canonical SDK Unicode RPC');
} finally {
 server.kill('SIGINT');
 const timer=setTimeout(()=>server.kill('SIGKILL'),5000);
 try {await stopped;} finally {clearTimeout(timer);}
}
