import {spawnSync} from 'node:child_process';
const args=['run','./omniui/plugins/chromium/cmd/setup',...process.argv.slice(2)];
if(process.env.CEF_ROOT&&!args.includes('--cef-dir'))args.push('--cef-dir',process.env.CEF_ROOT);
if(!args.includes('--output'))args.push('--output','build/chromium-runtime');
const r=spawnSync('go',args,{stdio:'inherit',env:{...process.env,CGO_ENABLED:'0'}});
if(r.error)throw r.error;process.exitCode=r.status??1;
