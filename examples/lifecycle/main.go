// This is a headless contract demonstration, not a apps window.
package main

import (
	"context"
	"fmt"
	"github.com/herlanai/omnigo/runtime/lifecycle"
	"log"

	"github.com/herlanai/omnigo/omniui"
	"github.com/herlanai/omnigo/runtime/rpc"
)

type demo struct{}

func (demo) Run(ctx context.Context) error {
	router := rpc.New(8, 4096)
	type Input struct {
		Name string `json:"name"`
	}
	if err := rpc.Bind(router, "greet", "", func(ctx context.Context, in Input) (string, error) { return "Hello, " + in.Name, nil }); err != nil {
		return err
	}
	out, err := router.Call(ctx, "greet", []byte(`{"name":"OmniUI"}`), nil)
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	before, err := omniui.NewNode("greeting", "omniui.text", omniui.Props{Text: "Hello"})
	if err != nil {
		return err
	}
	after, err := before.WithProps(omniui.Props{Text: "Hello, OmniUI"})
	if err != nil {
		return err
	}
	fmt.Printf("UI changes: %d; unchanged changes: %d\n", len(omniui.Diff(before, after)), len(omniui.Diff(after, after)))
	start, end, _ := omniui.VisibleRange(1_000_000, 24, 0, 720, 4)
	fmt.Printf("Visible rows: %d of 1000000\n", end-start)
	return nil
}

func main() {
	var registry omniui.Registry
	err := registry.Register(omniui.Extension{Descriptor: omniui.Descriptor{ID: "example.session", Version: "0.1.0", Kind: omniui.Service}, Start: func(context.Context) error { fmt.Println("session started"); return nil }, Stop: func(context.Context) error { fmt.Println("session stopped"); return nil }})
	if err != nil {
		log.Fatal(err)
	}
	app := lifecycle.New(lifecycle.Options{Registry: &registry, Extensions: []string{"example.session"}})
	if err := app.Run(context.Background(), demo{}); err != nil {
		log.Fatal(err)
	}
}
