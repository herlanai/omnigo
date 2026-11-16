package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func licenses(ctx context.Context, destination string) error {
	if err := os.MkdirAll(destination, 0755); err != nil {
		return err
	}
	pattern := regexp.MustCompile(`(?i)^(licen[sc]e|copying|notice|ofl)(\.|$|-)`)
	copyLicenses := func(directory, label string) error {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Type().IsRegular() && pattern.MatchString(entry.Name()) {
				if err = copyTree(filepath.Join(directory, entry.Name()), filepath.Join(destination, strings.ReplaceAll(label, "/", "_")+"-"+entry.Name())); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, item := range [][2]string{{"omniui/plugins/chromium", "golet-chromium-port"}, {"omniui/icon/lucide", "lucide-static-1.71.0"}, {"omniui/baseui/fonts/inter", "inter-4.1"}, {"omniui/plugins/omnibrowser/render/fonts", "noto-sans-sc"}} {
		if err := copyLicenses(item[0], item[1]); err != nil {
			return err
		}
	}
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Path}}|{{.Dir}}", "all")
	cmd.Stderr = os.Stderr
	modules, err := cmd.Output()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(modules)), "\n")[1:] {
		name, dir, ok := strings.Cut(line, "|")
		if !ok || dir == "" {
			continue
		}
		if err = copyLicenses(dir, name); err != nil {
			return err
		}
		if name == "golang.org/x/image" {
			if err = copyLicenses(filepath.Join(dir, "font/gofont"), "go-fonts"); err != nil {
				return err
			}
		}
	}
	goroot, err := exec.CommandContext(ctx, "go", "env", "GOROOT").Output()
	if err != nil {
		return err
	}
	if err = copyLicenses(strings.TrimSpace(string(goroot)), "go"); err != nil {
		return err
	}
	entries, err := os.ReadDir("node_modules/.pnpm")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "node_modules" {
			continue
		}
		base := filepath.Join("node_modules/.pnpm", entry.Name(), "node_modules")
		packages, e := os.ReadDir(base)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		for _, pkg := range packages {
			if !pkg.IsDir() {
				continue
			}
			if strings.HasPrefix(pkg.Name(), "@") {
				scoped, e := os.ReadDir(filepath.Join(base, pkg.Name()))
				if e != nil {
					return e
				}
				for _, s := range scoped {
					if s.IsDir() {
						if e = copyLicenses(filepath.Join(base, pkg.Name(), s.Name()), entry.Name()+"-"+s.Name()); e != nil {
							return e
						}
					}
				}
			} else {
				if e = copyLicenses(filepath.Join(base, pkg.Name()), entry.Name()); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
