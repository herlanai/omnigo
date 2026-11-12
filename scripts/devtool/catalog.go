package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed catalog-extra.json
var catalogExtra []byte

type record map[string]any

func catalog() error {
	var reference struct {
		Catalog []record `json:"catalog"`
	}
	var status, advanced struct {
		Controls []record `json:"controls"`
	}
	var baseline struct {
		Plugins []string `json:"plugins"`
	}
	var plugins struct {
		Plugins []record `json:"plugins"`
	}
	for _, r := range []struct {
		path  string
		value any
	}{
		{"migration/controls/reference.json", &reference}, {"migration/controls/status.json", &status},
		{"migration/reference.json", &baseline}, {"omniui/plugins/status.json", &plugins}, {"migration/controls/advanced.json", &advanced},
	} {
		if err := readJSON(r.path, r.value); err != nil {
			return err
		}
	}
	entries := []record{}
	for _, c := range reference.Catalog {
		for _, s := range status.Controls {
			if s["id"] == c["id"] {
				for k, v := range s {
					c[k] = v
				}
				break
			}
		}
		c["kind"] = "control"
		entries = append(entries, c)
	}
	for _, c := range advanced.Controls {
		c["group"] = "advanced"
		c["kind"] = "control"
		entries = append(entries, c)
	}
	for _, id := range baseline.Plugins {
		var plugin record
		for _, p := range plugins.Plugins {
			if p["id"] == id {
				plugin = p
				break
			}
		}
		if plugin == nil {
			return fmt.Errorf("missing plugin audit: %s", id)
		}
		plugin["id"] = "plugin/" + id
		plugin["group"] = "plugins"
		plugin["kind"] = "plugin"
		plugin["description"] = "已实现：" + joinItems(plugin["implemented"]) + "。尚缺：" + joinItems(plugin["remaining"])
		entries = append(entries, plugin)
	}
	var extra []record
	if err := json.Unmarshal(catalogExtra, &extra); err != nil {
		return err
	}
	entries = append(entries, extra...)
	return writeJSON("examples/studio/internal/catalog/catalog.json", entries)
}
func joinItems(value any) string {
	items, _ := value.([]any)
	s := make([]string, len(items))
	for i, v := range items {
		s[i], _ = v.(string)
	}
	return strings.Join(s, "；")
}
