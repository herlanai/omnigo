package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/herlanai/omnigo/i18n"
)

func catalogProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range map[string]string{
		"omnigo.json":     `{"i18n":{"dir":"locales","source":"en"}}`,
		"locales/en.json": `{"greet":"Hello {name}","fallback":"Fallback"}`,
		"locales/de.json": `{"greet":"Hallo {name}"}`,
	} {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
func TestI18nCommandWarningsAndErrors(t *testing.T) {
	dir := catalogProject(t)
	var output bytes.Buffer
	if err := checkI18n([]string{"check", "--json", dir}, &output); err != nil {
		t.Fatal(err)
	}
	var report i18n.CheckReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil || report.Errors() != 0 || len(report.Diagnostics) != 1 {
		t.Fatal(report, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "locales/de.json"), []byte(`{"greet":"Hallo {wrong}"}`), 0600); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := checkI18n([]string{"check", "--json", dir}, &output); err == nil {
		t.Fatal("bad arguments accepted")
	}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil || report.Errors() != 1 {
		t.Fatal(output.String(), err)
	}
}
func TestI18nExplicitDirectoryAndConfinement(t *testing.T) {
	dir := catalogProject(t)
	if err := os.Remove(filepath.Join(dir, "omnigo.json")); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := checkI18n([]string{"check", "--dir", "locales", dir}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "warning") {
		t.Fatal(output.String())
	}
	for _, args := range [][]string{{"check", dir}, {"check", "--dir", "../outside", dir}, {"check", dir, dir}} {
		if err := checkI18n(args, &output); err == nil {
			t.Fatal("invalid request accepted", args)
		}
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(dir, "escape")); err == nil {
		if err := checkI18n([]string{"check", "--dir", "escape", dir}, &output); err == nil {
			t.Fatal("catalog directory escaped project")
		}
	}
}

func TestI18nCommandChecksSourcePlaceholderTypes(t *testing.T) {
	dir := catalogProject(t)
	if err := os.WriteFile(filepath.Join(dir, "locales/en.json"), []byte(`{"greet":"{n}","@greet":{"placeholders":{"n":{"type":"String"}}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "locales/de.json"), []byte(`{"greet":"{n, number}"}`), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := checkI18n([]string{"check", "--json", dir}, &output); err == nil {
		t.Fatal("incompatible translation accepted")
	}
	var report i18n.CheckReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil || report.Errors() != 1 || !strings.Contains(output.String(), "argument-types") {
		t.Fatal(output.String(), err)
	}
}

func TestI18nCommandIncludesDiagnosticLine(t *testing.T) {
	dir := catalogProject(t)
	if err := os.WriteFile(filepath.Join(dir, "locales/de.json"), []byte("{\n\n  \"greet\": \"Hallo {wrong}\"\n}"), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := checkI18n([]string{"check", dir}, &output); err == nil {
		t.Fatal("bad parameter accepted")
	}
	if !strings.Contains(output.String(), "locales/de.json:3 [error]") {
		t.Fatal(output.String())
	}
	output.Reset()
	_ = checkI18n([]string{"check", "--json", dir}, &output)
	var report i18n.CheckReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	for _, d := range report.Diagnostics {
		if !d.Warning && d.Line != 3 {
			t.Fatal(d)
		}
	}
}
