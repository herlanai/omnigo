package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/herlanai/omnigo/i18n"
)

func checkI18n(args []string, output io.Writer) error {
	if len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		return checkI18n([]string{"check", "-h"}, output)
	}
	if len(args) == 0 || args[0] != "check" {
		return errors.New("usage: omni i18n check [--json] [--dir locales] [--source en] [project-dir]")
	}
	flags := flag.NewFlagSet("i18n check", flag.ContinueOnError)
	jsonOutput := flags.Bool("json", false, "structured diagnostics")
	directory := flags.String("dir", "", "catalog directory relative to the project")
	source := flags.String("source", "", "source language")
	if err := parseFlags(flags, args[1:], output); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return errors.New("i18n check accepts at most one project directory")
	}
	project := "."
	if flags.NArg() == 1 {
		project = flags.Arg(0)
	}
	root, err := os.OpenRoot(project)
	if err != nil {
		return err
	}
	defer root.Close()
	var config struct {
		I18n *struct {
			Dir    string `json:"dir"`
			Source string `json:"source"`
		} `json:"i18n"`
	}
	file, err := root.Open("omnigo.json")
	if err == nil {
		defer file.Close()
		data, readErr := io.ReadAll(io.LimitReader(file, (1<<20)+1))
		if readErr != nil {
			return readErr
		}
		if len(data) > 1<<20 {
			return errors.New("omnigo.json exceeds 1 MiB")
		}
		if err = json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("omnigo.json: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if config.I18n == nil && *directory == "" {
		return errors.New("set i18n in omnigo.json or provide --dir")
	}
	if config.I18n != nil {
		if *directory == "" {
			*directory = config.I18n.Dir
		}
		if *source == "" {
			*source = config.I18n.Source
		}
	}
	if *directory == "" {
		*directory = "locales"
	}
	if *source == "" {
		*source = "en"
	}
	local, err := filepath.Localize(*directory)
	if err != nil || local == "." {
		return errors.New("catalog directory must be a project-relative subdirectory")
	}
	catalogs, err := root.OpenRoot(local)
	if err != nil {
		return err
	}
	defer catalogs.Close()
	report, err := i18n.Check(catalogs.FS(), ".", *source)
	if err != nil {
		return err
	}
	if *jsonOutput {
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		if err = encoder.Encode(report); err != nil {
			return err
		}
	} else {
		for _, d := range report.Diagnostics {
			severity := "error"
			if d.Warning {
				severity = "warning"
			}
			location := filepath.ToSlash(filepath.Join(*directory, d.File))
			if d.Line > 0 {
				location = fmt.Sprintf("%s:%d", location, d.Line)
			}
			if _, err = fmt.Fprintf(output, "%s [%s] %s: %s\n", location, severity, d.Message, d.Detail); err != nil {
				return err
			}
		}
		if _, err = fmt.Fprintf(output, "Checked %d messages in %d languages (syntax, keys, argument names/types). Plural-category checks remain incomplete.\n", report.Messages, len(report.Languages)); err != nil {
			return err
		}
	}
	if report.Errors() > 0 {
		return fmt.Errorf("%d catalog errors", report.Errors())
	}
	return nil
}
