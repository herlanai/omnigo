package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed audit-aliases.json
var auditAliases []byte

func audit(args []string) error {
	flags := flag.NewFlagSet("audit-ui", flag.ContinueOnError)
	source := flags.String("source", "", "read-only reference repository")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *source == "" {
		return errors.New("--source is required")
	}
	var aliases map[string]string
	if err := json.Unmarshal(auditAliases, &aliases); err != nil {
		return err
	}
	implementations := map[string]string{}
	paths, err := filepath.Glob("omniui/baseui/*.go")
	if err != nil {
		return err
	}
	exported := regexp.MustCompile(`(?m)^func ([A-Z]\w*)(?:\[.*?\])?\(`)
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		data, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		for _, m := range exported.FindAllStringSubmatch(string(data), -1) {
			implementations[m[1]] = filepath.ToSlash(p)
		}
	}
	data, err := os.ReadFile(filepath.Join(*source, "backend/ui/api_gen.go"))
	if err != nil {
		return err
	}
	constructor := regexp.MustCompile(`^func (\w+)(?:\[.*?\])?\(.*\) Element \{`)
	entries := []record{}
	pending := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		m := constructor.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := m[1]
		target := name
		if alias, ok := aliases[name]; ok {
			target = alias
		}
		entry := record{"name": name, "signature": strings.TrimSuffix(line, " {"), "reference": "backend/ui/api_gen.go", "implementation": nil, "symbol": nil, "status": "pending", "evidence": "Name/adapter mapping only; consult individual capability audit and tests."}
		if path, ok := implementations[target]; ok {
			entry["implementation"] = path
			entry["symbol"] = target
			entry["status"] = "partial"
		} else {
			pending++
		}
		entries = append(entries, entry)
	}
	dirs, err := os.ReadDir(filepath.Join(*source, "backend/ui"))
	if err != nil {
		return err
	}
	subpackages := []string{}
	for _, d := range dirs {
		if d.IsDir() && d.Name() != "testdata" {
			subpackages = append(subpackages, "backend/ui/"+d.Name())
		}
	}
	output := record{"schema": 1, "policy": "Complete constructor inventory is separate from parity. Existing implementations are reused, never counted complete by name. Subpackage capabilities remain in reference.json and advanced.json.", "constructors": entries, "subpackages": subpackages}
	if err = writeJSON("migration/controls/ui.json", output); err != nil {
		return err
	}
	fmt.Printf("%d UI constructors inventoried; %d have no mapped implementation.\n", len(entries), pending)
	return nil
}
