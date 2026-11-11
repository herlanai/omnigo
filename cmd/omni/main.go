package main

import (
	"flag"
	"fmt"
	"os"

	"omnigo.dev/omnigo/internal/scaffold"
)

func run(args []string) error {
	if len(args) == 0 || args[0] != "init" {
		return fmt.Errorf("usage: omni init --framework /path/to/omnigo [--module example.com/app] destination")
	}
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	framework := flags.String("framework", "", "local OmniGo checkout")
	module := flags.String("module", "example.com/desktop", "new Go module path")
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

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
