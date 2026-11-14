package scaffold

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"html"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"github.com/herlanai/omnigo/internal/appbuild"
	"github.com/herlanai/omnigo/internal/mobilebuild"
)

//go:embed all:desktop
var desktop embed.FS

func validateIdentity(name, id string) error {
	if strings.TrimSpace(name) != name || name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\:") || strings.ContainsFunc(name, unicode.IsControl) {
		return errors.New("scaffold: invalid application display name")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$`).MatchString(id) {
		return errors.New("scaffold: invalid bundle identifier")
	}
	return nil
}

type projectWriter func(string, []byte) error

func writeConfig(write projectWriter, kind, name, id string) error {
	c := appbuild.Config{Toolchain: "go", Package: "./app", Output: "build/app", GOOS: "darwin", GOARCH: "arm64", Dev: &appbuild.DevOptions{Watch: []string{"app", "go.mod", "go.sum"}}}
	if kind == "mobile" {
		c = appbuild.Config{Toolchain: "go", Mobile: &mobilebuild.Config{Package: "./app", Output: "build/mobile", Name: name, ID: id, Version: "0.1.0"}}
	} else if kind == "browser" {
		c.Toolchain, c.GOOS, c.GOARCH = "web", runtime.GOOS, runtime.GOARCH
		if c.GOOS == "windows" {
			c.Output += ".exe"
		}
		c.Before = [][]string{{"npm", "--prefix", "ui", "run", "build"}}
		c.Dev.Watch = append(c.Dev.Watch, "ui/src", "ui/package.json", "ui/package-lock.json", "ui/vite.config.ts")
	} else {
		entries, err := fs.ReadDir(desktop, "desktop/licenses")
		if err != nil {
			return err
		}
		licenses := []string{"THIRD_PARTY_NOTICES.md"}
		for _, e := range entries {
			licenses = append(licenses, "licenses/"+e.Name())
		}
		c.Bundle = &appbuild.BundleConfig{Name: name, ID: id, Version: "0.1.0", Icon: "app/assets/AppIcon.icns", Licenses: licenses, Destination: "releases/0.1.0"}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return write("omnigo.json", append(data, '\n'))
}

func writeDesktop(write projectWriter, kind, name string) error {
	source := goMain
	if kind == "webview" {
		source = webMain
	}
	source = strings.ReplaceAll(source, "APP_NAME", strconv.Quote(name))
	formatted, err := format.Source([]byte(source))
	if err != nil {
		return err
	}
	if err = write("app/main.go", formatted); err != nil {
		return err
	}
	if kind == "webview" {
		page := strings.ReplaceAll(webPage, "APP_NAME", html.EscapeString(name))
		if err = write("app/web/index.html", []byte(page)); err != nil {
			return err
		}
	}
	if err = fs.WalkDir(desktop, "desktop", func(path string, d fs.DirEntry, err error) error {
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
		return write(strings.TrimPrefix(path, "desktop/"), b)
	}); err != nil {
		return err
	}
	if err = writeIcon(write, name); err != nil {
		return err
	}
	readme := fmt.Sprintf(`# %s

Template: %s. Requires Go 1.26+, a local OmniGo checkout and the omni CLI.
The generated desktop target is macOS arm64. Node.js is not required.

Install the CLI from the framework checkout: go install ./cmd/omni
Then run these commands in this project:

`+"```sh"+`
go mod tidy
omni doctor
omni dev
# After exiting development:
omni build
omni package
`+"```"+`

All commands read omnigo.json. Development rebuilds/restarts the app; it is not HMR.
Packaging creates releases/0.1.0/%s.app and its DMG with an Applications shortcut.
Choose a new bundle.destination for subsequent packages. Signing is ad-hoc, not notarized.
Copy the app out of the checkout before checking launch, icon, input and window behavior.
No Go toolchain, Node or source checkout is needed to run the packaged app.

Replace the generated starter icon in app/assets (PNG for runtime, ICNS for Finder).
Edit the product name and bundle identity in both app/main.go and omnigo.json when renaming.
Framework code stays in the dependency; business logic belongs in this project.
Review THIRD_PARTY_NOTICES.md and licenses when adding dependencies.

`, name, kind, name)
	if kind == "webview" {
		readme += "Edit app/web/index.html for the embedded static workspace. The example has local browser interactions; it does not implement an RPC protocol. For Go service calls, use the canonical OmniWebClient/HTTPTransport from omniweb/client/index.ts with a frontend build, and set toolchain to web in omnigo.json if Node is required.\n"
	}
	return write("README.md", []byte(readme))
}

const goMain = `package main
import (
 _ "embed"
 "fmt"
 "log"
 "github.com/herlanai/omnigo"
 ui "github.com/herlanai/omnigo/omniui/baseui"
)
//go:embed assets/icon.png
var icon []byte
func main() {
 err := omnigo.Run(omnigo.Options{Name:APP_NAME, Icon:icon, GoWindow:func(env omnigo.WindowEnvironment)(omnigo.ContentWindowOptions,error){
 count:=0
 return omnigo.ContentWindowOptions{Title:APP_NAME,Width:800,Height:560,View:func()*ui.Element{
 return ui.Column("root",ui.Heading("title",APP_NAME,1),ui.Text("count",fmt.Sprintf("Count: %d",count)),ui.Button("add","Add one",func(){count++})).Padding(24).Gap(16)
 }},nil
 }})
 if err!=nil {log.Fatal(err)}
}`
const webMain = `package main
import (
 "embed"
 "io/fs"
 "log"
 "github.com/herlanai/omnigo"
)
//go:embed all:web
var assets embed.FS
//go:embed assets/icon.png
var icon []byte
func main(){
 web,err:=fs.Sub(assets,"web");if err!=nil{log.Fatal(err)}
 err=omnigo.Run(omnigo.Options{Name:APP_NAME,Icon:icon,Assets:web,Window:&omnigo.WindowOptions{Title:APP_NAME,URL:"/",Width:1000,Height:700,MinWidth:640,MinHeight:480}})
 if err!=nil{log.Fatal(err)}
}`
const webPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>APP_NAME</title>
<style>
:root{color-scheme:light dark;font:15px system-ui}*{box-sizing:border-box}body{margin:0;height:100vh;height:100dvh;display:flex;flex-direction:column;background:Canvas;color:CanvasText}header{height:48px;flex-shrink:0;padding:12px 20px;border-bottom:1px solid GrayText;font-weight:600}main{flex:1;min-height:0;overflow:auto;padding:24px}label{display:block;margin-bottom:12px}textarea{font:inherit;display:block;width:100%;height:55vh;padding:12px;border:1px solid GrayText;border-radius:6px;resize:vertical}button{font:inherit;padding:8px 16px;margin-top:12px}button:focus-visible,textarea:focus-visible{outline:2px solid Highlight;outline-offset:3px}#status{margin-left:12px}
</style></head><body><header>APP_NAME</header><main><label for="note">Scratchpad — notes stay in this window until it closes</label><textarea id="note" placeholder="Start writing…"></textarea><button id="clear">Clear note</button><span id="status" role="status"></span></main>
<script>const note=document.getElementById('note');document.getElementById('clear').addEventListener('click',()=>{note.value='';note.focus();document.getElementById('status').textContent='Note cleared';});</script></body></html>`

func writeIcon(write projectWriter, name string) error {
	var err error
	// A deterministic, application-name-specific starter icon; no Studio artwork.
	sum := sha256.Sum256([]byte(name))
	img := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	for y := 0; y < 1024; y++ {
		for x := 0; x < 1024; x++ {
			c := color.NRGBA{sum[0]/2 + 40, sum[1]/2 + 40, sum[2]/2 + 40, 255}
			if x > 220 && x < 804 && y > 220 && y < 804 && (x < 320 || x > 704 || y < 320 || y > 704) {
				c = color.NRGBA{245, 245, 250, 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	var pngData bytes.Buffer
	if err = png.Encode(&pngData, img); err != nil {
		return err
	}
	if err = write("app/assets/icon.png", pngData.Bytes()); err != nil {
		return err
	}
	var icns bytes.Buffer
	icns.WriteString("icns")
	_ = binary.Write(&icns, binary.BigEndian, uint32(16+pngData.Len()))
	icns.WriteString("ic10")
	_ = binary.Write(&icns, binary.BigEndian, uint32(8+pngData.Len()))
	icns.Write(pngData.Bytes())
	if err = write("app/assets/AppIcon.icns", icns.Bytes()); err != nil {
		return err
	}
	return nil
}
