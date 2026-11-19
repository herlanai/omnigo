# iOS 与 Android：实验性 Go 界面构建

本轮先打通 Go 控件移动应用。**移动 WebView 尚未实现**；桌面的 WebView 模板不能
直接当作移动模板。当前平台/验收状态统一见 [status](status.md)。

## 外部原生 view 方案的参考结论

核对 2026-11-14 的公开原生 UI 框架构建实现（仅核对结论，未复制源码，也未作为依赖引入）：

- iOS 构建需要 `CGO_ENABLED=1`，使用 Xcode 的 clang/linker。
- Android 构建中 arm64 为 `CGO_ENABLED=0`，amd64 为 `CGO_ENABLED=1` 并选择 NDK clang。
- 该类方案采用每项平台各自的原生 view 挂载（含 Android 子进程方案），与下述 OmniGo 的选择不同。

OmniGo 选择复用 baseui 的 Go 布局、交互和 CPU RGBA 绘制，使用官方
`golang.org/x/mobile` 接入移动生命周期/触摸，并将像素提交到原生 GL 表面。
这是与原生 view 挂载不同的技术路线。移动宿主的 ABI 桥接和链接使用 cgo，
控件业务仍为 Go；桌面构建继续 `CGO_ENABLED=0`，不会链接 x/mobile/app 或 GL 驱动。
移动依赖仍属于同一 Go module，因此 go mod tidy 会解析其依赖图。

## 创建项目

从框架根目录：

```sh
go install ./cmd/omni
omni init --template mobile --framework "$PWD" --module example.com/mobile \
  --name "My Mobile App" --id com.example.mobile /tmp/my-mobile
cd /tmp/my-mobile
go mod tidy
```

公开入口 `omnigo.RunMobile(omnigo.MobileOptions{Create: ...})`，`Create` 返回
`omnigo.MobileContent{View: ...}`。状态在 Create 中初始化，View 根据状态返回
baseui 树；触摸、View、OnReady、OnError、Close 都在同一 UI 事件 goroutine。
回调不能阻塞。当前没有移动 RunTask/系统服务入口，不能从 worker 直接修改 UI 状态。

根包的桌面组合实现按 `!ios && !android` 隔离。桌面的 `Run`、菜单、标题栏和窗口
服务不是移动平台 API。不要把桌面入口原样放进手机应用。

## iOS

需要 macOS、完整 Xcode 和目标 SDK。模拟器运行还需对应的 Simulator runtime。

```sh
omni mobile doctor --target ios-simulator
omni mobile build --target ios-simulator
omni mobile build --target ios
```

### dev 监听循环

`omni mobile dev` 在启动中的 iOS Simulator 或运行中的 Android 模拟器/设备上
重建、重装并重启应用，然后监听项目源码变化并重复该循环：

```sh
omni mobile dev --target ios-simulator
omni mobile dev --target android          # 需要 adb 可见至少一台设备/模拟器
```

- iOS Simulator：自动选择已启动的模拟器；没有则启动列表中第一台可用设备并等待
  `simctl bootstatus`。
- Android：使用 `adb devices` 中第一台 `device` 状态的设备；启动组件读取
  app/AndroidManifest.xml 的 activity `android:name`。
- **这是完整重建循环，不是热代码替换**：每次变更都重新编译、重装并重启，
  应用内存状态丢失。iOS 真机目标被拒绝——安装需要应用自备签名。
- 输出目录跳过 `mobile.output`，每次迭代构建到独立临时目录，不影响 `mobile build` 产物。

2026-11-19 在本机实测（Apple M2 Max、Xcode iPhoneSimulator 27.0 SDK、iOS 17.0
模拟器、脚手架 mobile 模板）：模拟器未启动时自动选择并启动第一台可用设备；
首次安装启动 13.8s；修改 `app/main.go` 后检测-重建-重装-重启全程 1.8s，应用以
新进程重启。Android 路径（adb/emulator）本机无工具链，未实测；单元测试覆盖
设备解析、activity 解析与监听循环（注入假构建与假设备）。

默认 arm64；Intel 模拟器可指定 `--arch amd64`。产物位于：

