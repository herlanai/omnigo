import { readFile, writeFile } from 'node:fs/promises';
const read = async p => JSON.parse(await readFile(new URL('../'+p,import.meta.url),'utf8'));
const reference = await read('migration/controls/reference.json');
const status = await read('migration/controls/status.json');
const baseline = await read('migration/reference.json');
const plugins = await read('omniui/plugins/status.json');
const advanced = await read('migration/controls/advanced.json');
const entries = reference.catalog.map(c=>({...c,...status.controls.find(s=>s.id===c.id),kind:'control'}));
for (const entry of advanced.controls) entries.push({...entry,group:'advanced',kind:'control'});
for(const id of baseline.plugins) {
 const plugin = plugins.plugins.find(p=>p.id===id);
 if (!plugin) throw new Error(`Missing plugin audit: ${id}`);
 entries.push({...plugin,id:`plugin/${id}`,group:'plugins',kind:'plugin',description:`已实现：${plugin.implemented.join('；')}。尚缺：${plugin.remaining.join('；')}`});
}

entries.push({id:'plugin/omnibrowser',title:'OmniBrowser',group:'plugins',kind:'plugin',status:'partial',implementation:'omnibrowser',symbol:'Page',description:'参考 Obscura 的独立纯 Go 文档引擎：HTTP、Cookie、DOM 查询、JavaScript、交互、语义 HTML 截图与 CDP 子集。完整 CSS/Web API、Puppeteer/Playwright、隐身模式和 PDF 尚未实现。'});

for(const [id,title,description] of [
 ['lifecycle','Application lifecycle','启动、取消、依赖排序与失败回滚。'],
 ['dispatch','UI dispatch','有界主线程队列、取消与背压。'],
 ['event','Event topics','有界订阅、丢弃计数与资源释放。'],
 ['rpc','Typed RPC','Go 类型绑定、输入校验、并发限制。'],
 ['omniweb','OmniWeb','当前 Studio 的目录与原生窗口调用通过 OmniWeb 运行。'],
 ['native','Native host','macOS Apple Silicon 实验窗口；系统 IME、无障碍、GPU 与其他平台尚待实现。'],
 ['theme','Theme tokens','Go 控件共用明暗主题；切换主题后打开对应演示。'],
 ['virtual','Virtual rows','Table 演示仅物化当前行窗口，不代表已实现完整滚动。'],
]) entries.push({id:`system/${id}`,title,description,group:'framework',kind:'feature',status:'partial'});
await writeFile(new URL('../studio/internal/catalog/catalog.json',import.meta.url),JSON.stringify(entries,null,2)+'\n');
