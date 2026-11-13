package i18n

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestCheckContinuesAfterInvalidValuesAndDuplicateKeys(t *testing.T) {
	data := []byte("{\n  \"first\": 123,\n  \"good\": \"Hello {name}\",\n  \"second\": false,\n  \"good\": \"replacement\",\n  \"broken\": \"{\",\n  \"last\": []\n}")
	files := fstest.MapFS{"en.json": {Data: data}}
	report, err := Check(files, ".", "en")
	if err != nil {
		t.Fatal(err)
	}
	if report.Errors() != 5 {
		t.Fatal(report)
	}
	want := map[int]string{2: "must be string", 4: "must be string", 5: "duplicate key", 6: "empty argument", 7: "must be string"}
	for _, d := range report.Diagnostics {
		if text, ok := want[d.Line]; !ok || !strings.Contains(d.Detail, text) {
			t.Fatal(d)
		}
		delete(want, d.Line)
	}
	if len(want) != 0 {
		t.Fatal(want)
	}
	if _, err := Load(files, ".", "en"); err == nil || !strings.Contains(err.Error(), "first must be string") {
		t.Fatal("runtime did not reject first invalid value", err)
	}
}

func TestDuplicateMetadataCannotReplaceOriginalContractOrLocation(t *testing.T) {
	files := fstest.MapFS{
		"en.arb": {Data: []byte("{\n  \"m\": \"{n, number}\",\n  \"@m\": {\"placeholders\": {\"n\": {\"type\": \"String\"}}},\n  \"@m\": {\"placeholders\": {\"n\": {\"type\": \"Object\"}}}\n}")},
		"de.arb": {Data: []byte(`{"m":"{n, number}"}`)},
	}
	report, err := Check(files, ".", "en")
	if err != nil {
		t.Fatal(err)
	}
	if report.Errors() != 3 {
		t.Fatal(report)
	}
	for _, d := range report.Diagnostics {
		if strings.Contains(d.Detail, "duplicate key") && d.Line != 4 {
			t.Fatal(d)
		}
		if d.File == "en.arb" && d.Message == "m" && d.Line != 2 {
			t.Fatal(d)
		}
	}
}

func TestNullAndEmptyMessagesPreserveReferenceDecodeSemantics(t *testing.T) {
	files := fstest.MapFS{"en.json": {Data: []byte(`{"empty":"", "null":null}`)}}
	report, err := Check(files, ".", "en")
	if err != nil || report.Errors() != 0 || report.Messages != 2 {
		t.Fatal(report, err)
	}
	catalog, err := Load(files, ".", "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"empty", "null"} {
		text, err := catalog.Format("en", id, nil)
		if err != nil || text != "" {
			t.Fatal(text, err)
		}
	}
}

func TestInvalidLocaleDoesNotHideOtherMessageDiagnostics(t *testing.T) {
	files := fstest.MapFS{"en.arb": {Data: []byte("{\n  \"@@locale\": \"de\",\n  \"one\": false,\n  \"two\": {}\n}")}}
	report, err := Check(files, ".", "en")
	if err != nil || report.Errors() != 3 {
		t.Fatal(report, err)
	}
}
