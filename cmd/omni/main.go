package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/herlanai/omnigo/omniwindow"
	"io"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/herlanai/omnigo/internal/scaffold"
)

func run(args []string) error { return runTo(args, os.Stdout) }

func runTo(args []string, output io.Writer) error {
	if len(args) == 0 {
		return &usageError{message: "missing command"}
	}
	switch args[0] {
	case "help", "-h", "--help":
		if len(args) == 1 {
			_, err := io.WriteString(output, usage)
			return err
		}
		if args[0] != "help" || len(args) > 3 {
			return &usageError{message: "usage: omni help [command]"}
		}
		switch args[1] {
		case "init", "doctor", "keygen", "generate":
			if len(args) != 2 {
				return &usageError{message: "unexpected help argument"}
			}
			return runTo([]string{args[1], "-h"}, output)
		case "i18n":
			if len(args) == 3 && args[2] != "check" {
				return &usageError{message: "unknown i18n subcommand"}
			}
			return checkI18n([]string{"check", "-h"}, output)
		case "version":
			if len(args) != 2 {
				return &usageError{message: "unexpected help argument"}
			}
			_, err := fmt.Fprintln(output, "Usage: omni version (aliases: -v, --version)")
			return err
		default:
			return &usageError{message: fmt.Sprintf("unknown command %q", args[1])}
		}
	case "version", "-v", "--version":
		if len(args) != 1 {
			return &usageError{message: "version takes no arguments"}
		}
		info, _ := debug.ReadBuildInfo()
		_, err := fmt.Fprintln(output, buildVersion(info, version))
		return err
	case "i18n":
		return checkI18n(args[1:], output)
	case "generate":
		return generate(args[1:], output)
	case "keygen":
		return keygen(args[1:], output)
	case "doctor":
		return doctorTo(args[1:], output, localEnvironmentProbe())
	case "init":
	default:
		return &usageError{message: fmt.Sprintf("unknown command %q", args[0])}
	}

	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	framework := flags.String("framework", "", "local OmniGo checkout")
	module := flags.String("module", "example.com/apps", "new Go module path")
	if err := parseFlags(flags, args[1:], output); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("init requires exactly one new destination directory")
	}
	if err := scaffold.Create(flags.Arg(0), *module, *framework); err != nil {
		return err
	}
	_, err := fmt.Fprintf(output, "Created %s: OmniWeb + TanStack Start + shadcn (browser host; native WebView pending).\n", flags.Arg(0))
	if err != nil {
		return fmt.Errorf("project created at %s, but status output failed: %w", flags.Arg(0), err)
	}
	return nil
}

func doctor(args []string) error {
	return doctorTo(args, os.Stdout, localEnvironmentProbe())
}

func doctorTo(args []string, output io.Writer, probe environmentProbe) error {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	all := flags.Bool("all", false, "show six platform targets")
	jsonOutput := flags.Bool("json", false, "machine-readable platform capabilities (with --check: environment report)")
	check := flags.Bool("check", false, "check local development tools; default for human output without --all")
	if err := parseFlags(flags, args, output); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("doctor takes no positional arguments")
	}
	platforms := []omniwindow.PlatformSupport{omniwindow.CurrentPlatform()}
	if *all {
		platforms = omniwindow.Platforms()
	}
	if *check || (!*jsonOutput && !*all) {
		report := inspectEnvironment(context.Background(), runtime.GOOS, runtime.GOARCH, probe)
		report.Platforms = platforms
		var err error
		if *jsonOutput {
			encoder := json.NewEncoder(output)
			encoder.SetIndent("", "  ")
			err = encoder.Encode(report)
		} else {
			err = printEnvironment(output, report)
		}
		if err != nil {
			return err
		}
		if !report.Ready {
			return errors.New("development environment checks failed")
		}
		if *jsonOutput {
			return nil
		}
	}
	if *jsonOutput {
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(platforms)
	}
	for _, p := range platforms {
		if _, err := fmt.Fprintf(output, "%s/%s: core-build=%t native-window=%s webview=%s\n  %s\n", p.OS, p.Arch, p.CoreBuild, p.NativeWindow, p.WebView, p.Evidence); err != nil {
			return err
		}
	}
	return nil
}

func main() { os.Exit(cli(os.Args[1:], os.Stdout, os.Stderr)) }
