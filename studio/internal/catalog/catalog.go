package catalog

import (
	_ "embed"
	"encoding/json"
	"omnigo.dev/omnigo/studio/internal/demos"
	"strings"
)

//go:embed catalog.json
var data []byte

type Entry struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Group          string  `json:"group"`
	Kind           string  `json:"kind"`
	Status         string  `json:"status"`
	Implementation *string `json:"implementation"`
	Symbol         *string `json:"symbol"`
	Demo           string  `json:"demo,omitempty"`
}

func Entries() []Entry {
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		panic(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Title == "" {
			e.Title = strings.ReplaceAll(strings.TrimPrefix(e.ID, "plugin/"), "-", " ")
		}
		id := e.ID
		// Plugin demos must be explicitly mapped: equal names are not equal capabilities.
		if e.Kind == "plugin" && e.ID == "plugin/loading" {
			id = "loading"
		}
		if demos.Supported(id) || id == "plugin/chromium" || id == "web-view" {
			e.Demo = id
		}
		if e.Description == "" {
			if e.Status == "pending" {
				e.Description = "已纳入完整迁移目录，Go 重写尚未实现。"
			} else {
				e.Description = "提供局部 Go 实现与交互演示，尚未完成全部行为对齐。"
			}
		}
	}
	return entries
}
