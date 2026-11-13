package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime/debug"
	"strings"
)

const usage = `OmniGo application tools

Usage: omni <command> [options]

Commands:
  generate   Generate unary RPC TypeScript contracts from an application
  init       Create an OmniWeb application from the current template
  doctor     Inspect development tools and platform capabilities
  keygen     Generate an Ed25519 update-signing key pair
  i18n check Check application message catalogs
  version    Print CLI build version and revision
  help       Show this help or a command's flags

Examples:
  omni help init
  omni doctor --check --json
  omni i18n check --json ./my-app

General dev/build, typed-client generation and release publishing are not
implemented by this CLI yet. See migration/README.md for remaining parity work.
`

// Release builds may supply -ldflags '-X main.version=vX.Y.Z'. Otherwise use
// module build metadata, without inventing a release version for a checkout.
var version string

type usageError struct{ message string }

func (e *usageError) Error() string { return e.message + "\n\n" + usage }

func buildVersion(info *debug.BuildInfo, override string) string {
	selected := override
	if selected == "" && info != nil {
		selected = info.Main.Version
	}
	if selected == "" || selected == "(devel)" {
		selected = "devel"
	}
	detail := []string{}
	if info != nil {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if setting.Value != "" {
					detail = append(detail, "revision "+setting.Value)
				}
			case "vcs.modified":
				if setting.Value == "true" {
					detail = append(detail, "modified")
				}
			}
		}
	}
	text := "omni " + selected
	if len(detail) > 0 {
		text += " (" + strings.Join(detail, ", ") + ")"
	}
	return text
}

func cli(args []string, output, diagnostics io.Writer) int {
	err := runTo(args, output)
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return 0
	}
	fmt.Fprintln(diagnostics, err)
	var badUsage *usageError
	if errors.As(err, &badUsage) {
		return 2
	}
	return 1
}

// FlagSet writes to a private buffer so help can go to the caller's output,
// while parse errors are reported once through the CLI diagnostic stream.
func parseFlags(flags *flag.FlagSet, args []string, output io.Writer) error {
	var help bytes.Buffer
	flags.SetOutput(&help)
	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		if _, writeErr := io.Copy(output, &help); writeErr != nil {
			return writeErr
		}
	}
	return err
}
