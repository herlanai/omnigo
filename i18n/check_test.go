package i18n

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
)

func TestCheckCollectsErrorsAndWarningsDeterministically(t *testing.T) {
	files := fstest.MapFS{
		"en.json": {Data: []byte(`{"greet":"Hello {name}","nested":"{n, plural, one {{who}} other {#}}","missing":"Fallback"}`)},
		"de.json": {Data: []byte(`{"greet":"Hallo {wrong}","nested":"{n, plural, other {#}}","extra":"extra","broken":"{"}`)},
		"fr.json": {Data: []byte(`{"greet":"Bonjour {name}"}`)},
	}
	report, err := Check(files, ".", "en")
	if err != nil {
		t.Fatal(err)
	}
	if report.Errors() != 4 || report.Messages != 3 || len(report.Languages) != 3 {
		t.Fatal(report)
	}
	warnings := 0
	for _, d := range report.Diagnostics {
		if d.Warning {
			warnings++
		}
	}
	if warnings != 5 {
		t.Fatal(report)
	}
	before, _ := json.Marshal(report)
	for range 10 {
		next, err := Check(files, ".", "en")
		if err != nil {
			t.Fatal(err)
		}
		after, _ := json.Marshal(next)
		if string(before) != string(after) {
			t.Fatal("unstable diagnostic order")
		}
	}
}

func TestCheckWarningsDoNotFailAndDoesNotChangeRuntimeFallback(t *testing.T) {
	files := fstest.MapFS{"en.json": {Data: []byte(`{"greet":"Hello {name}","only":"Fallback"}`)}, "de.arb": {Data: []byte(`{"@@locale":"de","greet":"Hallo"}`)}}
	report, err := Check(files, ".", "en")
	if err != nil || report.Errors() != 0 || len(report.Diagnostics) != 2 {
		t.Fatal(report, err)
	}
	catalog, err := Load(files, ".", "en")
	if err != nil {
		t.Fatal(err)
	}
	text, err := catalog.Format("de", "only", nil)
	if err != nil || text != "Fallback" {
		t.Fatal(text, err)
	}
}

func TestCheckInvalidFilesAndDuplicateLocales(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files fstest.MapFS
		want  string
	}{
		{"duplicate-key", fstest.MapFS{"en.json": {Data: []byte(`{"x":"a","x":"b"}`)}}, "duplicate key"},
		{"duplicate-language", fstest.MapFS{"en.json": {Data: []byte(`{}`)}, "en.arb": {Data: []byte(`{}`)}}, "Duplicate normalized"},
		{"missing-source", fstest.MapFS{"de.json": {Data: []byte(`{}`)}}, "Source catalog missing"},
		{"arb-locale", fstest.MapFS{"en.arb": {Data: []byte(`{"@@locale":"de"}`)}}, "locale does not match"},
		{"oversized", fstest.MapFS{"en.json": {Data: []byte(strings.Repeat(" ", (8<<20)+1))}}, "exceeds 8 MiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, err := Check(tc.files, ".", "en")
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, d := range report.Diagnostics {
				if !d.Warning && strings.Contains(d.Detail, tc.want) {
					found = true
				}
			}
			if !found {
				t.Fatal(report)
			}
		})
	}
}
