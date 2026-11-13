package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestPartitionScope(t *testing.T) {
	for path, want := range map[string]string{"backend/ui/input.go": "deferred-controls", "backend/plugins/dsh-go/native/panel.go": "deferred-controls", "backend/plugins/dsh-go/conversation.go": "plugins", "backend/plugins/terminal/screen.go": "plugins", "backend/cmd/golet/template/shadcn/frontend/page.tsx": "toolchain", "frontend/packages/runtime/index.ts": "omniweb", "backend/paths.go": "desktop", "backend/app.go": "lifecycle", "backend/internal/platform/platform.go": "desktop"} {
		if got := classify(path); got != want {
			t.Errorf("%s: %s != %s", path, got, want)
		}
	}
}
func TestCommittedAuditCoverage(t *testing.T) {
	raw, err := os.ReadFile("../../migration/noncontrols/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var inv Inventory
	if err = json.Unmarshal(raw, &inv); err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	for _, file := range append(inv.Files, inv.Deferred...) {
		if files[file.Path] || len(file.SHA256) != 64 || file.Group != classify(file.Path) {
			t.Fatalf("invalid or duplicate file %+v", file)
		}
		files[file.Path] = true
	}
	for _, api := range inv.API {
		if !files[api.File] || api.Group == "deferred-controls" || api.Group != classify(api.File) {
			t.Fatalf("untracked API %s", api.ID)
		}
	}
	raw, err = os.ReadFile("../../migration/noncontrols/status.json")
	if err != nil {
		t.Fatal(err)
	}
	var status struct {
		Items []struct {
			ID, Group, Status                   string
			Implementation, Evidence, Remaining []string
		}
	}
	if err = json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	groups := map[string]bool{}
	ids := map[string]bool{}
	for _, item := range status.Items {
		if ids[item.ID] {
			t.Fatal("duplicate", item.ID)
		}
		ids[item.ID] = true
		groups[item.Group] = true
		if item.Status != "pending" && item.Status != "partial" && item.Status != "verified" {
			t.Fatal("invalid status", item.ID)
		}
		if item.Status == "verified" && (len(item.Evidence) == 0 || len(item.Remaining) > 0) {
			t.Fatal("unsupported completion", item.ID)
		}
		for _, path := range append(item.Implementation, item.Evidence...) {
			if _, err := os.Stat("../../" + path); err != nil {
				t.Fatal(item.ID, path, err)
			}
		}
	}
	for _, file := range inv.Files {
		if !groups[file.Group] {
			t.Fatal("untracked group", file.Group)
		}
	}
}
