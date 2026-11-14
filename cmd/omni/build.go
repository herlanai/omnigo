package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/herlanai/omnigo/internal/appbuild"
	"io"
	"os"
	"os/signal"
)

func buildApplication(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	config := flags.String("config", "omnigo.json", "trusted application build configuration")
	if err := parseFlags(flags, args, output); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("usage: omni build [-config omnigo.json]")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	target, err := appbuild.Build(ctx, *config, output, nil)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Built %s (executable only; use application packaging for distribution)\n", target)
	return err
}

func packageApplication(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("package", flag.ContinueOnError)
	config := flags.String("config", "omnigo.json", "application build and bundle configuration")
	if err := parseFlags(flags, args, output); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("usage: omni package [-config omnigo.json]")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	path, err := appbuild.PackageMac(ctx, *config, output)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Packaged %s (ad-hoc signed; not notarized)\n", path)
	return err
}

func devApplication(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("dev", flag.ContinueOnError)
	config := flags.String("config", "omnigo.json", "application configuration")
	if err := parseFlags(flags, args, output); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("usage: omni dev [-config omnigo.json]")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err := appbuild.Dev(ctx, *config, output)
	if ctx.Err() != nil {
		return nil
	}
	return err
}
