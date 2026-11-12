// Command titlebarcheck verifies native titlebar transitions on macOS.
package main

import (
	"fmt"
	"github.com/herlanai/omnigo/omniwindow/host"
	"os"
)

func main() {
	if err := host.CheckTitlebar(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Native titlebar: hide/restore, HiddenInset traffic lights at 12/11 and 18/19, resize and safe area verified")
}
