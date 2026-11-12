# 性能与健壮性验收

目前没有与参考框架的同工作负载对比，不能声称 100 倍提升。

## 基准范围

当前基准只测算法核心：

baseui 新增 `BenchmarkMount100Controls`，测 100 控件的校验、快照与行列布局，不含窗口、GPU 或像素绘制。Go CPU 画廊是功能/视觉验证，不是性能对照。

- `BenchmarkDiffUnchanged`：复用同一个不可变树，跳过更新。
- `BenchmarkDiffOneOf10000`：10,000 个兄弟节点中更新一个属性；仍需扫描兄弟列表，不是 O(1)。不包含新树创建、布局或绘制。
- `BenchmarkVirtualMillionRows`：计算百万行固定高度列表的可见范围，不含行组件创建或渲染。
- `BenchmarkPublish`：16 个已经满的订阅队列，测丢弃路径；不能拿它与同步调用 16 个回调的基准直接比较。

```sh
go test -run '^$' -bench . -benchmem -count 5 ./omniui ./runtime/event
```

基准必须说明 CPU、OS、Go 版本、构建模式、样本数、工作负载、分配量与测量边界。不可用竞态检测版本的数据当发布性能。

## 100 倍目标的后续验证

| 目标 | 必须测量的内容 |
| --- | --- |
| 运行性能 | 同一应用的启动、空闲 CPU、RSS、输入到显示 P50/P95/P99、帧时间、吞吐与分配 |
| 界面构建速度 | 分别测 UI 树构建、布局、绘制与项目冷/热编译；不能混为一个指标 |
| 大数据界面 | 固定数据集下的列表、表格、编辑器、图表与流式更新；验证结果等价 |
| 健壮性 | 崩溃、资源泄漏、竞态、死锁、故障恢复与取消传播；不是单一倍率 |
| 开发效率 | 固定任务的完成时间、配置步骤、生成物正确率与可维护性；不能由代码行数推断 |
| 兼容性 | 各 OS/架构、DPI、IME、无障碍、GPU/软件路径与安装升级实测 |

当前测试使用确定性同步验证依赖失败、清理、异常、背压、取消和更新协议，另有 RPC 与树差分的模糊测试。它们是建立质量门槛的起点，不是整套桌面框架稳定性的证明。

## 2026-11-11：CPU 光栅化与原生窗口帧缓冲

环境：Apple M2 Max、darwin/arm64、Go 1.26.0，`CGO_ENABLED=0`，非 race 构建，每组 5 个样本，下表为中位数。基准只与本仓库优化前实现比较，不是与 golet 的对照。

CPU profile 在固定控件负载下显示 `Painter.rasterShade` 累计占采样的 91.19%。本次将循环边界和纵向剪裁计算移出像素内循环，避免为完全不可见像素计算颜色，给完全不透明像素直接赋值，并在圆角之外使用轴向距离代替 hypot。抗锯齿、半透明混色和分数剪裁规则保持一致。

负载：640×480 DIP，8 行，每行 Label / Button / Badge，外层 Card。布局在计时之前完成，只测绘制；原有 RenderScaled 仍返回独立快照。

| 测量 | 优化前 | 优化后 | 说明 |
| --- | ---: | ---: | --- |
| 1× 渲染 | 10.735 ms | 6.359 ms | 耗时降低约 40.8% |
| 2× 渲染 | 42.106 ms | 24.397 ms | 耗时降低约 42.1% |
| 2× 每帧分配，独立快照 | 4,950,218 B | 4,950,215 B | 保留快照所有权，分配基本不变 |
| 2× 每帧分配，调用方复用帧 | 4,950,218 B | 34,948 B | 新 RenderScaledInto 路径，降低约 99.3% |
| Mount 100 个文本控件 | 63.247 µs / 81,946 B | 62.655 µs / 81,946 B | 未优化此路径，不将微小波动视为提升 |

Studio 原生窗口使用共享的单个 RGBA 帧、PNG 输出 buffer 和 PNG encoder buffer；多个窗口不会各自保留一套 Go scratch buffer。DPI/尺寸变化时替换帧，最后一个演示窗口关闭时释放全部 scratch 引用。NSData 同步复制编码数据后才允许复用，不改变 Render/RenderScaled 的独立图像所有权。

另一个渲染加 PNG 编码负载（640×480 DIP、2×、Card / Heading / Button）在同一优化后绘制实现上比较临时分配与复用：每帧分配中位数 **5,828,373 B → 20,012 B**，分配次数 **68 → 35**；耗时 **37.018 ms → 37.115 ms**，没有可证明的编码性能提升。这个比较隔离了 buffer 复用的影响，不是完整优化前后的帧管线耗时对比。

复用会保留一帧和编码工作区以减少分配与 GC 压力，**每帧分配降低不等于进程 RSS 同比例降低**。本批没有测量 AppKit 上传/显示、应用启动、空闲 CPU、长期 RSS 或端到端输入延迟，仍需原生设备场景验证。绘制按需运行策略不变。

验证：确定性随机几何逐像素对照旧光栅器，覆盖 1/1.25/2/3×、透明度、圆角边框和分数剪裁；验证帧重用清屏、快照独立、无效目标拒绝与关闭释放；按钮、卡片、输入框、树形表格的 8 张 2× 明暗 PNG 与优化前字节一致。

复现与原始记录：

```sh
CGO_ENABLED=0 go test ./omniui/baseui -run '^$' -bench 'BenchmarkControlsRender|BenchmarkMount100Controls' -benchmem -count 5
CGO_ENABLED=0 go test ./apps/host -run '^$' -bench BenchmarkFramePipeline -benchmem -count 5
```

- [优化前](benchmarks/render-2026-11-11-before.txt)
- [优化后及复用路径](benchmarks/render-2026-11-11-after.txt)
- [原生帧 PNG 管线](benchmarks/frame-pipeline-2026-11-11.txt)

## 虚拟列表新增实现的规模检查

`BenchmarkVirtualListMount` 包含 VirtualList 构建与 Surface.Mount，640×480 Surface、240 DIP 视口、48 DIP 行高、overscan=2，每个可见项包含选择按钮与文本。同一机器 Go 1.26.0 / Apple M2 Max / darwin arm64、CGO_ENABLED=0，3 次采样中位数：1,000 项 **70.766 µs / 72,843 B / 300 allocs**；1,000,000 项 **71.057 µs / 72,843 B / 300 allocs**。这说明该固定视口没有随总行数构建全部元素；不代表真实数据获取、可变行高或像素绘制性能。原始输出见 [虚拟列表记录](benchmarks/virtual-list-2026-11-11.txt)。
