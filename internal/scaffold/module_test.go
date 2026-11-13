package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModulePathsFollowGoModuleInitializationRules(t *testing.T) {
	for _, name := range []string{"app", "my-app", "example.com/team/app", "example.com/app/v2", "gopkg.in/yaml.v3", "example.com/~team/app", "example.com/a..b"} {
		if err := validateModule(name); err != nil {
			t.Fatal(name, err)
		}
	}
	framework, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "go", "toolchain", "example.com/app/v1", "gopkg.in/yaml", "CON", "example.com/NUL.txt", "example.com/x.", "foo//bar", "../outside", "example.com/app\nmodule evil", "example.com/app~1"} {
		destination := filepath.Join(t.TempDir(), "must-not-exist")
		if err := Create(destination, name, framework); err == nil {
			t.Fatal("invalid module accepted", name)
		}
		if _, err := os.Stat(destination); !os.IsNotExist(err) {
			t.Fatal("invalid module created destination", name, err)
		}
	}
}

func TestFrameworkModuleIdentityUsesParsedDirective(t *testing.T) {
	for _, tc := range []struct {
		name, contents string
		valid          bool
	}{
		{"quoted-crlf", "module \"github.com/herlanai/omnigo\"\r\n\r\ngo 1.26.0\r\n", true},
		{"trailing-comment", "module github.com/herlanai/omnigo // local checkout\n\ngo 1.26.0\n", true},
		{"comment-is-not-module", "module example.com/other\n// module github.com/herlanai/omnigo\n", false},
		{"missing-module", "// module github.com/herlanai/omnigo\ngo 1.26.0\n", false},
		{"duplicate-directive", "module github.com/herlanai/omnigo\nmodule example.com/other\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			framework := t.TempDir()
			if err := os.WriteFile(filepath.Join(framework, "go.mod"), []byte(tc.contents), 0600); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(t.TempDir(), "app")
			err := Create(destination, "my-app", framework)
			if (err == nil) != tc.valid {
				t.Fatal(err)
			}
			if !tc.valid {
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatal("invalid framework created target", err)
				}
			}
		})
	}
}
