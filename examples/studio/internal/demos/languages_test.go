package demos

import (
	"testing"

	ui "github.com/herlanai/omnigo/omniui/baseui"
)

func TestNativeLanguageSwitchPreservesApplicationState(t *testing.T) {
	for _, dark := range []bool{false, true} {
		d := New("i18n")
		s, err := ui.NewSurface(900, 700, ui.NewTheme(dark))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Close() })
		t.Cleanup(d.Close)
		mount := func() {
			t.Helper()
			if err := s.Mount(d.View()); err != nil {
				t.Fatal(err)
			}
		}
		press := func(id string) {
			t.Helper()
			if !s.Focus(id) {
				t.Fatal("missing control", id)
			}
			if err := s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
				t.Fatal(err)
			}
			mount()
		}
		mount()
		press("demo/add")
		press("demo/add")
		for _, option := range languageOptions {
			press("demo/locale/trigger")
			press("demo/locale/option/" + option.ID)
			if d.Count != 2 || d.language.Language() != option.ID || d.Select.Value != option.ID || d.Select.Open {
				t.Fatalf("lost state or locale: %s", option.ID)
			}
			if _, err := s.Render(); err != nil {
				t.Fatal(err)
			}
		}
		for _, tc := range []struct{ locale, want string }{
			{"zh-Hans", "已选择 2 个文件"}, {"zh-Hant", "已選取 2 個檔案"}, {"de", "2 Dateien ausgewählt"},
		} {
			press("demo/locale/trigger")
			press("demo/locale/option/" + tc.locale)
			text, err := d.language.Text("files", map[string]any{"n": d.Count})
			if err != nil || text != tc.want {
				t.Fatal(text, err)
			}
		}
		press("demo/clear")
		if d.Count != 0 {
			t.Fatal("translated clear action failed")
		}
	}
}

func TestLanguageCatalogCoverageAndRegionalMatching(t *testing.T) {
	catalog, err := demoLanguages()
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range languageOptions {
		for _, key := range []string{"language", "heading", "files", "add", "reset", "amount"} {
			for _, n := range []int{0, 1, 2, 3, 11, 100} {
				value, err := catalog.Format(option.ID, key, map[string]any{"n": n})
				if err != nil || value == "" {
					t.Fatalf("%s/%s: %q %v", option.ID, key, value, err)
				}
			}
		}
	}
	for _, tc := range []struct{ region, want string }{
		{"zh-CN", "zh-Hans"}, {"zh-SG", "zh-Hans"}, {"zh-TW", "zh-Hant"}, {"zh-HK", "zh-Hant"}, {"ja-JP", "ja"},
	} {
		got, err := catalog.Resolve(tc.region)
		if err != nil || got != tc.want {
			t.Fatalf("%s: %s %v", tc.region, got, err)
		}
	}
	first, _ := catalog.Translator("en")
	second, _ := catalog.Translator("en")
	if err := first.SetLanguage("zh-CN"); err != nil {
		t.Fatal(err)
	}
	if second.Language() != "en" {
		t.Fatal("language state leaked between windows")
	}
}
