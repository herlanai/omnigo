// A native Go chat panel backed by the omnix agent runtime.
package main

import (
	"log"
	"os"

	"github.com/herlanai/omnigo"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	dshgo "github.com/herlanai/omnigo/omniui/plugins/dsh-go"
)

func main() {
	err := omnigo.Run(omnigo.Options{GoWindow: func(env omnigo.WindowEnvironment) (omnigo.ContentWindowOptions, error) {
		panel, err := dshgo.NewPanel(dshgo.PanelOptions{RunTask: env.RunTask, Agent: dshgo.Options{DeepSeek: dshgo.DeepSeekOptions{APIKey: os.Getenv("DEEPSEEK_API_KEY")}}})
		if err != nil {
			return omnigo.ContentWindowOptions{}, err
		}
		return omnigo.ContentWindowOptions{Title: "Assistant", Width: 900, Height: 780, TitleBarHidden: func() bool { return true }, DragRegion: "assistant/title", View: func() *ui.Element { return panel.View("assistant") }, Close: func() { _ = panel.Close() }}, nil
	}})
	if err != nil {
		log.Fatal(err)
	}
}
