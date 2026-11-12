package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/herlanai/omnigo/apps"
	"os"

	"github.com/herlanai/omnigo/internal/scaffold"
)

func run(args []string) error {
	if len(args) > 0 && args[0] == "doctor" {
		return doctor(args[1:])
	}
	if len(args) == 0 || args[0] != "init" {
		return fmt.Errorf("usage: omni init --framework /path/to/omnigo [--module example.com/app] destination | omni doctor [--all] [--json]")
	}
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	framework := flags.String("framework", "", "local OmniGo checkout")
	module := flags.String("module", "example.com/apps", "new Go module path")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("init requires exactly one new destination directory")
	}
	if err := scaffold.Create(flags.Arg(0), *module, *framework); err != nil {
		return err
	}
	fmt.Printf("Created %s: OmniWeb + TanStack Start + shadcn (browser host; native WebView pending).\n", flags.Arg(0))
	return nil
}

func doctor(args []string) error {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	all := flags.Bool("all", false, "show six platform targets")
	jsonOutput := flags.Bool("json", false, "machine-readable platform capabilities")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("doctor takes no positional arguments")
	}
	platforms := []apps.PlatformSupport{apps.CurrentPlatform()}
	if *all {
		platforms = apps.Platforms()
	}
	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(platforms)
	}
	for _, p := range platforms {
		fmt.Printf("%s/%s: core-build=%t native-window=%s webview=%s\n  %s\n", p.OS, p.Arch, p.CoreBuild, p.NativeWindow, p.WebView, p.Evidence)
	}
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
