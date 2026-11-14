package appbuild

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestDevFailedRebuildKeepsChildAndCleansUp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changes := make(chan struct{}, 1)
	calls, starts, stops := 0, 0, 0
	err := runDev(ctx, changes, func() (string, error) {
		calls++
		switch calls {
		case 1:
			return "binary", nil
		case 2:
			if stops != 0 {
				t.Fatal("stopped healthy child before build")
			}
			changes <- struct{}{}
			return "", errors.New("compile")
		default:
			return "binary", nil
		}
	}, func(string) (devChild, error) {
		starts++
		if starts == 1 {
			changes <- struct{}{}
		} else {
			cancel()
		}
		return devChild{done: make(chan error), stop: func() { stops++ }}, nil
	}, io.Discard)
	if !errors.Is(err, context.Canceled) || starts != 2 || stops != 2 || calls != 3 {
		t.Fatal(err, starts, stops, calls)
	}
}
func TestDevFingerprintDetectsContentAndDeletion(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "source")
	os.WriteFile(p, []byte("old"), 0600)
	a, e := fingerprint([]string{dir})
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(p, []byte("new"), 0600)
	b, _ := fingerprint([]string{dir})
	if a == b {
		t.Fatal("same-size edit missed")
	}
	os.Remove(p)
	c, _ := fingerprint([]string{dir})
	if b == c {
		t.Fatal("delete missed")
	}
}

func TestDevCancellationDuringBuildNeverStartsApplication(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := runDev(ctx, make(chan struct{}), func() (string, error) { cancel(); return "binary", nil }, func(string) (devChild, error) { t.Fatal("started after cancellation"); return devChild{}, nil }, io.Discard)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestFingerprintHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fingerprintContext(ctx, []string{t.TempDir()}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
