package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func TestCLIHelpUsesOutputAndSucceedsWithoutDoingWork(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}, {"help", "build"}, {"build", "-h"}, {"help", "init"}, {"init", "-h"}, {"help", "doctor"}, {"doctor", "--help"}, {"help", "keygen"}, {"keygen", "-h"}, {"help", "i18n"}, {"help", "i18n", "check"}, {"i18n", "--help"}, {"i18n", "check", "-h"}, {"help", "version"}} {
		var out, diagnostics bytes.Buffer
		if code := cli(args, &out, &diagnostics); code != 0 || out.Len() == 0 || diagnostics.Len() != 0 {
			t.Fatal(args, code, out.String(), diagnostics.String())
		}
	}
}

func TestCLIUnknownCommandAndFlagExitCodes(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
	}{
		{nil, 2}, {[]string{"not-a-command"}, 2}, {[]string{"help", "unknown"}, 2}, {[]string{"version", "extra"}, 2}, {[]string{"--help", "extra"}, 2},
		{[]string{"doctor", "--unknown"}, 1}, {[]string{"keygen", "--unknown"}, 1}, {[]string{"i18n", "check", "--unknown"}, 1},
	} {
		var out, diagnostics bytes.Buffer
		if code := cli(tc.args, &out, &diagnostics); code != tc.code || out.Len() != 0 || diagnostics.Len() == 0 {
			t.Fatal(tc.args, code, out.String(), diagnostics.String())
		}
	}
}

func TestVersionDoesNotInventReleaseAndIncludesBuildProvenance(t *testing.T) {
	for _, tc := range []struct {
		info           *debug.BuildInfo
		override, want string
	}{
		{nil, "", "omni devel"},
		{&debug.BuildInfo{Main: debug.Module{Version: "v0.2.0"}}, "", "omni v0.2.0"},
		{&debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abcdef"}, {Key: "vcs.modified", Value: "true"}}}, "", "omni devel (revision abcdef, modified)"},
		{&debug.BuildInfo{Main: debug.Module{Version: "v0.2.0"}}, "v1.0.0", "omni v1.0.0"},
	} {
		if got := buildVersion(tc.info, tc.override); got != tc.want {
			t.Fatal(got, tc.want)
		}
	}
	var expected string
	for _, alias := range []string{"version", "-v", "--version"} {
		var out, diagnostics bytes.Buffer
		if code := cli([]string{alias}, &out, &diagnostics); code != 0 || diagnostics.Len() != 0 || !strings.HasPrefix(out.String(), "omni ") {
			t.Fatal(code, out.String(), diagnostics.String())
		}
		if expected != "" && expected != out.String() {
			t.Fatal("version aliases differ")
		}
		expected = out.String()
	}
}

type failedOutput struct{}

func (failedOutput) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }
func TestCLIReportsFailedHelpAndVersionOutput(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"version"}, {"init", "-h"}} {
		var diagnostics bytes.Buffer
		if code := cli(args, failedOutput{}, &diagnostics); code != 1 || !strings.Contains(diagnostics.String(), "output unavailable") {
			t.Fatal(args, code, diagnostics.String())
		}
	}
}

func TestCLIPlatformReportOutputFailureIsNotSuccess(t *testing.T) {
	var diagnostics bytes.Buffer
	if code := cli([]string{"doctor", "--all"}, failedOutput{}, &diagnostics); code != 1 || !strings.Contains(diagnostics.String(), "output unavailable") {
		t.Fatal(code, diagnostics.String())
	}
}

func TestInitOutputFailurePreservesCreatedProject(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "app")
	err = runTo([]string{"init", "--framework", root, destination}, failedOutput{})
	if err == nil || !strings.Contains(err.Error(), "project created at") || !strings.Contains(err.Error(), "status output failed") {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(destination, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runTo([]string{"init", "--framework", root, destination}, &out); err == nil {
		t.Fatal("retry overwrote created project")
	}
	after, err := os.ReadFile(filepath.Join(destination, "go.mod"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("project changed on retry", err)
	}
}
