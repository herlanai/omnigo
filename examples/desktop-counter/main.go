// A complete native Go application using only public OmniGo APIs.
package main

import (
	"fmt"
	"github.com/herlanai/omnigo"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"log"
)

func main() {
	err := omnigo.Run(omnigo.Options{GoWindow: func(window omnigo.WindowEnvironment) (omnigo.ContentWindowOptions, error) {
		count := 0
		return omnigo.ContentWindowOptions{Title: "OmniGo counter", Width: 480, Height: 300, View: func() *ui.Element {
			return ui.Column("root", ui.Heading("title", "OmniGo counter", 1), ui.Text("count", fmt.Sprintf("Count: %d", count)), ui.Button("add", "Add one", func() { count++ })).Padding(24).Gap(16)
		}}, nil
	}})
	if err != nil {
		log.Fatal(err)
	}
}
