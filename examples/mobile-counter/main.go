package main

import (
	"fmt"
	"log"

	"github.com/herlanai/omnigo"
	ui "github.com/herlanai/omnigo/omniui/baseui"
)

func main() {
	if err := omnigo.RunMobile(omnigo.MobileOptions{
		Create: func() omnigo.MobileContent {
			count := 0
			return omnigo.MobileContent{View: func() *ui.Element {
				return ui.Column("root", ui.Heading("title", "OmniGo Mobile", 1), ui.Text("count", fmt.Sprintf("Count: %d", count)), ui.Button("add", "Add one", func() { count++ }), ui.Text("status", "Go controls on a native mobile surface")).Padding(32).Gap(20)
			}}
		}, OnReady: func() { log.Print("OMNIGO_MOBILE_READY") },
	}); err != nil {
		log.Fatal(err)
	}
}
