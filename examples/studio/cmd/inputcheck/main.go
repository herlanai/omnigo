package main

import (
	"fmt"
	"github.com/herlanai/omnigo/omniwindow/host"
	"os"
)

func main() {
	if err := host.CheckTextAccessibility(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("PASS: AppKit marked text, UTF-16, native commit/undo, candidate rectangle, accessibility roles/actions and stale-object cleanup")
}
