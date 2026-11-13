package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
)

func generate(args []string, output io.Writer) error {
	return generateWith(args, output, func(pkg, path string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		cmd := exec.CommandContext(ctx, "go", "run", pkg)
		for _, value := range os.Environ() {
			if !strings.HasPrefix(value, "OMNIGO_GENERATE=") {
				cmd.Env = append(cmd.Env, value)
			}
		}
		cmd.Env = append(cmd.Env, "OMNIGO_GENERATE="+path)
		cmd.Stdout = output
		cmd.Stderr = output
		return cmd.Run()
	})
}
func generateWith(args []string, output io.Writer, run func(string, string) error) error {
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	pkg := flags.String("package", ".", "application main package (trusted local code)")
	path := flags.String("out", "", "generated TypeScript file")
	if err := parseFlags(flags, args, output); err != nil {
		return err
	}
	if flags.NArg() != 0 || *path == "" || *pkg == "" || strings.HasPrefix(*pkg, "-") {
		return fmt.Errorf("usage: omni generate -package ./cmd/app -out ui/services.ts")
	}
	absolute, err := filepath.Abs(*path)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(absolute), 0755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(absolute), ".omni-generate-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	generated := filepath.Join(staging, "services.ts")
	if err = run(*pkg, generated); err != nil {
		return fmt.Errorf("generate application contracts: %w", err)
	}
	data, err := os.ReadFile(generated)
	if err != nil {
		return fmt.Errorf("application did not write contracts; call omnigo.Run or app.RunMain: %w", err)
	}
	previous, readErr := os.ReadFile(absolute)
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	if !bytes.Equal(previous, data) || readErr != nil {
		if err = os.Rename(generated, absolute); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(output, "Generated %s\n", absolute)
	return err
}
