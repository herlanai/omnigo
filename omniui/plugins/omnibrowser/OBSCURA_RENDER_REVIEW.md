# Obscura 渲染实现对照（2026-11-12）

审阅版本：`3bae60909615ed5a2ca263a12c300a4a545ed3ce`。本次只读取源码和测试，**没有运行 Obscura 二进制对 omniorbit.si 做像素对比**，因此不能声称该站在 Obscura 中完全正确，也不能把源码限制直接当成已复现的整站故障。

## 已核实的实现方式

- [Cargo.toml](https://github.com/h4ckf0r0day/obscura/blob/3bae60909615ed5a2ca263a12c300a4a545ed3ce/crates/obscura-render/Cargo.toml)：Taffy 处理布局，cosmic-text/swash 处理文字；resvg/usvg 启用 SVG text/raster-images。SVG 不是仅绘制路径的轻量预览。
- [paint.rs](https://github.com/h4ckf0r0day/obscura/blob/3bae60909615ed5a2ca263a12c300a4a545ed3ce/crates/obscura-render/src/paint.rs)：将已计算的 SVG 尺寸、字体数据库传给 usvg，再用 resvg 栅格化；支持 Web 字体解码路径。
- [dom.rs](https://github.com/h4ckf0r0day/obscura/blob/3bae60909615ed5a2ca263a12c300a4a545ed3ce/crates/obscura-render/src/dom.rs)：before/after 生成布局盒，并有 generated content/counter 处理；不是在父元素背景上画一段固定文字。
- [lib.rs](https://github.com/h4ckf0r0day/obscura/blob/3bae60909615ed5a2ca263a12c300a4a545ed3ce/crates/obscura-render/src/lib.rs) 与 [style.rs](https://github.com/h4ckf0r0day/obscura/blob/3bae60909615ed5a2ca263a12c300a4a545ed3ce/crates/obscura-render/src/style.rs)：动画具有独立时间契约和影响分类；采样与布局/绘制相分离。
- [layout_test.rs](https://github.com/h4ckf0r0day/obscura/blob/3bae60909615ed5a2ca263a12c300a4a545ed3ce/crates/obscura-render/tests/layout_test.rs)：有定位伪元素百分比 padding、空 inline-block 伪元素作为 Flex 项、计数器文字进入绘制路径的回归用例。

## 它也有明确边界

`LayoutStyle.animation_name` 注释和 shorthand parser 明确针对首个动画；不应据此宣称多动画列表完整实现。源码也明确：未支持属性构成的动画不会产生渲染效果；多栏布局目前不能分割一个子盒内部；Taffy 本身没有原生 table display，项目另行模拟表格相关布局。这些是已核实的实现限制，而不是声称其所有表格/动画都失效。

## OmniBrowser 本次独立 Go 实现

- `fit-content` 按内容测量，隐藏子项不再占 Flex gap；修复实际页头对齐。
- SVG presentation attributes 参与 CSS 级联、变量解析；简单 text 转为 Inter/Noto 字形路径，保持图形绘制顺序、组变换和 viewBox 缩放，支持 x/y、dx/dy、anchor、字距与粗体。
- before/after 在截图私有树中生成独立盒，支持字符串、CSS 十六进制转义、attr()、定位、边框和常规布局。原 Page DOM 不变。
- 显式 `ScreenshotAt(..., elapsed)` / CLI `--animation-time` 采样首个 transform 动画：duration/delay/count/direction/fill/play-state、常用 ease 和 linear、匹配结构的二维变换插值；SVG 组使用采样变换。

没有复制 Obscura 实现体，没有引入 Rust/V8/Taffy/resvg 运行时。

## 本次仍未实现

完整 tspan 独立定位/textPath/双向文字 shaping；完整生成内容 counter()/quotes、复杂伪元素选择器及嵌套；CSS 多动画合成、opacity/颜色/尺寸动画、任意 transform 矩阵分解、transition、动画事件、暂停/恢复历史、完整 transform-origin/transform-box、真实浏览器事件循环。截图动画时间不会推动 JS timer/rAF；Studio Page 预览仍为静态图，不宣称实时浏览器播放能力。
