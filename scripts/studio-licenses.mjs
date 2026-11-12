import { readdir, mkdir, cp } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import path from 'node:path';
export async function licenses(destination){
 await mkdir(destination,{recursive:true});
 async function copyLicenses(directory,label){
  const files=await readdir(directory,{withFileTypes:true});
  for(const file of files)if(file.isFile()&&/^(licen[sc]e|copying|notice|ofl)(\.|$|-)/i.test(file.name))await cp(path.join(directory,file.name),path.join(destination,`${label.replaceAll('/','_')}-${file.name}`));
 }
 await copyLicenses('omniui/plugins/chromium','golet-chromium-port');
 await copyLicenses('omniui/icon/lucide','lucide-static-1.71.0');
 const modules=execFileSync('go',['list','-m','-f','{{.Path}}|{{.Dir}}','all'],{encoding:'utf8'}).trim().split('\n').slice(1);
 for(const line of modules){const [name,directory]=line.split('|');if(directory){await copyLicenses(directory,name);if(name==='golang.org/x/image')await copyLicenses(path.join(directory,'font/gofont'),'go-fonts')}}
 const goRoot=execFileSync('go',['env','GOROOT'],{encoding:'utf8'}).trim();await copyLicenses(goRoot,'go');
 for(const entry of await readdir('node_modules/.pnpm',{withFileTypes:true})){
  if(!entry.isDirectory()||entry.name==='node_modules')continue;
  const base=path.join('node_modules/.pnpm',entry.name,'node_modules');
  let packages;try{packages=await readdir(base,{withFileTypes:true})}catch{continue}
  for(const pkg of packages){if(!pkg.isDirectory())continue;
   if(pkg.name.startsWith('@')){for(const scoped of await readdir(path.join(base,pkg.name),{withFileTypes:true})){if(scoped.isDirectory())await copyLicenses(path.join(base,pkg.name,scoped.name),`${entry.name}-${scoped.name}`)}}
   else await copyLicenses(path.join(base,pkg.name),entry.name);
  }
 }
}
