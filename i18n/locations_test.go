package i18n

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestCatalogDiagnosticLines(t *testing.T) {
	for _, tc := range []struct {
		name, text, fragment string
		line                 int
	}{
		{"duplicate-key", "{\n  \"a\": \"first\",\n  \"a\": \"second\"\n}", "duplicate key", 3},
		{"invalid-value", "{\n  \"a\": 123\n}", "must be string", 2},
		{"bad-json", "{\n  \"a\": true,\n  \"b\": !\n}", "must be string", 2},
		{"syntax-offset", "{\n  \"a\": \"okay\",\n  \"b\": !\n}", "invalid character", 3},
		{"unexpected-eof", "{\n  \"a\": \"okay\",\n", "EOF", 3},
		{"message-syntax", "{\n  \"a\": \"fine\",\n  \"b\": \"{\"\n}", "empty argument", 3},
		{"metadata", "{\n  \"a\": \"{name}\",\n  \"@a\": {\n    \"placeholders\": {\"name\": {\"type\": \"BadType\"}}\n  }\n}", "unknown type", 3},
		{"trailing-data", "{\n  \"a\": \"fine\"\n}\n{}", "trailing catalog", 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, err := Check(fstest.MapFS{"en.json": {Data: []byte(tc.text)}}, ".", "en")
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, d := range report.Diagnostics {
				if strings.Contains(d.Detail, tc.fragment) {
					found = true
					if d.Line != tc.line {
						t.Fatalf("got line %d, want %d: %+v", d.Line, tc.line, d)
					}
				}
			}
			if !found {
				t.Fatal(report)
			}
		})
	}
}

func TestDiagnosticLocationsHandleUnicodeCRLFAndEscapedKeys(t *testing.T) {
	files := fstest.MapFS{
		"en.json": {Data: []byte("{\r\n  \"你好\": \"Hello {name}\",\r\n  \"quote\\\"key\": \"Okay\",\r\n  \"missing\": \"Fallback\"\r\n}")},
		"de.json": {Data: []byte("{\r\n\r\n  \"你好\": \"{wrong}\",\r\n  \"quote\\\"key\": \"{\"\r\n}")},
	}
	report, err := Check(files, ".", "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range report.Diagnostics {
		switch d.Message {
		case "你好":
			if d.Line != 3 {
				t.Fatal(d)
			}
		case "quote\"key":
			if d.Line != 4 {
				t.Fatal(d)
			}
		case "missing":
			if d.Line != 0 {
				t.Fatal("invented location for absent translation", d)
			}
		default:
			t.Fatal(d)
		}
	}
	if report.Errors() != 2 {
		t.Fatal(report)
	}
}
