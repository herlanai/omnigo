package appbuild

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPackageRejectsExistingRelease(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS package entry")
	}
	dir := t.TempDir()
	release := filepath.Join(dir, "release")
	os.Mkdir(release, 0755)
	marker := filepath.Join(release, "old")
	os.WriteFile(marker, []byte("keep"), 0600)
	config := filepath.Join(dir, "omnigo.json")
	os.WriteFile(config, []byte(`{"package":".","output":"app","goos":"darwin","goarch":"arm64","bundle":{"name":"Test","id":"com.example.test","version":"1.0.0","icon":"missing.icns","licenses":["LICENSE"],"destination":"release"}}`), 0600)
	if _, err := PackageMac(context.Background(), config, io.Discard); err == nil {
		t.Fatal("overwrote existing release")
	}
	got, _ := os.ReadFile(marker)
	if string(got) != "keep" {
		t.Fatal("old artifact modified")
	}
}
func TestCopyRejectsSymlinkAndExistingTarget(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	os.WriteFile(a, []byte("a"), 0600)
	if err := copyRegular(a, a, 0600); err == nil {
		t.Fatal("same file accepted")
	}
	b := filepath.Join(dir, "b")
	if err := os.Symlink(a, b); err != nil {
		t.Skip(err)
	}
	if err := copyRegular(b, filepath.Join(dir, "c"), 0600); err == nil {
		t.Fatal("symlink accepted")
	}
}
