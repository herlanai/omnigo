package catalog

import (
	"encoding/json"
	"omnigo.dev/omnigo/studio/internal/demos"
	"os"
	"testing"
)

func TestCatalogTracksEveryReferenceEntryAndPlugin(t *testing.T) {
	entries := Entries()
	seen := map[string]Entry{}
	for _, e := range entries {
		if _, ok := seen[e.ID]; ok {
			t.Fatalf("duplicate %s", e.ID)
		}
		seen[e.ID] = e
		if e.Demo != "" && e.Demo != "plugin/chromium" && e.Demo != "web-view" && !demos.Supported(e.Demo) {
			t.Fatalf("invalid demo %s", e.ID)
		}
	}
	var ref struct {
		Catalog []struct {
			ID string `json:"id"`
		} `json:"catalog"`
	}
	read(t, "../../../migration/controls/reference.json", &ref)
	var status struct {
		Controls []Entry `json:"controls"`
	}
	read(t, "../../../migration/controls/status.json", &status)
	for _, e := range ref.Catalog {
		if _, ok := seen[e.ID]; !ok {
			t.Errorf("missing control %s", e.ID)
		}
	}
	for _, e := range status.Controls {
		if seen[e.ID].Status != e.Status {
			t.Errorf("stale status %s; run pnpm studio:catalog", e.ID)
		}
		if e.Status == "partial" && seen[e.ID].Demo == "" {
			t.Errorf("partial control has no executable demo: %s", e.ID)
		}
	}
	var baseline struct {
		Plugins []string `json:"plugins"`
	}
	read(t, "../../../migration/reference.json", &baseline)
	for _, id := range baseline.Plugins {
		if _, ok := seen["plugin/"+id]; !ok {
			t.Errorf("missing plugin %s", id)
		}
	}
}
func read(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

func TestPluginsNeverBorrowSameNamedControlDemo(t *testing.T) {
	for _, e := range Entries() {
		if e.Kind == "plugin" && e.ID != "plugin/loading" && e.Demo != "" && e.Demo != e.ID {
			t.Errorf("unimplemented plugin %s borrows %s", e.ID, e.Demo)
		}
		if e.ID == "terminal" && e.Demo != "terminal" {
			t.Fatal("missing Forma Terminal demo")
		}
	}
}

func TestAdvancedControlsRetainTheirOwnAudit(t *testing.T) {
	var audit struct {
		Controls []Entry `json:"controls"`
	}
	read(t, "../../../migration/controls/advanced.json", &audit)
	entries := map[string]Entry{}
	for _, entry := range Entries() {
		entries[entry.ID] = entry
	}
	for _, control := range audit.Controls {
		entry, ok := entries[control.ID]
		if !ok || entry.Status != control.Status {
			t.Errorf("missing or stale advanced control %s", control.ID)
		}
		if control.Status == "pending" && entry.Demo != "" {
			t.Errorf("pending control borrows a demo: %s", control.ID)
		}
	}
}
