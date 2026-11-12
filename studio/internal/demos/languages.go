package demos

import (
	"github.com/herlanai/omnigo/i18n"
	ui "github.com/herlanai/omnigo/omniui/baseui"
)

func (d *Demo) languageContent() *ui.Element {
	if d.language == nil {
		catalog, err := i18n.New("en", map[string]map[string]string{
			"en": {"heading": "Your workspace", "files": "{n, plural, =0 {No files selected} one {# file selected} other {# files selected}}", "add": "Add a file", "reset": "Clear selection", "amount": "Storage: {n, number, ::.00} MB"},
			"de": {"heading": "Dein Arbeitsbereich", "files": "{n, plural, =0 {Keine Dateien ausgewählt} one {# Datei ausgewählt} other {# Dateien ausgewählt}}", "add": "Datei hinzufügen", "reset": "Auswahl löschen", "amount": "Speicher: {n, number, ::.00} MB"},
			"fr": {"heading": "Votre espace de travail", "files": "{n, plural, =0 {Aucun fichier sélectionné} one {# fichier sélectionné} other {# fichiers sélectionnés}}", "add": "Ajouter un fichier", "reset": "Effacer la sélection", "amount": "Stockage : {n, number, ::.00} Mo"},
		})
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
	choose := func(lang string) func() {
		return func() {
			if err := d.language.SetLanguage(lang); err != nil {
				d.Message = err.Error()
			}
		}
	}
	return ui.Column("demo/languages",
		ui.Row("demo/locales", ui.Button("demo/en", "English", choose("en")), ui.Button("demo/de", "Deutsch", choose("de")), ui.Button("demo/fr", "Français", choose("fr"))).Gap(8),
		ui.Text("demo/heading", text("heading", nil)), ui.Text("demo/files", text("files", map[string]any{"n": d.Count})), ui.Text("demo/amount", text("amount", map[string]any{"n": 1234.5})),
		ui.Row("demo/actions", ui.Button("demo/add", text("add", nil), func() { d.Count++ }), ui.Button("demo/clear", text("reset", nil), func() { d.Count = 0 })).Gap(8),
	).Gap(16)
}
