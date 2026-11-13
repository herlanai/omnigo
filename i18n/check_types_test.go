package i18n

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestCheckArgumentTypeContracts(t *testing.T) {
	for _, tc := range []struct {
		name, source, translation string
		errors                    int
	}{
		{"inferred-string-cannot-be-number", `{"m":"Hello {name}"}`, `{"m":"{name, number}"}`, 1},
		{"inferred-number-cannot-select", `{"m":"{n, plural, other {#}}"}`, `{"m":"{n, select, other {text}}"}`, 1},
		{"numeric-plain-interpolation", `{"m":"{n, number}"}`, `{"m":"{n}"}`, 0},
		{"declared-double", `{"m":"{n}","@m":{"placeholders":{"n":{"type":"double"}}}}`, `{"m":"{n, plural, one {one} other {many}}"}`, 0},
		{"object-can-select-or-count", `{"m":"{v}","@m":{"placeholders":{"v":{"type":"Object"}}}}`, `{"m":"{v, select, one {{v, number}} other {other}}"}`, 0},
		{"translation-cannot-widen", `{"m":"{v}"}`, `{"m":"{v, number}","@m":{"placeholders":{"v":{"type":"Object"}}}}`, 1},
		{"source-contradicts-declaration", `{"m":"{v, number}","@m":{"placeholders":{"v":{"type":"String"}}}}`, `{"m":"{v}"}`, 1},
		{"nested-type-error", `{"m":"{choice, select, other {{value}}}"}`, `{"m":"{choice, select, other {{value, number}}}"}`, 1},
		{"date-interpolation", `{"m":"{day}","@m":{"placeholders":{"day":{"type":"DateTime"}}}}`, `{"m":"{day}"}`, 0},
		{"date-not-numeric", `{"m":"{day}","@m":{"placeholders":{"day":{"type":"DateTime"}}}}`, `{"m":"{day, number}"}`, 1},
		{"untyped-and-numeric-same-argument", `{"m":"{n} {n, number}"}`, `{"m":"{n, number}"}`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, err := Check(fstest.MapFS{"en.arb": {Data: []byte(tc.source)}, "de.arb": {Data: []byte(tc.translation)}}, ".", "en")
			if err != nil || report.Errors() != tc.errors {
				t.Fatal(report, err)
			}
		})
	}
}

func TestDeclaredUnusedPlaceholderIsPartOfTranslationContract(t *testing.T) {
	report, err := Check(fstest.MapFS{
		"en.arb": {Data: []byte(`{"m":"Hello","@m":{"placeholders":{"name":{"type":"String"}}}}`)},
		"de.arb": {Data: []byte(`{"m":"Hallo {name}"}`)},
	}, ".", "en")
	if err != nil || report.Errors() != 0 || len(report.Diagnostics) != 1 || !report.Diagnostics[0].Warning || !strings.Contains(report.Diagnostics[0].Detail, "unused by source") {
		t.Fatal(report, err)
	}
}

func TestCheckSourceMetadataErrorsAndWarnings(t *testing.T) {
	for _, tc := range []struct {
		name, metadata, want string
		warning              bool
	}{
		{"unknown-type", `"@m":{"placeholders":{"x":{"type":"Boolean"}}}`, "unknown type", false},
		{"duplicate-placeholder", `"@m":{"placeholders":{"x":{},"x":{}}}`, "duplicate placeholder", false},
		{"invalid-placeholders", `"@m":{"placeholders":[]}`, "expected an object", false},
		{"invalid-descriptor", `"@m":{"placeholders":{"x":42}}`, "Invalid placeholders", false},
		{"invalid-type-field", `"@m":{"placeholders":{"x":{"type":42}}}`, "Invalid placeholders", false},
		{"invalid-description", `"@m":{"description":42}`, "Invalid message metadata", false},
		{"orphan-metadata", `"@absent":{"description":"orphan"}`, "describes no message", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, err := Check(fstest.MapFS{"en.arb": {Data: []byte(`{"m":"{x}",` + tc.metadata + `}`)}}, ".", "en")
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, d := range report.Diagnostics {
				if d.Warning == tc.warning && strings.Contains(d.Detail, tc.want) {
					found = true
				}
			}
			if !found {
				t.Fatal(report)
			}
		})
	}
}
