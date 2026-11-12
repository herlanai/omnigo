import { spawn } from 'node:child_process';
import { mkdir, cp, writeFile, access, rm } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { licenses } from './studio-licenses.mjs';
const root=fileURLToPath(new URL('../',import.meta.url));
process.chdir(root);
const mode=process.argv[2]??'dev';
if(!['dev','build','start','package'].includes(mode))throw new Error(`Unknown Studio command: ${mode}`);
const windows=process.platform==='win32',mac=process.platform==='darwin';
const children=new Set();let stopping=false;
function terminate(child){if(child.exitCode!==null)return;try{if(windows)child.kill();else process.kill(-child.pid,'SIGTERM')}catch{}}
function stop(){if(stopping)return;stopping=true;for(const child of children)terminate(child)}
process.on('SIGINT',stop);process.on('SIGTERM',stop);
function run(command,args,options={}){
 if(stopping)return Promise.reject(new Error('Studio stopped'));
 return new Promise((resolve,reject)=>{
 const child=spawn(command,args,{cwd:root,stdio:'inherit',detached:!windows,env:process.env,...options});children.add(child);
 child.once('error',err=>{children.delete(child);reject(err)});
 child.once('exit',(code,signal)=>{children.delete(child);if(code===0||stopping)resolve();else reject(new Error(`${command} exited ${code??signal}`))});
 });
}
const pnpm=windows?'pnpm.cmd':'pnpm';
const output=path.join(root,'build/studio');
const bundle=path.join(output,mac?'Omni Studio.app':'omnigo-studio');
const contents=mac?path.join(bundle,'Contents'):bundle;
const binary=path.join(contents,'MacOS',windows?'omnigo-studio.exe':'omnigo-studio');
async function prepare(){await run(process.execPath,['scripts/studio-catalog.mjs']);await cp('omniweb/client/index.ts','studio/ui/src/lib/omniweb.ts')}
async function compile(dest){await mkdir(path.dirname(dest),{recursive:true});await run('go',['build','-trimpath','-o',dest,'./studio/cmd/studio'],{env:{...process.env,CGO_ENABLED:'0'}})}
async function build(){
 await prepare();await run(pnpm,['--filter','@omnigo/studio-ui','build']);
 await rm('studio/web/assets',{recursive:true,force:true});await cp('studio/ui/dist/client','studio/web',{recursive:true});
 await run('go',['run','./studio/cmd/appicon','-out',path.join(output,'AppIcon.icns'),'-preview','studio/assets/app-icon.png'],{env:{...process.env,CGO_ENABLED:'0'}});
 await rm(bundle,{recursive:true,force:true});await compile(binary);
 await mkdir(path.join(contents,'Resources'),{recursive:true});
 await licenses(path.join(contents,'Resources/licenses'));
 if(mac)await cp(path.join(output,'AppIcon.icns'),path.join(contents,'Resources/AppIcon.icns'));
 const cefRuntime=path.join(root,'build/chromium-runtime/Omni Browser.app');
 if(mac){try{await access(cefRuntime);await cp(cefRuntime,path.join(contents,'Resources/chromium/Omni Browser.app'),{recursive:true,verbatimSymlinks:true})}catch(error){if(error.code!=='ENOENT')throw error;console.log('CEF runtime not installed; run pnpm setup:chromium to include the browser.')}}
 await cp('studio/README.md',path.join(contents,'Resources/README.md'));
 await cp('studio/THIRD_PARTY_NOTICES.md',path.join(contents,'Resources/THIRD_PARTY_NOTICES.md'));
 if(mac)await writeFile(path.join(contents,'Info.plist'),`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CFBundleExecutable</key><string>omnigo-studio</string><key>CFBundleIdentifier</key><string>dev.omnigo.studio</string><key>CFBundleName</key><string>Omni Studio</string><key>CFBundleDisplayName</key><string>Omni Studio</string><key>CFBundleVersion</key><string>2</string><key>CFBundleShortVersionString</key><string>0.1.0</string><key>CFBundleIconFile</key><string>AppIcon.icns</string><key>CFBundlePackageType</key><string>APPL</string><key>NSHighResolutionCapable</key><true/></dict></plist>`);
 if(mac){
  await run('codesign',['--force','--sign','-',bundle]);
  await run('/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister',['-f',bundle]);
 }
 console.log(`\nStudio built: ${bundle}\nRun with pnpm start. Native controls currently require macOS arm64.`);
}
try{
 if(mode==='dev'){
  await build();if(!stopping)await run(binary,[]);
 }else if(mode==='build')await build();
 else if(mode==='start'){await access(binary);await run(binary,[])}
 else{
  await build();const archive=path.join(output,`omnigo-studio-${process.platform}-${process.arch}.${mac?'zip':'tar.gz'}`);
  if(mac)await run('ditto',['-c','-k','--keepParent',bundle,archive]);else await run('tar',['-czf',archive,'-C',output,path.basename(bundle)]);
  console.log(`Package: ${archive}\nLocal unsigned build; signing and notarization are not configured.`);
 }
}catch(error){stop();console.error(error.message);process.exitCode=1}
