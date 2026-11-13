package i18n

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type diagnosticSink func(file, id, detail string, warning bool)

func argumentKinds(nodes []segment) map[string]map[string]bool {
	uses := map[string]map[string]bool{}
	var visit func([]segment)
	visit = func(nodes []segment) {
		for _, node := range nodes {
			if node.name != "" {
				if uses[node.name] == nil {
					uses[node.name] = map[string]bool{}
				}
				uses[node.name][node.kind] = true
			}
			for _, branch := range node.branches {
				visit(branch)
			}
		}
	}
	visit(nodes)
	return uses
}

// Plain interpolation accepts every declared type. Select needs text; number
// formatting and plural selection need numeric values. Object is intentionally
// unconstrained, as in ARB declarations.
func argumentAccepts(typ, kind string) bool {
	if typ == "Object" || kind == "argument" {
		return true
	}
	if kind == "select" {
		return typ == "String"
	}
	if kind == "number" || kind == "plural" || kind == "selectordinal" {
		return typ == "int" || typ == "double" || typ == "num"
	}
	return false
}
func knownArgumentType(typ string) bool {
	switch typ {
	case "", "String", "int", "double", "num", "DateTime", "Object":
		return true
	}
	return false
}

// Source declarations, including unused placeholders, define the contract for
// every translation. Translation metadata cannot widen that contract.
func sourceArgumentTypes(file string, data []byte, raw map[string]string, compiled map[string][]segment, add diagnosticSink) map[string]map[string]string {
	result := map[string]map[string]string{}
	_, fields := catalogLocations(data)
	for _, field := range slices.Sorted(maps.Keys(fields)) {
		if !strings.HasPrefix(field, "@") || strings.HasPrefix(field, "@@") {
			continue
		}
		if _, ok := raw[strings.TrimPrefix(field, "@")]; !ok {
			add(file, field, "Metadata describes no message", true)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(raw)) {
		declared := map[string]string{}
		if metadata, ok := fields["@"+id]; ok {
			var description struct {
				Description  string          `json:"description"`
				Placeholders json.RawMessage `json:"placeholders"`
			}
			if err := json.Unmarshal(metadata, &description); err != nil {
				add(file, "@"+id, "Invalid message metadata: "+err.Error(), false)
			} else {
				var err error
				declared, err = decodePlaceholderTypes(description.Placeholders)
				if err != nil {
					add(file, "@"+id, "Invalid placeholders: "+err.Error(), false)
				}
			}
		}
		uses := argumentKinds(compiled[id])
		contract := map[string]string{}
		for name := range uses {
			contract[name] = ""
		}
		for name, typ := range declared {
			if !knownArgumentType(typ) {
				add(file, "@"+id, fmt.Sprintf("Placeholder %q has unknown type %q", name, typ), false)
				typ = ""
			}
			contract[name] = typ
			if _, parsed := compiled[id]; parsed && uses[name] == nil {
				add(file, "@"+id, fmt.Sprintf("Declared placeholder %q is unused by source", name), true)
			}
		}
		for _, name := range slices.Sorted(maps.Keys(contract)) {
			typ := contract[name]
			if typ == "" {
				typ = "String"
				for kind := range uses[name] {
					if kind == "number" || kind == "plural" || kind == "selectordinal" {
						typ = "int"
					}
				}
				contract[name] = typ
			}
			for _, kind := range slices.Sorted(maps.Keys(uses[name])) {
				if !argumentAccepts(typ, kind) {
					add(file, id, fmt.Sprintf("Argument %q of type %s cannot use %s", name, typ, kind), false)
				}
			}
		}
		result[id] = contract
	}
	return result
}

func decodePlaceholderTypes(raw json.RawMessage) (map[string]string, error) {
	result := map[string]string{}
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return result, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return result, fmt.Errorf("expected an object")
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return result, err
		}
		name, ok := token.(string)
		if !ok {
			return result, fmt.Errorf("invalid placeholder name")
		}
		if _, ok := result[name]; ok {
			return result, fmt.Errorf("duplicate placeholder %q", name)
		}
		var descriptor struct {
			Type string `json:"type"`
		}
		if err := decoder.Decode(&descriptor); err != nil {
			return result, err
		}
		result[name] = descriptor.Type
	}
	_, err = decoder.Token()
	return result, err
}
