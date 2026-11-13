package demos

import (
	"sync"

	"github.com/herlanai/omnigo/i18n"
	ui "github.com/herlanai/omnigo/omniui/baseui"
)

var languageOptions = []ui.Option{
	{ID: "en", Label: "English"}, {ID: "zh-Hans", Label: "简体中文"}, {ID: "zh-Hant", Label: "繁體中文"},
	{ID: "de", Label: "Deutsch"}, {ID: "fr", Label: "Français"}, {ID: "ja", Label: "日本語"},
	{ID: "ar", Label: "العربية"}, {ID: "he", Label: "עברית"}, {ID: "hi", Label: "हिन्दी"},
}

// Compile application messages once; each window owns its selected language.
var demoLanguages = sync.OnceValues(func() (*i18n.Catalog, error) {
	return i18n.New("en", map[string]map[string]string{
		"en":      {"language": "Language", "heading": "Your workspace", "files": "{n, plural, =0 {No files selected} one {# file selected} other {# files selected}}", "add": "Add a file", "reset": "Clear selection", "amount": "Storage: {n, number, ::.00} MB"},
		"zh-Hans": {"language": "语言", "heading": "你的工作空间", "files": "{n, plural, =0 {未选择文件} other {已选择 # 个文件}}", "add": "添加文件", "reset": "清除选择", "amount": "存储空间：{n, number, ::.00} MB"},
		"zh-Hant": {"language": "語言", "heading": "你的工作空間", "files": "{n, plural, =0 {未選取檔案} other {已選取 # 個檔案}}", "add": "新增檔案", "reset": "清除選取", "amount": "儲存空間：{n, number, ::.00} MB"},
		"de":      {"language": "Sprache", "heading": "Dein Arbeitsbereich", "files": "{n, plural, =0 {Keine Dateien ausgewählt} one {# Datei ausgewählt} other {# Dateien ausgewählt}}", "add": "Datei hinzufügen", "reset": "Auswahl löschen", "amount": "Speicher: {n, number, ::.00} MB"},
		"fr":      {"language": "Langue", "heading": "Votre espace de travail", "files": "{n, plural, =0 {Aucun fichier sélectionné} one {# fichier sélectionné} other {# fichiers sélectionnés}}", "add": "Ajouter un fichier", "reset": "Effacer la sélection", "amount": "Stockage : {n, number, ::.00} Mo"},
		"ja":      {"language": "言語", "heading": "ワークスペース", "files": "{n, plural, =0 {ファイルが選択されていません} other {# 個のファイルを選択中}}", "add": "ファイルを追加", "reset": "選択を解除", "amount": "ストレージ：{n, number, ::.00} MB"},
		"ar":      {"language": "اللغة", "heading": "مساحة العمل", "files": "{n, plural, zero {لم يتم تحديد ملفات} one {تم تحديد ملف واحد} two {تم تحديد ملفين} few {تم تحديد # ملفات} many {تم تحديد # ملفًا} other {تم تحديد # ملف}}", "add": "إضافة ملف", "reset": "مسح التحديد", "amount": "التخزين: {n, number, ::.00} MB"},
		"he":      {"language": "שפה", "heading": "סביבת העבודה שלך", "files": "{n, plural, =0 {לא נבחרו קבצים} one {נבחר קובץ אחד} other {נבחרו # קבצים}}", "add": "הוספת קובץ", "reset": "ניקוי הבחירה", "amount": "אחסון: {n, number, ::.00} MB"},
		"hi":      {"language": "भाषा", "heading": "आपका कार्यक्षेत्र", "files": "{n, plural, =0 {कोई फ़ाइल नहीं चुनी गई} one {# फ़ाइल चुनी गई} other {# फ़ाइलें चुनी गईं}}", "add": "फ़ाइल जोड़ें", "reset": "चयन हटाएँ", "amount": "स्टोरेज: {n, number, ::.00} MB"},
	})
})

func (d *Demo) languageContent() *ui.Element {
	if d.language == nil {
		catalog, err := demoLanguages()
		if err != nil {
			return ui.Text("demo/error", err.Error())
		}
		d.language, err = catalog.Translator("en")
		if err != nil {
			return ui.Text("demo/error", err.Error())
		}
	}
	text := func(id string, args map[string]any) string {
		s, err := d.language.Text(id, args)
		if err != nil {
			return err.Error()
		}
		return s
	}
	d.Select.Value = d.language.Language()
	selector := ui.Select("demo/locale", text("language", nil), d.Select, languageOptions, func(lang string) {
		if err := d.language.SetLanguage(lang); err != nil {
			d.Message = err.Error()
		}
	})
	direction := ui.LeftToRight
	if d.language.RTL() {
		direction = ui.RightToLeft
	}
	content := ui.Direction("demo/translated", direction,
		ui.Heading("demo/heading", text("heading", nil), 2),
		ui.Text("demo/files", text("files", map[string]any{"n": d.Count})),
		ui.Text("demo/amount", text("amount", map[string]any{"n": 1234.5})),
		ui.Row("demo/actions", ui.Button("demo/add", text("add", nil), func() { d.Count++ }), ui.Button("demo/clear", text("reset", nil), func() { d.Count = 0 })).Gap(8),
	).Gap(16)
	children := []*ui.Element{ui.Label("demo/locale-label", text("language", nil)), selector, content}
	switch d.language.Language() {
	case "ar", "he", "hi":
		children = append(children, ui.Text("demo/shaping-note", "Translation and number formatting are available. Complex-script shaping and Unicode bidirectional text layout are not yet supported.").Muted(true))
	}
	return ui.Column("demo/languages", children...).Gap(16)
}
