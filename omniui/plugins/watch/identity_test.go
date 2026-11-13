package watch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIdentityIndexMatchesConservativePairing(t *testing.T) {
	dir := t.TempDir()
	before, after, created := map[string]fs.FileInfo{}, map[string]fs.FileInfo{}, map[string]fs.FileInfo{}
	for i := 0; i < 64; i++ {
		name := fmt.Sprintf("file-%02d", i)
		full := filepath.Join(dir, name)
		mustDo(t, os.WriteFile(full, nil, 0600))
		info, err := os.Lstat(full)
		mustDo(t, err)
		if _, ok := fileIdentity(info); !ok {
			t.Skip("native identity index unavailable on this platform")
		}
		before[name] = info
		after["moved/"+name] = info
		created["moved/"+name] = info
	}
	// An alias that remains in place makes this identity ambiguous.
	before["retained"] = before["file-00"]
	after["retained"] = before["file-00"]
	index, err := identityIndex(t.Context(), before, after, created)
	mustDo(t, err)
	if index == nil {
		t.Fatal("missing complete index")
	}
	for name, info := range before {
		got, want := uniqueDestination(info, index), compareDestination(info, before, after, created)
		if got != want {
			t.Fatal(name, got, want)
		}
	}
	if uniqueDestination(before["file-00"], index) != "" {
		t.Fatal("hard-link ambiguity ignored")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err = identityIndex(ctx, before, after, created); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

type unknownInfo struct{}

func (unknownInfo) Name() string       { return "unknown" }
func (unknownInfo) Size() int64        { return 0 }
func (unknownInfo) Mode() fs.FileMode  { return 0600 }
func (unknownInfo) ModTime() time.Time { return time.Time{} }
func (unknownInfo) IsDir() bool        { return false }
func (unknownInfo) Sys() any           { return nil }
func TestUnknownIdentityCannotProducePartialIndex(t *testing.T) {
	index, err := identityIndex(t.Context(), map[string]fs.FileInfo{"unknown": unknownInfo{}}, nil, nil)
	if err != nil || index != nil {
		t.Fatal(index, err)
	}
}
func BenchmarkSnapshotDiffRenames(b *testing.B) {
	for _, size := range []int{1000, 4000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			dir := b.TempDir()
			before, after := map[string]fs.FileInfo{}, map[string]fs.FileInfo{}
			for i := 0; i < size; i++ {
				name := fmt.Sprint(i)
				full := filepath.Join(dir, name)
				if err := os.WriteFile(full, nil, 0600); err != nil {
					b.Fatal(err)
				}
				info, err := os.Lstat(full)
				if err != nil {
					b.Fatal(err)
				}
				before["old/"+name] = info
				after["new/"+name] = info
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				events, err := snapshotDiff(context.Background(), before, after)
				if err != nil || len(events) != size {
					b.Fatal(len(events), err)
				}
			}
		})
	}
}
