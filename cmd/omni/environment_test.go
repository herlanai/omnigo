package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/herlanai/omnigo/omniwindow"
)

func fixtureProbe(outputs map[string]string) environmentProbe {
	return environmentProbe{
		run: func(ctx context.Context, name string, args ...string) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			if output, ok := outputs[name]; ok {
				return output, nil
			}
			return "", fs.ErrNotExist
		},
		lookup: func(string) (string, error) { return "", fs.ErrNotExist },
		getenv: func(string) string { return "" },
	}
}
func healthyProbe() environmentProbe {
	return fixtureProbe(map[string]string{
		"go": "go version go1.26.0 darwin/arm64", "node": "v24.18.0", "npm": "11.16.0", "npm.cmd": "11.16.0", "sw_vers": "26.1",
	})
}

func TestEnvironmentRequiredVersionsAndOptionalTools(t *testing.T) {
	for _, tc := range []struct {
		name, goVersion, nodeVersion, npmVersion string
		ready                                    bool
	}{
		{"ready", "go version go1.26.0 linux/amd64", "v24.0.0", "11.0.0", true},
		{"old-go", "go version go1.25.9 linux/amd64", "v24.0.0", "11.0.0", false},
		{"old-node", "go version go1.26.0 linux/amd64", "v22.20.0", "11.0.0", false},
		{"missing-npm", "go version go1.26.0 linux/amd64", "v24.0.0", "", false},
		{"malformed", "error: go1.26.0", "v24.0.0", "11.0.0", false},
		{"prerelease-go", "go version go1.26rc1 linux/amd64", "v24.0.0", "11.0.0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := inspectEnvironment(t.Context(), "linux", "amd64", fixtureProbe(map[string]string{"go": tc.goVersion, "node": tc.nodeVersion, "npm": tc.npmVersion}))
			if report.Ready != tc.ready {
				t.Fatal(report)
			}
			for _, check := range report.Checks {
				if check.Name == "gh" && check.Status != "warn" {
					t.Fatal(check)
				}
			}
		})
	}
}

func TestDoctorLegacyJSONDoesNotRunEnvironmentTools(t *testing.T) {
	var out bytes.Buffer
	probe := healthyProbe()
	probe.run = func(context.Context, string, ...string) (string, error) {
		t.Fatal("legacy query executed a tool")
		return "", nil
	}
	if err := doctorTo([]string{"--all", "--json"}, &out, probe); err != nil {
		t.Fatal(err)
	}
	var platforms []omniwindow.PlatformSupport
	if err := json.Unmarshal(out.Bytes(), &platforms); err != nil || len(platforms) != 6 {
		t.Fatal(out.String(), err)
	}
}

func TestDoctorStructuredFailureStillProducesJSON(t *testing.T) {
	var out bytes.Buffer
	err := doctorTo([]string{"--check", "--json"}, &out, fixtureProbe(nil))
	if err == nil {
		t.Fatal("missing tools accepted")
	}
	var report environmentReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(out.String(), err)
	}
	if report.Ready || len(report.Checks) < 3 || len(report.Platforms) != 1 {
		t.Fatal(report)
	}
}

func TestDoctorDefaultChecksToolsAndSupportsAllTargets(t *testing.T) {
	var out bytes.Buffer
	if err := doctorTo(nil, &out, healthyProbe()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "[pass] go:") || strings.Contains(out.String(), "[pass] node:") {
		t.Fatal(out.String())
	}
	out.Reset()
	if err := doctorTo([]string{"--check", "--all", "--json"}, &out, healthyProbe()); err != nil {
		t.Fatal(err)
	}
	var report environmentReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || !report.Ready || len(report.Platforms) != 6 {
		t.Fatal(report, err)
	}
}

