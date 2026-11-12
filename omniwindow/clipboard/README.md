# 系统剪贴板与数据所有权

`New(Item{Source{Format: Text, Bytes: ...}, ...})` 创建快照；Bytes 在构造时复制，`Read` 返回新副本。`Provide` 延迟数据每个快照至多求值一次，缓存错误并捕获 panic。`Snapshot` 为一次新的系统写入建立独立缓存。多项/多格式、总数据/原生输入预算与取消均有测试。

`Open(Options{})` 打开系统剪贴板。Clipboard 属于 UI goroutine；Windows 绑定真实 owner HWND（Options.Owner），未提供则创建内部 message-only owner window。macOS 使用 NSPasteboard，Options.Name 可访问命名粘贴板。Linux 系统选择协议尚未接入，Open 明确返回 ErrUnsupported。

```go
board, err := clipboard.Open(clipboard.Options{}) // UI thread
// Handle err; defer board.Close().
err = board.WriteText(ctx, "Hello")
text, err := board.ReadText(ctx)
```

- macOS：多项数据、UTF-8 文本、HTML、PNG、Markdown 与私有 MIME 表示。
- Windows：单个 item 的多格式表示，CF_UNICODETEXT、CF_HTML 字节偏移、PNG、自有命名的 MIME 表示。自有格式包含长度封装，避免读取 GlobalAlloc padding；不向其他应用的 MIME 格式伪装该封装。多项数据明确报错。
- 所有 provider 在触碰系统所有权前完成，成功写入后系统持有物化副本，`released` 当场且仅一次调用。失败不调用 released。Flush 因已经物化而立即完成；尚非原生延迟渲染或剪贴板管理器接管协议。
- Close 不清空系统剪贴板，拒绝之后的读写并释放 adapter/owner。Windows 原生写入中途失败可能留下部分新格式，接口会报错，不声称事务回滚。
- URIList 目前只是数据表示，不等于 Finder/Explorer 原生文件列表。原生文件、DIB/TIFF 转换、X11/Wayland selection、外部所有权通知及完整拖放仍需接入。

Studio macOS 的复制/剪切/粘贴与控件复制按钮已使用这个公共模块，不再各自直接操作 NSPasteboard。

验证：数据所有权/race/错误/取消/预算/CF_HTML/PNG 测试；macOS ARM64 私有命名粘贴板的 Unicode/HTML 多项往返与释放已实机通过。测试不访问用户 general pasteboard。Windows 仅有编译/vet 证据。

```sh
OMNIGO_NATIVE_TEST=1 CGO_ENABLED=0 go test ./omniwindow/clipboard -run TestPrivatePasteboardRoundTrip -v
```
