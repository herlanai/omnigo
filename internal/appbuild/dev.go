package appbuild

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// DevOptions names source paths explicitly; generated outputs must not be watched.
type DevOptions struct {
	Watch []string `json:"watch"`
	Args  []string `json:"args,omitempty"`
}

type devChild struct {
	done <-chan error
	stop func()
}

// runDev serializes rebuilds. A failed rebuild keeps the previous child alive.
// Changes arriving during a build remain queued for a subsequent build.
func runDev(ctx context.Context, changes <-chan struct{}, build func() (string, error), start func(string) (devChild, error), output io.Writer) error {
	var child devChild
	defer func() {
		if child.stop != nil {
			child.stop()
		}
	}()
	rebuild := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		binary, err := build()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Fprintf(output, "Build failed: %v\n", err)
			return nil
		}
		if child.stop != nil {
			child.stop()
			child = devChild{}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		child, err = start(binary)
		return err
	}
	if err := rebuild(); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-child.done:
			if err != nil {
				return fmt.Errorf("application exited: %w", err)
			}
			return nil
		case _, ok := <-changes:
			if !ok {
				return nil
			}
			if err := rebuild(); err != nil {
				return err
			}
		}
	}
}

func fingerprint(paths []string) ([32]byte, error) {
	return fingerprintContext(context.Background(), paths)
}

func fingerprintContext(ctx context.Context, paths []string) ([32]byte, error) {
	h := sha256.New()
	files := []string{}
	for _, root := range paths {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if e := ctx.Err(); e != nil {
				return e
			}
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if d.IsDir() {
				if p != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Type().IsRegular() {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return [32]byte{}, err
		}
	}
	sort.Strings(files)
	buffer := make([]byte, 32<<10)
	for _, p := range files {
		if err := ctx.Err(); err != nil {
			return [32]byte{}, err
		}
		file, err := os.Open(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return [32]byte{}, err
		}
		stat, err := file.Stat()
		if err != nil {
			file.Close()
			return [32]byte{}, err
		}
		fmt.Fprintf(h, "%d:%s:%d:", len(p), p, stat.Size())
		for {
			if err := ctx.Err(); err != nil {
				file.Close()
				return [32]byte{}, err
			}
			n, e := file.Read(buffer)
			h.Write(buffer[:n])
			if e == io.EOF {
				break
			}
			if e != nil {
				file.Close()
				return [32]byte{}, e
			}
		}
		if err := file.Close(); err != nil {
			return [32]byte{}, err
		}
	}

	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

func Dev(ctx context.Context, config string, output io.Writer) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("appbuild: dev process-tree cleanup is not implemented on Windows")
	}
	absolute, err := filepath.Abs(config)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(absolute)
	if err != nil {
		return err
	}
	var c Config
	if err = json.Unmarshal(raw, &c); err != nil {
		return err
	}
	if c.GOOS != runtime.GOOS || c.GOARCH != runtime.GOARCH {
		return fmt.Errorf("appbuild: dev requires a host-native target")
	}
	if c.Dev == nil || len(c.Dev.Watch) == 0 {
		return fmt.Errorf("appbuild: dev.watch must explicitly list source paths, excluding generated outputs")
	}
	dir := filepath.Dir(absolute)
	paths := []string{absolute}
	for _, p := range c.Dev.Watch {
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		paths = append(paths, p)
	}
	baseline, err := fingerprintContext(ctx, paths)
	if err != nil {
		return err
	}
	changes := make(chan struct{}, 1)
	watchErr := make(chan error, 1)
	lifetime, cancel := context.WithCancel(ctx)
	defer cancel()
	finished := make(chan struct{})
	defer func() { cancel(); <-finished }()
	go func() {
		defer close(finished)
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-lifetime.Done():
				return
			case <-ticker.C:
				next, e := fingerprintContext(lifetime, paths)
				if e != nil {
					watchErr <- e
					cancel()
					return
				}
				if next != baseline {
					baseline = next
					select {
					case changes <- struct{}{}:
					default:
					}
				}
			}
		}
	}()
	err = runDev(lifetime, changes, func() (string, error) { return Build(lifetime, absolute, output, nil) }, func(binary string) (devChild, error) { return launchDev(lifetime, dir, binary, c.Dev.Args, output) }, output)
	select {
	case e := <-watchErr:
		return e
	default:
		return err
	}
}
