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

Studio 的 Languages 原生 Go 演示通过下拉菜单提供英语、简体中文（zh-Hans）、繁体中文（zh-Hant）、德语、法语、日语、阿拉伯语、希伯来语、印地语切换。切换时计数状态保持，标题、按钮、选择统计和数字格式同步变化；每个窗口持有独立 Translator。中文地区标签通过 Catalog.Resolve 匹配简繁体（如 zh-CN 与 zh-TW）。阿拉伯语和希伯来语切换行布局方向，但复杂文字整形和 Unicode 双向段落排版仍未实现，演示会显示限制提示。

仍未覆盖完整 ICU/CLDR：日期/时间 skeleton、相对时间、compact number、全局系统语言跟随、强类型生成器、伪本地化、所有框架控件和原生菜单文本。RTL() 仅提供方向信息，不等于双向排版/字体回退。未支持的格式明确报错，不静默输出原串。数值限定 IEEE754 可精确表示的整数范围，避免大整数复数选择失真；高精度小数及可见末尾零语义仍待扩展。

## 目录检查

在项目根目录配置 `omnigo.json`：

```json
{"i18n":{"dir":"locales","source":"en"}}
```

运行 `omni i18n check [project-dir]`；未配置时可用 `omni i18n check --dir locales --source en [project-dir]`。配置中的目录和源语言省略时分别默认 locales、en；显式参数优先。目录必须位于项目内。添加 `--json` 输出结构化诊断，存在错误时仍输出完整报告并以非零状态退出。命令只读文件，不生成访问器或修改翻译。

`i18n.Check(fsys, dir, source)` 可供其他工具调用。它汇总 JSON/ARB 解码、重复键/语言、消息语法、未知消息键、嵌套分支中的未知参数等错误；缺失翻译和未使用源参数是警告，不影响退出状态。诊断按文件、消息和说明排序。单目录文件读取限制为每文件 8 MiB，复用运行时消息解析器的嵌套和长度预算。

当前检查范围是消息语法、键、参数名及源语言 ARB placeholder 声明类型（见 JSON 的 coverage 字段）。支持 String、int、double、num、DateTime、Object；未声明的数值用法推断为 int，其他参数推断为 String。翻译遵循源语言契约，不能通过翻译文件中的类型声明放宽约束。未使用的声明也属于契约，并产生警告；孤立元数据同样警告。尚不检查语言对应的完整基数/序数复数类别、框架消息命名空间覆盖。诊断包含一基行号：消息问题定位到消息键，元数据问题定位到 @message 声明，JSON 语法错误定位到出错位置；缺失翻译、读文件失败等没有对应源行的诊断不填写行号。DateTime 类型的普通插值可通过检查，但日期/时间 skeleton 仍未实现。检查通过不能视为这些能力或所有 ICU 格式已对齐。运行时 fallback 行为不因增加检查命令而改变。

目录检查会在 JSON 结构仍可读取时继续汇总同一文件内的非字符串值、重复键及 ARB locale 声明错误；JSON 结构损坏时停止读取该文件。重复字段报告后保留第一次出现的值及声明，避免后续重复元数据改变参数类型契约。运行时 Load 仍遇错即停止。消息值 null 与空字符串一样解码为空消息，保持参考及现有运行时行为。