func TestEnvironmentPlatformEvidenceDoesNotClaimNativeAvailability(t *testing.T) {
	probe := healthyProbe()
	run := probe.run
	probe.getenv = func(string) string { return ":0" }
	probe.run = func(ctx context.Context, name string, args ...string) (string, error) {
		if name == "ldconfig" {
			return "libX11.so.6 (libc6,x86-64) => /lib/libX11.so.6", nil
		}
		return run(ctx, name, args...)
	}
	report := inspectEnvironment(t.Context(), "linux", "amd64", probe)
	foundX11, foundNative := false, false
	for _, check := range report.Checks {
		if check.Name == "X11" {
			foundX11 = check.Status == "pass" && strings.Contains(check.Detail, "not tested")
		}
		if check.Name == "native-host" {
			foundNative = check.Status == "warn" && strings.Contains(check.Detail, "unavailable")
		}
	}
	if !report.Ready || !foundX11 || !foundNative {
		t.Fatal(report)
	}
	if inspectEnvironment(t.Context(), "linux", "386", probe).Ready {
		t.Fatal("unsupported architecture accepted")
	}
}

func TestEnvironmentCancellationAndBoundedOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	report := inspectEnvironment(ctx, "linux", "amd64", healthyProbe())
	if report.Ready {
		t.Fatal("canceled check accepted")
	}
	if _, err := localEnvironmentProbe().run(ctx, "go", "version"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	var output probeOutput
	if _, err := output.Write(bytes.Repeat([]byte{'x'}, 16<<10)); err != nil {
		t.Fatal(err)
	}
	if _, err := output.Write([]byte("x")); err == nil || len(output.data) != 16<<10 {
		t.Fatal("output limit not enforced")
	}
}

func TestDoctorPropagatesOutputFailuresInEveryFormat(t *testing.T) {
	for _, args := range [][]string{{"--all"}, {"--json"}, {"--check", "--json"}, nil} {
		if err := doctorTo(args, failedOutput{}, healthyProbe()); err == nil || !strings.Contains(err.Error(), "output unavailable") {
			t.Fatal(args, err)
		}
	}
}

func TestDoctorWebKitLibrariesAreOptionalAndExact(t *testing.T) {
	for _, present := range []bool{false, true} {
		output := "libgtk-4.so.10 (libc6) => /wrong\nlibwebkitgtk-6.0.so.40 (libc6) => /wrong"
		if present {
			output = "libgtk-4.so.1 (libc6) => /gtk\nlibwebkitgtk-6.0.so.4 (libc6) => /webkit"
		}
		p := healthyProbe()
		previous := p.run
		p.run = func(ctx context.Context, name string, args ...string) (string, error) {
			if name == "ldconfig" {
				return output, nil
			}
			return previous(ctx, name, args...)
		}
		report := inspectEnvironment(t.Context(), "linux", "amd64", p)
		if !report.Ready {
			t.Fatal("optional libraries blocked ordinary applications")
		}
		found := 0
		for _, check := range report.Checks {
			if check.Name == "libgtk-4.so.1" || check.Name == "libwebkitgtk-6.0.so.4" {
				found++
				want := "warn"
				if present {
					want = "pass"
				}
				if check.Status != want {
					t.Fatal(check)
				}
			}
		}
		if found != 2 {
			t.Fatal("missing WebKit diagnostics")
		}
	}
}

func TestGoProfileDoesNotRequireNode(t *testing.T) {
	p := fixtureProbe(map[string]string{"go": "go version go1.26.0 darwin/arm64"})
	for _, profile := range []string{"go", "web", "studio"} {
		report := inspectEnvironmentProfile(t.Context(), "darwin", "arm64", p, profile)
		if report.Ready != (profile == "go") {
			t.Fatal(profile, report)
		}
	}
}

func TestDoctorSelectsProjectToolchain(t *testing.T) {
	for _, profile := range []string{"go", "web", "studio", "invalid"} {
		path := filepath.Join(t.TempDir(), "omnigo.json")
		if err := os.WriteFile(path, []byte(`{"toolchain":"`+profile+`"}`), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := environmentProfile("auto", path)
		if profile == "invalid" {
			if err == nil {
				t.Fatal("invalid toolchain accepted")
			}
			continue
		}
		if err != nil || got != profile {
			t.Fatal(got, err)
		}
	}
	if _, err := environmentProfile("auto", filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("explicit missing config ignored")
	}
	if got, err := environmentProfile("go", "missing.json"); err != nil || got != "go" {
		t.Fatal(got, err)
	}
}
