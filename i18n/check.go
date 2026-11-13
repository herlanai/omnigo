package i18n

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"golang.org/x/text/language"
)

// Diagnostic identifies one catalog problem. Missing translations and unused
// source arguments are warnings because the runtime supports source fallback.
type Diagnostic struct {
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message,omitempty"`
	Warning bool   `json:"warning"`
	Detail  string `json:"detail"`
}
type CheckReport struct {
	Languages   []string     `json:"languages"`
	Messages    int          `json:"messages"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	// Coverage makes the scope explicit; a clean report is not a guarantee that
	// every reference check (notably full CLDR cases) ran.
	Coverage []string `json:"coverage"`
}

func (r CheckReport) Errors() int {
	count := 0
	for _, d := range r.Diagnostics {
		if !d.Warning {
			count++
		}
	}
	return count
}

// Check collects message parse, key and argument-name diagnostics in stable
// order. It does not modify files or compile generated accessors.
func Check(fsys fs.FS, dir, source string) (CheckReport, error) {
	report := CheckReport{Languages: []string{}, Diagnostics: []Diagnostic{}, Coverage: []string{"message-syntax", "message-keys", "argument-names", "argument-types"}}
	sourceTag, err := language.Parse(source)
	if err != nil {
		return report, err
	}
	source = sourceTag.String()
	files, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return report, err
	}
	type checkedFile struct {
		file     string
		raw      map[string]string
		compiled map[string][]segment
	}
	catalogs := map[string]checkedFile{}
	var sourceData []byte
	locations := map[string]map[string]int{}
	add := func(file, id, detail string, warning bool) {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{File: file, Line: locations[file][id], Message: id, Warning: warning, Detail: detail})
	}
	for _, entry := range files {
		if entry.IsDir() || (path.Ext(entry.Name()) != ".json" && path.Ext(entry.Name()) != ".arb") {
			continue
		}
		file := path.Join(dir, entry.Name())
		name := strings.TrimSuffix(entry.Name(), path.Ext(entry.Name()))
		tag, err := language.Parse(name)
		if err != nil {
			add(file, "", "Invalid language tag", false)
			continue
		}
		locale := tag.String()
		if _, found := catalogs[locale]; found {
			add(file, "", "Duplicate normalized language: "+locale, false)
			continue
		}
		// Reserve the language even if decoding fails, so a second extension cannot
		// silently replace an invalid first catalog.
		catalogs[locale] = checkedFile{file: file}
		opened, err := fsys.Open(file)
		if err != nil {
			add(file, "", err.Error(), false)
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(opened, (8<<20)+1))
		closeErr := opened.Close()
		if readErr != nil {
			add(file, "", readErr.Error(), false)
			continue
		}
		if closeErr != nil {
			add(file, "", closeErr.Error(), false)
			continue
		}
		if len(data) > 8<<20 {
			add(file, "", "Catalog exceeds 8 MiB", false)
			continue
		}
		locations[file] = messageLines(data)
		raw, err := decodeCatalog(data, name, func(id string, line int, problem error) {
			add(file, id, problem.Error(), false)
			report.Diagnostics[len(report.Diagnostics)-1].Line = line
		})
		if err != nil {
			add(file, "", err.Error(), false)
			var located *catalogLocationError
			if errors.As(err, &located) {
				report.Diagnostics[len(report.Diagnostics)-1].Line = located.line
			}
			continue
		}
		parsed := map[string][]segment{}
		for _, id := range slices.Sorted(maps.Keys(raw)) {
			if id == "" {
				add(file, id, "Empty message key", false)
				continue
			}
			nodes, err := compile(raw[id])
			if err != nil {
				add(file, id, err.Error(), false)
				continue
			}
			parsed[id] = nodes
		}
		catalogs[locale] = checkedFile{file, raw, parsed}
		if locale == source {
			sourceData = data
		}
	}
	report.Languages = slices.Sorted(maps.Keys(catalogs))
	original, found := catalogs[source]
	if !found {
		add(path.Join(dir, source+".json"), "", "Source catalog missing", false)
	}
	if original.raw != nil {
		report.Messages = len(original.raw)
		contracts := sourceArgumentTypes(original.file, sourceData, original.raw, original.compiled, add)
		for _, locale := range report.Languages {
			if locale == source {
				continue
			}
			translation := catalogs[locale]
			if translation.raw == nil {
				continue
			}
			for _, id := range slices.Sorted(maps.Keys(translation.raw)) {
				if _, ok := original.raw[id]; !ok {
					add(translation.file, id, "Message is absent from source catalog", false)
					continue
				}
				_, sourceOK := original.compiled[id]
				nodes, ok := translation.compiled[id]
				if !ok || !sourceOK {
					continue
				}
				want, got := contracts[id], messageArguments(nodes)
				kinds := argumentKinds(nodes)
				for _, name := range slices.Sorted(maps.Keys(got)) {
					typ, known := want[name]
					if !known {
						add(translation.file, id, fmt.Sprintf("Unknown argument %q", name), false)
						continue
					}
					for _, kind := range slices.Sorted(maps.Keys(kinds[name])) {
						if !argumentAccepts(typ, kind) {
							add(translation.file, id, fmt.Sprintf("Argument %q of source type %s cannot use %s", name, typ, kind), false)
						}
					}
				}
				for _, name := range slices.Sorted(maps.Keys(want)) {
					if !got[name] {
						add(translation.file, id, fmt.Sprintf("Source argument %q is unused", name), true)
					}
				}
			}
			for _, id := range slices.Sorted(maps.Keys(original.raw)) {
				if _, ok := translation.raw[id]; !ok {
					add(translation.file, id, "Missing translation; runtime uses fallback", true)
				}
			}
		}
	}
	slices.SortFunc(report.Diagnostics, func(a, b Diagnostic) int {
		if n := strings.Compare(a.File, b.File); n != 0 {
			return n
		}
		if n := strings.Compare(a.Message, b.Message); n != 0 {
			return n
		}
		return strings.Compare(a.Detail, b.Detail)
	})
	return report, nil
}
func messageArguments(nodes []segment) map[string]bool {
	result := map[string]bool{}
	var visit func([]segment)
	visit = func(nodes []segment) {
		for _, node := range nodes {
			if node.name != "" {
				result[node.name] = true
			}
			for _, branch := range node.branches {
				visit(branch)
			}
		}
	}
	visit(nodes)
	return result
}
