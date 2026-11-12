# backend/internal 对齐范围

只读扫描 `/Users/halley/dev/golet/backend/internal`，当前保留 **42 个顶层目录、861 个文件、1,054 个测试/benchmark/fuzz 声明**。`reference.json` 保存文件路径、哈希和 Go 声明/测试名称，不包含实现体；`status.json` 逐目录保留已有实现和缺口。数量是参考发现结果，不是覆盖率。所有目录均保留，不能将未实现目录从分母移除。

```sh
go run ./tools/featureaudit -source /absolute/path/to/golet -internal
sh scripts/check.sh
```

本轮实际落地：

- `rope` → `text/rope`：独立实现持久化 UTF-8 文本树，字节/rune/UTF-16/行定位、替换、不可变版本、分块与行遍历；接入 `baseui.TextBuffer` 编辑、撤销、重做和快照。当前渲染仍物化字符串，未宣称完整编辑器增量绘制或参考性能等价。
- `accelerator` → `apps/accelerator`：跨平台修饰键/键名解析、格式化、XKB/XDG 表示、精确按键匹配；原生事件补独立 Ctrl/Alt/Super。全局热键注册、所有键盘布局与原生 IME 验收仍未实现。
- `download` → `runtime/download`：有界下载、SHA-256、重试、取消、临时文件清理、原子发布、跨进程缓存锁；updater 已接入。完整更新安装与 delta 不包含在这个模块中。
- Studio 增加 **Text snapshots** 与 **Keyboard shortcuts**，使用真实 Go 状态和事件。下载属于服务层，没有用假进度动画冒充真实下载演示。

基于独立事件测试验收状态变化，保留 Unicode/IME、历史、按键重复/释放、取消、资源清理及并发测试。没有执行参考测试体，没有复制参考算法实现。未修改 OmniBrowser，浏览器相关参考项仍保留，实施交给另一程序。

**当前不是 100% 对齐。** shader/shaderir、完整 GPU/text/raster/scene、各平台服务、代码生成/迁移/检查工具、更新安装、原生 e2e 等仍有大量未实现行为。每个目录必须继续补到可运行实现和行为验收；构建成功、API 存在或演示入口均不能升级为完整覆盖。
