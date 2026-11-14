package mobilebuild

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestInvalidTargetNeverCreatesOutput(t *testing.T) {
	for _, target := range []Target{{"ios", "amd64"}, {"darwin", "arm64"}, {"android", "386"}} {
		dir := t.TempDir()
		_, err := Build(t.Context(), dir, Config{Package: "./app", Output: "output", Name: "App", ID: "com.example.app", Version: "1.0"}, target, io.Discard)
		if err == nil {
			t.Fatal(target)
		}
		if _, err := os.Stat(filepath.Join(dir, "output")); !os.IsNotExist(err) {
			t.Fatal("invalid target created output")
		}
	}
}
func TestBuildNeverOverwritesExistingRelease(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "output", "android-arm64")
	os.MkdirAll(dest, 0755)
	os.WriteFile(filepath.Join(dest, "owned"), []byte("old"), 0600)
	_, err := Build(t.Context(), dir, Config{Package: "./app", Output: "output", Name: "App", ID: "com.example.app", Version: "1.0"}, Target{"android", "arm64"}, io.Discard)
	if err == nil {
		t.Fatal("overwrote existing release")
	}
	b, _ := os.ReadFile(filepath.Join(dest, "owned"))
	if string(b) != "old" {
		t.Fatal("old artifact changed")
	}
}

func TestSDKManifestPreservesIdentityAndPermissions(t *testing.T) {
	source := []byte(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.app" android:versionName="1.0"><uses-permission android:name="android.permission.INTERNET"/><application android:label="A &amp; B"><activity android:name="org.golang.app.GoNativeActivity"/></application></manifest>`)
	result, err := manifestForSDK(source, true)
	if err != nil {
		t.Fatal(err)
	}
	d := xml.NewDecoder(bytes.NewReader(result))
	permission, icon, identity := false, false, false
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if start, ok := tok.(xml.StartElement); ok {
			for _, a := range start.Attr {
				if start.Name.Local == "manifest" && a.Name.Local == "package" && a.Value == "com.example.app" {
					identity = true
				}
				if start.Name.Local == "uses-permission" && a.Name.Space == androidSchema && a.Value == "android.permission.INTERNET" {
					permission = true
				}
				if start.Name.Local == "application" && a.Name.Space == androidSchema && a.Name.Local == "icon" && a.Value == "@mipmap/icon" {
					icon = true
				}
			}
		}
	}
	if !permission || !icon || !identity {
		t.Fatal(string(result))
	}
}
