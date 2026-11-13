package system

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// PathName identifies a standard application or user location.
type PathName string

const (
	PathHome      PathName = "home"
	PathAppData   PathName = "appData"
	PathUserData  PathName = "userData"
	PathCache     PathName = "cache"
	PathLogs      PathName = "logs"
	PathResources PathName = "resources"
	PathTemp      PathName = "temp"
	PathExe       PathName = "exe"
	PathDesktop   PathName = "desktop"
	PathDocuments PathName = "documents"
	PathDownloads PathName = "downloads"
	PathMusic     PathName = "music"
	PathPictures  PathName = "pictures"
	PathVideos    PathName = "videos"
)

var folders = map[PathName]string{PathDesktop: "Desktop", PathDocuments: "Documents", PathDownloads: "Downloads", PathMusic: "Music", PathPictures: "Pictures", PathVideos: "Videos"}

// Paths resolves application-scoped paths independently of a GUI lifecycle.
// It is safe for concurrent use. Only userData/cache/logs defaults are created;
// overrides and user folders are never implicitly created.
type Paths struct {
	name      string
	mu        sync.RWMutex
	overrides map[PathName]string
}

func NewPaths(name string) (*Paths, error) {
	if name == "" || name == "." || name == ".." || strings.TrimSpace(name) != name || strings.ContainsAny(name, "/\\\x00<>:\"|?*") || strings.ContainsAny(name, "\r\n") || strings.HasSuffix(name, ".") {
		return nil, errors.New("system: application name must be a single path component")
	}
	for _, r := range name {
		if r < 32 {
			return nil, errors.New("system: control character in application name")
		}
	}
	device := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	reserved := device == "CON" || device == "PRN" || device == "AUX" || device == "NUL"
	if len(device) == 4 && (strings.HasPrefix(device, "COM") || strings.HasPrefix(device, "LPT")) && device[3] >= '1' && device[3] <= '9' {
		reserved = true
	}
	if reserved {
		return nil, errors.New("system: reserved application path component")
	}
	return &Paths{name: name, overrides: map[PathName]string{}}, nil
}
func knownPath(name PathName) bool {
	switch name {
	case PathHome, PathAppData, PathUserData, PathCache, PathLogs, PathResources, PathTemp, PathExe:
		return true
	}
	_, ok := folders[name]
	return ok
}

// Set overrides a path with an absolute snapshot. Empty clears the override.
// It does not touch the filesystem, and unknown names leave the mapping intact.
func (p *Paths) Set(name PathName, path string) error {
	if !knownPath(name) {
		return fmt.Errorf("system: unknown path %q", name)
	}
	if strings.ContainsRune(path, 0) {
		return errors.New("system: invalid path")
	}
	if path != "" {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		path = absolute
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if path == "" {
		delete(p.overrides, name)
	} else {
		p.overrides[name] = path
	}
	return nil
}
func (p *Paths) Get(name PathName) (string, error) {
	if !knownPath(name) {
		return "", fmt.Errorf("system: unknown path %q", name)
	}
	p.mu.RLock()
	override := p.overrides[name]
	p.mu.RUnlock()
	if override != "" {
		return override, nil
	}
	var path string
	var err error
	create := false
	switch name {
	case PathHome:
		path, err = os.UserHomeDir()
	case PathAppData:
		path, err = os.UserConfigDir()
	case PathExe:
		path, err = os.Executable()
	case PathTemp:
		path = os.TempDir()
	case PathUserData, PathCache, PathLogs:
		var base string
		if name == PathCache {
			base, err = os.UserCacheDir()
		} else if name == PathLogs && runtime.GOOS == "darwin" {
			base, err = os.UserHomeDir()
			base = filepath.Join(base, "Library", "Logs")
		} else {
			base, err = os.UserConfigDir()
		}
		path = filepath.Join(base, p.name)
		if name == PathLogs && runtime.GOOS != "darwin" {
			path = filepath.Join(path, "logs")
		}
		create = true
	case PathResources:
		var executable string
		executable, err = os.Executable()
		if err == nil {
			if resolved, e := filepath.EvalSymlinks(executable); e == nil {
				executable = resolved
			}
			path, err = resourceDirectory(executable, os.Getwd)
		}
	default:
		path, err = userFolder(name)
	}
	if err != nil {
		return "", err
	}
	if path == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("system: path %q did not resolve to an absolute location", name)
	}
	if create {
		if err = os.MkdirAll(path, 0700); err != nil {
			return "", err
		}
	}
	return filepath.Clean(path), nil
}

func resourceDirectory(executable string, wd func() (string, error)) (string, error) {
	parent := filepath.Dir(executable)
	if filepath.Base(parent) == "MacOS" && filepath.Base(filepath.Dir(parent)) == "Contents" {
		return filepath.Join(filepath.Dir(parent), "Resources"), nil
	}
	cursor := parent
	for i := 0; i < 3; i++ {
		if strings.HasPrefix(filepath.Base(cursor), "go-build") {
			dir, err := wd()
			if err != nil {
				return "", err
			}
			return filepath.Join(dir, "resources"), nil
		}
		cursor = filepath.Dir(cursor)
	}
	return parent, nil
}

// xdgFolder parses the quoted user-dirs format without evaluating shell code.
// Only a leading $HOME or ${HOME} is expanded; malformed or relative values are
// ignored, preserving the standard-folder fallback.
func xdgFolder(r io.Reader, name PathName, home string) string {
	key := "XDG_" + strings.ToUpper(string(name)) + "_DIR"
	scanner := bufio.NewScanner(io.LimitReader(r, 1<<20))
	scanner.Buffer(make([]byte, 4096), 64<<10)
	var selected string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		left, right, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(left) != key {
			continue
		}
		raw := strings.TrimSpace(right)
		if len(raw) < 2 || raw[0] != '"' {
			continue
		}
		var value strings.Builder
		end := -1
		for i := 1; i < len(raw); i++ {
			if raw[i] == '"' {
				end = i
				break
			}
			if raw[i] == '\\' && i+1 < len(raw) {
				i++
				if !strings.ContainsRune("\\\"$`", rune(raw[i])) {
					value.WriteByte('\\')
				}
			}
			value.WriteByte(raw[i])
		}
		if end < 0 {
			continue
		}
		tail := strings.TrimSpace(raw[end+1:])
		if tail != "" && !strings.HasPrefix(tail, "#") {
			continue
		}
		path := value.String()
		for _, prefix := range []string{"$HOME", "${HOME}"} {
			if strings.HasPrefix(raw, `"`+prefix+`/`) || strings.HasPrefix(raw, `"`+prefix+`"`) {
				path = home + strings.TrimPrefix(path, prefix)
				break
			}
		}
		if filepath.IsAbs(path) && !strings.ContainsRune(path, 0) {
			selected = filepath.Clean(path)
		}
	}
	return selected
}
