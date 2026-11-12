package i18n

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestCatalogSwitchPluralFallback(t *testing.T) {
	c, err := New("en", map[string]map[string]string{
		"en": {"files": "{n, plural, =0 {Empty} one {# file} other {# files}}", "fallback": "Hello {name}", "ordinal": "{n, selectordinal, one {#st} two {#nd} few {#rd} other {#th}}"},
		"ru": {"files": "{n, plural, one {# файл} few {# файла} many {# файлов} other {# файла}}"}, "zh-Hans": {"files": "{n} 个文件"}, "ar": {"files": "{n, plural, zero {zero} one {one} two {two} few {few} many {many} other {other}}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		lang, id string
		n        int
		want     string
	}{{"en", "files", 0, "Empty"}, {"en", "files", 1, "1 file"}, {"en", "files", 3, "3 files"}, {"ru-RU", "files", 2, "2 файла"}, {"ru", "files", 5, "5 файлов"}, {"ru", "files", 21, "21 файл"}, {"zh-CN", "files", 3, "3 个文件"}, {"ar", "files", 2, "two"}, {"ar", "files", 7, "few"}, {"en", "ordinal", 22, "22nd"}} {
		got, err := c.Format(tc.lang, tc.id, map[string]any{"n": tc.n})
		if err != nil || got != tc.want {
			t.Fatalf("%+v: %q %v", tc, got, err)
		}
	}
	tr, _ := c.Translator("ru")
	calls := 0
	unsubscribe := tr.Subscribe(func() { calls++ })
	if err = tr.SetLanguage("ar"); err != nil {
		t.Fatal(err)
	}
	if !tr.RTL() || calls != 1 {
		t.Fatal("language change")
	}
	unsubscribe()
	tr.SetLanguage("en")
	if calls != 1 || tr.RTL() {
		t.Fatal("unsubscribe")
	}
	got, err := c.Format("ru", "fallback", map[string]any{"name": "世界"})
	if err != nil || got != "Hello 世界" {
		t.Fatal(got, err)
	}
}
func TestNestedMessageAndNumbers(t *testing.T) {
	c, err := New("en", map[string]map[string]string{"en": {"nested": "{n, plural, offset:1 =0 {none} one {{gender, select, male {He and # guest} other {They and # guest}}} other {# guests}}", "quote": "'{literal}' '' '#': {value}", "number": "{n, number, ::.00}"}, "de": {"number": "{n, number, ::.00}"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Format("en", "nested", map[string]any{"n": 2, "gender": "male"})
	if err != nil || got != "He and 1 guest" {
		t.Fatal(got, err)
	}
	got, err = c.Format("en", "quote", map[string]any{"value": "ok"})
	if err != nil || got != "{literal} ' #: ok" {
		t.Fatal(got, err)
	}
	got, err = c.Format("de", "number", map[string]any{"n": 1234.5})
	if err != nil || got != "1.234,50" {
		t.Fatal(got, err)
	}
	if _, err = c.Format("en", "number", nil); err == nil {
		t.Fatal("missing argument accepted")
	}
}
func TestCatalogValidationAndSnapshots(t *testing.T) {
	for _, pattern := range []string{"{", "}", "{x, plural, one {x}}", "{x, select, other {a} other {b}}", "{x, date, short}", "{x, plural, zero {a}", strings.Repeat("{x,select,other{", 34) + "ok" + strings.Repeat("}}", 34)} {
		if _, err := compile(pattern); err == nil {
			t.Fatalf("accepted invalid message %q", pattern)
		}
	}
	fsys := fstest.MapFS{"en.arb": {Data: []byte(`{"@@locale":"en","greeting":"Hello {name}","@greeting":{"description":"test"}}`)}}
	c, err := Load(fsys, ".", "en")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Format("en", "greeting", map[string]any{"name": "x"}); err != nil {
		t.Fatal(err)
	}
	fsys["en.arb"].Data = []byte(`{"x":"one","x":"two"}`)
	if _, err = Load(fsys, ".", "en"); err == nil {
		t.Fatal("duplicate JSON key")
	}
	raw := map[string]map[string]string{"en": {"x": "old"}}
	c, _ = New("en", raw)
	raw["en"]["x"] = "new"
	if got, _ := c.Format("en", "x", nil); got != "old" {
		t.Fatal("catalog retained caller state")
	}
}
func FuzzMessageParser(f *testing.F) {
	f.Add("{n, plural, one {# file} other {# files}}")
	f.Add("hello '{'")
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 65536 {
			t.Skip()
		}
		_, _ = compile(s)
	})
}

func TestCurrencyAndCanonicalExactSelectors(t *testing.T) {
	c, err := New("en", map[string]map[string]string{"en": {"price": "{n, number, ::currency/USD}", "exact": "{n, plural, =1.0 {exact} other {other}}"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Format("en", "price", map[string]any{"n": 1234.5})
	if err != nil || !strings.Contains(got, "1,234.50") || !strings.Contains(got, "$") {
		t.Fatal(got, err)
	}
	got, err = c.Format("en", "exact", map[string]any{"n": 1})
	if err != nil || got != "exact" {
		t.Fatal(got, err)
	}
	if _, err = compile("{n, plural, =1 {first} =1.0 {second} other {other}}"); err == nil {
		t.Fatal("duplicate numeric selector")
	}
}
