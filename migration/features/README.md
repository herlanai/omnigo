# 核心能力专项对齐

2026-11-12 重新核对 childwindow、clipboard、diff、e2e、GPU、多语言。原有导出 API 清单和 297 项平台接口都不能代替这里的内部行为与测试覆盖。参考仓库只读；`reference.json` 保存文件指纹、声明名、测试名，没有实现体。

## 现状与缺口

| 领域 | OmniGo 当前实现 | 仍需重写/接入 | 状态 |
| --- | --- | --- | --- |
| childwindow | `omniwindow/nativewindow/children.go` 共享循环、Win32 owner HWND、X11 transient-for/事件路由、隐藏联动、子树关闭 | Cocoa 公共驱动、目标系统实机验收、全套层叠/全屏/跨屏/模态行为 | partial |
| clipboard | `omniwindow/clipboard` 数据快照/延迟求值/释放，Mac 多项与 Win 单项多格式系统适配；Studio 已接公共模块 | 原生文件/DIB/TIFF、Windows 实机、X11/Wayland、原生延迟渲染与外部所有权通知 | partial |
| diff | 独立行级 LCS + 有工作量预算的 Myers，大文本稀疏修改保留独立 hunk；DiffView 还原/撤销 | Histogram、词/行内差异、紧凑化、预算外高质量算法与性能比较 | partial |
| e2e | 原有探针外新增 Mac 私有剪贴板往返、Metal 像素比对和可选 Win/Linux 父子窗生命周期探针 | 六平台实机执行、原生输入/IME/无障碍/拖放与完整画面比对 | partial |
| GPU | `omniui/baseui/gpu` 真实 Metal 设备/compute pipeline、矩形批处理/alpha 合成/裁剪/读回 | D3D/OpenGL、文本/路径/抗锯齿、纹理/效果/损伤、直呈与设备失败恢复、完整 baseui 接入 | partial |
| 多语言 | `i18n` JSON/ARB、BCP47 匹配/回退、嵌套 plural/select/ordinal、CLDR 数字/货币、语言变更通知；Studio 原生演示 | 日期/相对时间/skeleton、生成器、系统语言与全局控件/菜单文本、伪本地化、完整 RTL | partial |

`pending` 指该能力域尚无有效实现链路；`partial` 不表示列出的参考方法已经实现。控件文本复制不是系统剪贴板，多个独立窗口不是 childwindow；CPU 绘制和高 DPI 软件栅格化不是 GPU；Unicode 输入不是国际化。已有 CDP/browser 测试也不计作原生桌面 e2e。OmniBrowser 不在此次改动范围。

## 代码归属

现有实现继续使用以下路径，避免为了目录看起来完整而创建空壳：

- `omniwindow`：公共生命周期/窗口契约；`omniwindow/nativewindow`：可选 Win32/X11 驱动；Cocoa 已归入公共 `omniwindow/host`，Studio 通过根包应用 API 调用。Cocoa 与 Win32/X11 的完整窗口管理契约仍未全部统一。
- `omniui/baseui`：控件、主题、绘制与交互状态；`omniui/baseui/internal/diff`：不依赖 Surface/平台的纯比较算法。`diff_view.go` 保留 UI 状态和绘制，公开 `DiffState`/`DiffHunk` 不变。
- `runtime/dispatch`：跨 goroutine 的 UI 派发；原生资源所有权留在所属驱动线程。
- `tools/featureaudit`：只读专项扫描；`migration/features`：参考索引与实现差距；原有 `migration/reference.json`、`controls`、`platform` 清单保留。

新增模块已经落地，按以下边界继续扩展：

- 父子窗口管理归入 `omniwindow` 与各原生驱动，共享同一 UI 事件循环；不能在当前 X11 单循环旁开启互不关联的新循环冒充父子窗。
- 剪贴板数据契约与系统适配归入 `omniwindow/clipboard`，控件编辑操作继续留在 baseui；不要再往 Studio 演示宿主堆业务实现。
- 国际化纯逻辑归入独立 `i18n` 包；baseui 控件与原生菜单显式消费语言上下文，后台加载通过 dispatch 更新 UI。
- GPU 的 Go 渲染实现归入 baseui 下的可选子包；设备/原生 surface 接口由驱动提供，未导入 GPU 的应用不链接其运行依赖。
- 原生 e2e 建立独立测试入口和平台前置条件；默认跳过必须显式报告，不能计入通过的原生用例。

## 实施与验收顺序

1. 统一原生多窗口生命周期，再实现 parent/child 关系。对齐参考的父窗抬升、焦点排序、隐藏/最小化、跨屏、置顶、全屏子窗测试。
2. 剪贴板先做独立数据/所有权契约，再逐平台接入。验证格式往返、文件、错误、外部替换、持久化和关闭释放。读取/写入真实用户剪贴板的测试应显式启用，并做好数据恢复。
3. e2e 与功能同步补：记录目标系统、CPU、原生后端、前置条件、执行/跳过结果。现有 Mac 探针与 Win/Linux 编译证据分开。
4. 多语言先 catalog/回退/消息格式，再数字日期与控件文本；GPU 先资源与帧生命周期，再批次/裁剪/效果。每阶段给出已实现范围，不用接口存在宣称完成。
5. diff 继续沿独立算法层扩展；已验证 12,000 行中两个相距很远的修改得到两个独立 hunk。超过 Myers 工作/编辑距离预算仍会整块替换，没有宣称无界最小差异或性能倍数。

## 可复现检查

```sh
go run ./tools/featureaudit -source /absolute/path/to/golet
go test ./tools/featureaudit ./omniui/baseui/internal/diff ./omniui/baseui
sh scripts/check.sh
```

本次路径扫描：childwindow 3 文件/8 测试声明；clipboard 15/22；diff 6/8；e2e 115/276；GPU 242/343；i18n 65/93。分类会交叉，测试声明含 benchmark/fuzz，不能相加作为独立功能数。选择器基于文件路径，因此仍需人工检查散落在通用文件中的行为；参考测试没有自动移植或执行。

原先整理阶段没有新增原生证据。后续实现阶段已实际运行 macOS ARM64 的 `TestPrivatePasteboardRoundTrip` 和 `TestMetalNativePixels`，分别验证私有粘贴板数据与真实 GPU 输出。父子窗的 Windows/Linux 原生探针仍未在目标桌面执行。任何默认跳过的 native test 都不计为原生通过。
