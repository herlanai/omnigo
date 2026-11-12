import { useEffect, useState } from 'react';
import { HTTPTransport, OmniWebClient } from '@/lib/omniweb';

type Sample={at:number;processCPU:number|null;systemCPU:number|null;rss:number|null;systemUsed:number|null;systemTotal:number|null;heap:number;heapReserved:number;goReserved:number;goroutines:number;gc:number;gcPauseMS:number;errors:string[]};
type Snapshot={samples:Sample[];cores:number;platform:string;intervalMS:number};
type IconCatalog={names:string[];version:string};
type API={'studio.performance':{input:Record<string,never>;output:Snapshot};'studio.icons':{input:Record<string,never>;output:IconCatalog}};
const client=new OmniWebClient<API>(new HTTPTransport());
const percent=(n:number|null|undefined)=>n==null?'—':`${n.toFixed(1)}%`;
const bytes=(n:number|null|undefined)=>n==null?'—':n>=1024**3?`${(n/1024**3).toFixed(2)} GiB`:`${(n/1024**2).toFixed(1)} MiB`;
export function usePerformance(){
 const [data,setData]=useState<Snapshot|null>(null),[error,setError]=useState(''),[paused,setPaused]=useState(false);
 useEffect(()=>{
  if(paused)return;
  let disposed=false,timer:ReturnType<typeof setTimeout>|undefined,controller:AbortController|null=null;
  async function poll(){
   if(disposed||document.hidden)return;
   controller=new AbortController();
   try{const value=await client.call('studio.performance',{}, {signal:controller.signal,timeoutMs:5000});if(!disposed){setData(value);setError('')}}catch{if(!disposed&&!controller.signal.aborted)setError('监控连接中断，正在重试；下方保留上次采样。')}
   finally{controller=null;if(!disposed&&!document.hidden)timer=setTimeout(()=>void poll(),2000)}
  }
  const visibility=()=>{clearTimeout(timer);if(document.hidden)controller?.abort();else if(!controller)void poll()};
  document.addEventListener('visibilitychange',visibility);void poll();
  return()=>{disposed=true;clearTimeout(timer);controller?.abort();document.removeEventListener('visibilitychange',visibility)};
 },[paused]);
 return {data,error,paused,setPaused};
}
export function PerformanceSummary({monitor}:{monitor:ReturnType<typeof usePerformance>}){
 const s=monitor.data?.samples.at(-1);
 return <span className="performance-summary" title="Studio Go 主进程；CPU 以单核 100% 计，RSS 不含 WebKit 子进程">{monitor.error?'监控离线':monitor.paused?'监控已暂停':`CPU ${percent(s?.processCPU)} · RSS ${bytes(s?.rss)}`}</span>
}
function Trend({samples,field,label,format}:{samples:Sample[];field:'processCPU'|'systemCPU'|'rss'|'heap';label:string;format:(v:number|null)=>string}){
 const values=samples.map(s=>s[field]);const valid=values.filter((v):v is number=>v!=null);const high=Math.max(field.includes('CPU')?100:1,...valid);
 // Missing samples break the line instead of being shown as zero activity.
 const segments:string[]=[];let points:string[]=[];
 values.forEach((v,i)=>{if(v==null){if(points.length)segments.push(points.join(' '));points=[]}else points.push(`${i/Math.max(1,values.length-1)*600},${110-v/high*100}`)});if(points.length)segments.push(points.join(' '));
 return <article className="trend-card"><div><h3>{label}</h3><strong>{format(values.at(-1)??null)}</strong></div><svg viewBox="0 0 600 120" role="img" aria-label={`${label}，最近 ${samples.length} 次采样`} preserveAspectRatio="none"><path d="M0 110H600 M0 60H600 M0 10H600" stroke="var(--line)" fill="none"/>{segments.map((p,i)=><polyline key={i} points={p} fill="none" stroke="var(--green)" strokeWidth="2" vectorEffect="non-scaling-stroke"/>)}</svg><small>最近最多 4 分钟 · 纵轴 0 — {format(high)}</small></article>
}
export function PerformancePage({monitor}:{monitor:ReturnType<typeof usePerformance>}){
 const {data,error,paused,setPaused}=monitor,s=data?.samples.at(-1),samples=data?.samples??[];
 return <section className="insights-page"><div className="section-heading"><div><h1>性能监控</h1><p>Studio 主进程与系统实时资源 · 每 2 秒采样</p></div><button className="tool-button" onClick={()=>setPaused(!paused)}>{paused?'继续实时更新':'暂停展示更新'}</button></div>
 {error&&<p role="alert" className="error-banner">{error}</p>}
 <div className="metrics"><div><span>Studio CPU · 单核 100%</span><strong>{percent(s?.processCPU)}</strong></div><div><span>Studio 常驻内存 RSS</span><strong>{bytes(s?.rss)}</strong></div><div><span>系统 CPU · 全核平均</span><strong>{percent(s?.systemCPU)}</strong></div><div><span>系统非空闲内存</span><strong>{bytes(s?.systemUsed)}<small>/ {bytes(s?.systemTotal)}</small></strong></div></div>
 <div className="trend-grid"><Trend samples={samples} field="processCPU" label="Studio CPU" format={percent}/><Trend samples={samples} field="systemCPU" label="系统 CPU" format={percent}/><Trend samples={samples} field="rss" label="Studio 常驻内存" format={bytes}/><Trend samples={samples} field="heap" label="Go 已分配堆" format={bytes}/></div>
 <div className="runtime-panel"><h2>Go Runtime</h2><dl>{[['平台',data?.platform??'—'],['逻辑 CPU',data?.cores??'—'],['Goroutines',s?.goroutines??'—'],['已分配堆',bytes(s?.heap)],['堆保留空间',bytes(s?.heapReserved)],['Go 向系统申请空间',bytes(s?.goReserved)],['GC 次数',s?.gc??'—'],['最近 GC 暂停',s?`${s.gcPauseMS.toFixed(3)} ms`:'—'],['最近采样',s?new Date(s.at).toLocaleTimeString():'等待首次采样']].map(([k,v])=><div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}</dl></div>
 {!!s?.errors?.length&&<div role="status" className="error-banner">{s.errors.join('；')}</div>}
 <p className="insights-note">CPU 使用相邻采样差值，首次采样显示 —。Studio CPU 可超过 100%；RSS 仅含 Go 主进程与原生演示，不含 WebKit 辅助进程。系统内存为物理总量减空闲页，包含缓存，不等同于 macOS 活动监视器的“已使用内存”。暂停只冻结展示，宿主保留最近 120 次采样；隐藏页面停止轮询。</p>
 </section>
}
const iconURL=(name:string,size:number,stroke:number,dark:boolean)=>`/studio/icon.png?name=${encodeURIComponent(name)}&size=${size}&stroke=${stroke}&dark=${dark}`;
export function IconPage({dark,native,opening,notice,onOpenNative}:{dark:boolean;native:boolean;opening:boolean;notice:string;onOpenNative:()=>void}){
 const [catalog,setCatalog]=useState<IconCatalog|null>(null),[error,setError]=useState(''),[retry,setRetry]=useState(0),[query,setQuery]=useState(''),[page,setPage]=useState(0),[selected,setSelected]=useState('activity'),[size,setSize]=useState(24),[stroke,setStroke]=useState(2),[copied,setCopied]=useState('');
 useEffect(()=>{const controller=new AbortController();setError('');client.call('studio.icons',{}, {signal:controller.signal,timeoutMs:5000}).then(setCatalog).catch(()=>{if(!controller.signal.aborted)setError('图标目录加载失败。')});return()=>controller.abort()},[retry]);
 const names=(catalog?.names??[]).filter(n=>n.includes(query.trim().toLowerCase())),pages=Math.max(1,Math.ceil(names.length/72)),current=Math.min(page,pages-1);
 const code=`package main\n\nimport (\n    "github.com/herlanai/omnigo/omniui/baseui"\n    "github.com/herlanai/omnigo/omniui/icon/lucide"\n)\n\nfunc iconView() *baseui.Element {\n    return baseui.Icon(\n        "${selected}", "${selected}",\n        lucide.Must("${selected}"),\n        baseui.IconOptions{Size: ${size}, StrokeWidth: ${stroke}},\n    )\n}\n\nfunc iconButton(onClick func()) *baseui.Element {\n    return baseui.IconButton(\n        "action", "${selected}",\n        lucide.Must("${selected}"), onClick,\n    ).Variant(baseui.Outline)\n}`;
 async function copy(){try{await navigator.clipboard.writeText(code);setCopied('代码已复制')}catch{setCopied('无法访问剪贴板，请在下方代码框中选择并复制。')}}
 return <section className="insights-page"><div className="section-heading"><div><h1>Icon 图标库</h1><p>Lucide {catalog?.version??'…'} · {catalog?.names.length??'…'} 个图标（含别名）· Go 实际渲染</p></div><button className="tool-button" disabled={!native||opening} onClick={onOpenNative}>{opening?'正在打开…':'打开 Go 原生演示'}</button></div>
 <p className="insights-note">{native?'在独立原生窗口中搜索图标、调整尺寸和线宽，并查看 Go 实现代码。':'原生演示当前支持 macOS Apple Silicon。'}</p><div role="status" aria-live="polite" className="action-notice">{notice}</div>
 {error&&<div className="error-banner" role="alert">{error}<button onClick={()=>setRetry(retry+1)}>重试</button></div>}
 <div className="icon-controls"><label>搜索<input aria-label="搜索图标" placeholder="search / arrow / folder…" value={query} onChange={e=>{setQuery(e.target.value);setPage(0)}}/></label><label>尺寸 {size}px<input type="range" min="16" max="48" step="4" value={size} onChange={e=>{setSize(+e.target.value);setCopied('')}}/></label><label>线宽 {stroke}<input type="range" min="0.5" max="4" step="0.5" value={stroke} onChange={e=>{setStroke(+e.target.value);setCopied('')}}/></label></div>
 <div className="icon-layout"><div><div className="icon-grid">{names.slice(current*72,(current+1)*72).map(name=><button className={name===selected?'icon-tile selected':'icon-tile'} key={name} aria-pressed={name===selected} onClick={()=>{setSelected(name);setCopied('')}}><img src={iconURL(name,size,stroke,dark)} width={size} height={size} alt="" loading="lazy"/><span>{name}</span></button>)}</div>{!catalog&&!error&&<p role="status">正在加载图标目录…</p>}{catalog&&names.length===0&&<p role="status" className="empty-state">没有匹配的图标，请更换关键词。</p>}<div className="icon-pagination"><button className="tool-button" disabled={current===0} onClick={()=>setPage(current-1)}>上一页</button><span>{names.length} 项 · {current+1} / {pages}</span><button className="tool-button" disabled={current+1>=pages} onClick={()=>setPage(current+1)}>下一页</button></div></div>
 <aside className="icon-code"><div className="icon-large"><img src={iconURL(selected,64,stroke,dark)} width="64" height="64" alt={selected}/></div><h2>{selected}</h2><p>导入 Lucide，在 Go 控件中使用</p><button className="tool-button" onClick={()=>void copy()}>复制 Go 代码</button><span role="status" className="copy-status">{copied}</span><pre tabIndex={0} aria-label="Go 实现代码"><code>{code}</code></pre><p className="insights-note">图标来源：omniui/icon/lucide。预览 PNG 由同一 Go 栅格器生成，颜色跟随主题。选用固定名称可使用 Must；外部输入请用 Get 并处理错误。</p></aside></div>
 </section>
}
