package scaffold

import (
	"embed"
	"fmt"
	"go/format"
	"html"
	"io/fs"
	"strconv"
	"strings"
)

//go:embed mobile/LICENSE.mobile
var mobileFiles embed.FS

func writeMobile(write projectWriter, name, id string) error {
	source := strings.ReplaceAll(mobileMain, "APP_NAME", strconv.Quote(name))
	b, err := format.Source([]byte(source))
	if err != nil {
		return err
	}
	if err = write("app/main.go", b); err != nil {
		return err
	}
	if err = writeIcon(write, name); err != nil {
		return err
	}
	manifest := fmt.Sprintf(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="%s" android:versionCode="1" android:versionName="0.1.0">
<application android:label="%s" android:debuggable="true">
<activity android:name="org.golang.app.GoNativeActivity" android:label="%s" android:exported="true" android:configChanges="orientation|keyboardHidden|screenSize">
<meta-data android:name="android.app.lib_name" android:value="mobile"/>
<intent-filter><action android:name="android.intent.action.MAIN"/><category android:name="android.intent.category.LAUNCHER"/></intent-filter>
</activity></application></manifest>`, id, html.EscapeString(name), html.EscapeString(name))
	if err = write("app/AndroidManifest.xml", []byte(manifest)); err != nil {
		return err
	}
	if err = fs.WalkDir(desktop, "desktop/licenses", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := desktop.ReadFile(path)
		if err != nil {
			return err
		}
		return write("app/assets/"+strings.TrimPrefix(path, "desktop/"), b)
	}); err != nil {
		return err
	}
	license, _ := mobileFiles.ReadFile("mobile/LICENSE.mobile")
	if err = write("app/assets/licenses/x-mobile-LICENSE.txt", license); err != nil {
		return err
	}
	return write("README.md", []byte(`# Mobile Go application

Experimental iOS/Android Go controls, native lifecycle/touch and CPU RGBA presentation.
Requires Go 1.26+. Mobile native ABI/linking uses cgo; desktop builds do not.
Install the framework CLI from its checkout: go install ./cmd/omni

From this project:

    go mod tidy
    omni mobile doctor --target ios-simulator
    omni mobile build --target ios-simulator
    omni mobile build --target ios
    omni mobile build --target android

Targets go to build/mobile/<target>-arm64; choose a new mobile.output before rebuilding.
iOS needs macOS/Xcode. Simulator bundles are ad-hoc signed; device bundles are unsigned
and require your provisioning/signing/export workflow. Android needs the official NDK,
Java, SDK platform 35 / Build Tools 35.0.0, gomobile and gomobile init; the generated APK is development-signed, not store-ready.
Install gomobile at the version pinned in the framework docs/mobile.md. The SDK tools
set minSdk 23 / targetSdk 35, align native libraries and verify the development signature.
Keep the generated .omni/android-debug.p12 private and stable for development updates.
iOS uses the bundle ID as the .app filename; it applies UIKit scale and safe-area insets.

Edit app/main.go for Go controls. Keep state in Create, not in View. UI callbacks must
not block. Android app identity/version is also declared in app/AndroidManifest.xml;
build rejects mismatches with omnigo.json. Replace the starter app/assets/icon.png.

This first mobile driver does not yet provide software keyboard/IME, system accessibility,
complete Android edge-to-edge support, gesture scrolling, WebView mode, background services or desktop
window APIs. Use buttons and pointer-driven controls; editable controls are not mobile-ready.
Native App Store/Play Store release and physical-device validation remain separate work.
`))
}

const mobileMain = `package main
import("fmt";"log";"github.com/herlanai/omnigo";ui "github.com/herlanai/omnigo/omniui/baseui")
func main(){
 err:=omnigo.RunMobile(omnigo.MobileOptions{Create:func()omnigo.MobileContent{
 count:=0
 return omnigo.MobileContent{View:func()*ui.Element{
 return ui.Column("root",ui.Heading("title",APP_NAME,1),ui.Text("count",fmt.Sprintf("Count: %d",count)),ui.Button("add","Add one",func(){count++})).Padding(32).Gap(20)
 }}
 },OnReady:func(){log.Print("OMNIGO_MOBILE_READY")}})
 if err!=nil{log.Fatal(err)}
}`
