// Command desktopcheck verifies native macOS dialogs, tray and notifications.
package main

import (
	"fmt"
	"github.com/herlanai/omnigo/omniwindow/host"
	"os"
)

func main() {
	if err := host.CheckDesktopServices(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Native dialogs: cancel, alert response and context cleanup verified; tray: create/update/menu action/close verified; events: appearance KVO and synthetic display/wake notifications verified")
}
