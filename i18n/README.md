# Go 多语言运行时

不可变 Catalog 可在窗口间共享；Translator 属于一个 UI goroutine。后台请求语言变化时使用 `internal/runtime/dispatch` 或窗口 Post 回到 UI。

```go
catalog, err := i18n.Load(os.DirFS("locales"), ".", "en")
// Handle err.
translator, err := catalog.Translator("de-DE")
text, err := translator.Text("files", map[string]any{"count": 3})
unsubscribe := translator.Subscribe(func() { /* remount UI */ })
defer unsubscribe()
err = translator.SetLanguage("fr")
```

目录包含 `en.json` / `de.json` 等 BCP47 文件名，也支持 `.arb` 和 `@@locale` / `@message` 元数据。加载时一次性解析、验证所有消息，拒绝重复 key/归一化语言名、非法格式、缺少 other 的分支和超出预算的输入。原始 map/文件内容不会在加载后影响 Catalog。

已实现：语言匹配与消息级父语言/源语言回退、参数替换、嵌套 select/plural/selectordinal、精确数值分支、offset、复数 #、ICU apostrophe 引用、CLDR cardinal/ordinal 规则，以及本地化 decimal/integer/percent、`.00`、group-off、ISO 货币格式。语言/复数/数字/货币数据使用项目已有 `golang.org/x/text`，不复制 golet 数据或实现。

`{n, plural, =0 {No files} one {# file} other {# files}}`

Studio 的 Languages 原生 Go 演示提供英语、德语、法语切换，切换时计数状态保持，按钮和数字格式同步变化。

仍未覆盖完整 ICU/CLDR：日期/时间 skeleton、相对时间、compact number、全局系统语言跟随、强类型生成器、伪本地化、所有框架控件和原生菜单文本。RTL() 仅提供方向信息，不等于双向排版/字体回退。未支持的格式明确报错，不静默输出原串。数值限定 IEEE754 可精确表示的整数范围，避免大整数复数选择失真；高精度小数及可见末尾零语义仍待扩展。