- `build/mobile/ios-simulator-arm64/com.example.mobile.app`：ad-hoc 签名的模拟器包。
- `build/mobile/ios-arm64/com.example.mobile.app`：未签名真机包，需要应用自己的
  provisioning、证书与导出流程；没有自动导出商店 IPA。

bundle 文件名使用 ID，显示名称仍是 name，避免 Go iOS runtime 对含空格的 bundle
URL 工作目录处理不正确。图标与许可证来自 app/assets，资源随 bundle 携带。
iOS 宿主将 UIKit 的逻辑 scale 与安全区换算到 Go 布局/触摸坐标；不能使用物理
72-dpi 点直接当成 UIKit point。首帧 OnReady 发生在原生提交之后。

```sh
xcrun simctl list devices available
xcrun simctl bootstatus DEVICE_UDID -b
xcrun simctl install DEVICE_UDID build/mobile/ios-simulator-arm64/com.example.mobile.app
xcrun simctl launch --console DEVICE_UDID com.example.mobile
```

## Android

需要 Java（建议 JDK 21）、Android SDK 与 NDK。设置 ANDROID_HOME，必要时用
ANDROID_NDK_HOME 指定 NDK；Java 和 keytool 必须能从 PATH 找到。
本轮实际构建使用 NDK r30，API 35、Build Tools 35.0.0：

```sh
sdkmanager 'platforms;android-35' 'build-tools;35.0.0' 'ndk;30.0.16248370'
go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20261011034528-197770444867
gomobile init
omni mobile doctor --target android
omni mobile build --target android
# x86_64 模拟器包：
omni mobile build --target android --arch amd64
```

产物为 `build/mobile/android-arm64/My Mobile App.apk`；amd64 独立输出到
`android-amd64`。**这是开发包，不是 Play Store 发布包/AAB。**

工具先用固定 gomobile 构建原生库与宿主，再用官方 aapt2 生成现代清单/资源：
minSdk 23、targetSdk 35。不能使用 gomobile 默认旧 SDK 声明作为现代 Android 支持证据。
随后 zipalign 进行 16 KiB native 库文件对齐，apksigner 签名并验证。
这并不替代真机 16 KiB 页运行测试或当前商店政策审核。

开发签名使用项目 `.omni/android-debug.p12`，权限 0600，由首次构建生成并在后续
构建复用；`.omni/` 已加入生成项目的 gitignore。密码是 Android debug 约定值，
此密钥只能用于开发。构建不会选择或读取生产签名身份，也不会把开发私钥放进 APK。
保留同一开发密钥才能覆盖安装；不要提交它。

`app/AndroidManifest.xml` 与配置中的 ID/version 必须一致。该清单供 gomobile
消费，**不要手写 uses-sdk 或 android:icon**；这两项由后续 SDK 资源处理生成。
权限与其余清单属性会保留。模板不请求网络等额外权限。

## 当前边界与验收

已实现：Go 控件、CPU 绘制/GL 原生提交、单主指触摸、前后台取消当前交互、
尺寸变化保留状态、GPU 上下文失效后重建纹理、首帧通知、窗口关闭清理。
iOS 已接入逻辑坐标与安全区；Android 使用 dp 换算，但未做完整 edge-to-edge 验收。

尚未实现/验收：软件键盘与完整 IME、系统读屏桥接、触摸取消的全平台系统事件、
多指/惯性滚动、移动后台服务、移动 WebView、移动原生 RPC、商店发布与真机矩阵。
不要把可编辑控件视为已支持手机编辑；参考框架的控件对齐仍维持 partial。
x/mobile 使用的 iOS GLKit/OpenGLES 和部分 UIKit API 已被标记 deprecated；
当前原生启动可用不等于长期渲染后端规划已经完成。

`omni mobile build` 只发布新的目标目录，拒绝覆盖已有产物。重复构建请更换
mobile.output 或由应用管理旧构建；不自动删除应用或源码。doctor 只检查工具位置，
不是编译、签名或原生运行验收。

`.github/workflows/mobile.yml` 配置了独立项目构建和原生首帧探针；
`go run ./tools/cmd/nativeprobe ios` / `android` 缺少首帧标记就失败并保留日志/截图。
远程工作流尚未运行，不能宣称 CI 已通过。实际本地证据见 [validation](validation.md)。
