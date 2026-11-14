package mobilebuild

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const androidSchema = "http://schemas.android.com/apk/res/android"

func androidTools() (string, string, error) {
	sdk := os.Getenv("ANDROID_HOME")
	if sdk == "" {
		return "", "", errors.New("mobilebuild: ANDROID_HOME is required (platforms;android-35 and build-tools;35.0.0)")
	}
	tools := filepath.Join(sdk, "build-tools", "35.0.0")
	jar := filepath.Join(sdk, "platforms", "android-35", "android.jar")
	for _, p := range []string{filepath.Join(tools, "aapt2"), filepath.Join(tools, "zipalign"), filepath.Join(tools, "lib", "apksigner.jar"), jar} {
		if _, err := os.Stat(p); err != nil {
			return "", "", fmt.Errorf("mobilebuild: missing Android SDK tool %s: %w", p, err)
		}
	}
	if runtime.GOOS == "windows" {
		return "", "", errors.New("mobilebuild: Android packaging currently requires a macOS or Linux build host")
	}
	return tools, jar, nil
}

func manifestForSDK(source []byte, icon bool) ([]byte, error) {
	d := xml.NewDecoder(bytes.NewReader(source))
	var b bytes.Buffer
	e := xml.NewEncoder(&b)
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if start, ok := token.(xml.StartElement); ok {
			attrs := make([]xml.Attr, 0, len(start.Attr)+1)
			for _, a := range start.Attr {
				if a.Name.Space != "xmlns" && a.Name.Local != "xmlns" {
					attrs = append(attrs, a)
				}
			}
			if start.Name.Local == "application" && icon {
				attrs = append(attrs, xml.Attr{Name: xml.Name{Space: androidSchema, Local: "icon"}, Value: "@mipmap/icon"})
			}
			start.Attr = attrs
			token = start
		}
		if err = e.EncodeToken(token); err != nil {
			return nil, err
		}
	}
	if err := e.Flush(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func developmentKey(ctx context.Context, dir string, out io.Writer) (string, error) {
	private := filepath.Join(dir, ".omni")
	if err := os.MkdirAll(private, 0700); err != nil {
		return "", err
	}
	key := filepath.Join(private, "android-debug.p12")
	if info, err := os.Lstat(key); err == nil {
		if !info.Mode().IsRegular() {
			return "", errors.New("mobilebuild: debug key must be regular")
		}
		return key, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	lock := filepath.Join(private, "key.lock")
	if err := os.Mkdir(lock, 0700); err != nil {
		return "", errors.New("mobilebuild: debug key initialization already in progress")
	}
	defer os.Remove(lock)
	temp, err := os.MkdirTemp(private, "key-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)
	staged := filepath.Join(temp, "debug.p12")
	if err = command(ctx, dir, os.Environ(), out, "keytool", "-genkeypair", "-keystore", staged, "-storetype", "PKCS12", "-storepass", "android", "-keypass", "android", "-alias", "androiddebugkey", "-dname", "CN=Android Debug,O=Android,C=US", "-keyalg", "RSA", "-validity", "10000"); err != nil {
		return "", err
	}
	if err = os.Chmod(staged, 0600); err != nil {
		return "", err
	}
	if _, err = os.Lstat(key); !os.IsNotExist(err) {
		return "", errors.New("mobilebuild: key appeared during initialization")
	}
	if err = os.Rename(staged, key); err != nil {
		return "", err
	}
	return key, nil
}

// finalizeAndroid replaces gomobile's legacy SDK declaration/resources using
// the official SDK, then aligns and signs with a persistent project debug key.
func finalizeAndroid(ctx context.Context, dir, apk string, c Config, manifest []byte, out io.Writer) error {
	tools, jar, err := androidTools()
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp(filepath.Dir(apk), "android-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	run := func(args ...string) error { return command(ctx, dir, os.Environ(), out, args...) }
	icon, err := os.ReadFile(filepath.Join(dir, c.Package, "assets", "icon.png"))
	hasIcon := err == nil
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	args := []string{filepath.Join(tools, "aapt2"), "link", "-I", jar, "--manifest", filepath.Join(work, "AndroidManifest.xml"), "--min-sdk-version", "23", "--target-sdk-version", "35", "-o", filepath.Join(work, "resources.apk")}
	if hasIcon {
		res := filepath.Join(work, "res", "mipmap-xxxhdpi")
		if err = os.MkdirAll(res, 0755); err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(res, "icon.png"), icon, 0644); err != nil {
			return err
		}
		compiled := filepath.Join(work, "compiled.zip")
		if err = run(filepath.Join(tools, "aapt2"), "compile", "--dir", filepath.Join(work, "res"), "-o", compiled); err != nil {
			return err
		}
		args = append(args, compiled)
	}
	updated, err := manifestForSDK(manifest, hasIcon)
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(work, "AndroidManifest.xml"), updated, 0644); err != nil {
		return err
	}
	if err = run(args...); err != nil {
		return err
	}
	unsigned := filepath.Join(work, "unsigned.apk")
	if err = mergeAPK(apk, filepath.Join(work, "resources.apk"), unsigned); err != nil {
		return err
	}
	aligned := filepath.Join(work, "aligned.apk")
	if err = run(filepath.Join(tools, "zipalign"), "-P", "16", "-f", "4", unsigned, aligned); err != nil {
		return err
	}
	key, err := developmentKey(ctx, dir, out)
	if err != nil {
		return err
	}
	signer := filepath.Join(tools, "lib", "apksigner.jar")
	if err = run("java", "-jar", signer, "sign", "--ks", key, "--ks-pass", "pass:android", "--key-pass", "pass:android", "--ks-key-alias", "androiddebugkey", "--out", apk, aligned); err != nil {
		return err
	}
	if err = run("java", "-jar", signer, "verify", "--verbose", apk); err != nil {
		return err
	}
	return run(filepath.Join(tools, "zipalign"), "-c", "-P", "16", "4", apk)
}
func mergeAPK(native, resources, output string) (result error) {
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, f.Close()) }()
	w := zip.NewWriter(f)
	defer func() { result = errors.Join(result, w.Close()) }()
	seen := map[string]bool{}
	for i, path := range []string{native, resources} {
		r, err := zip.OpenReader(path)
		if err != nil {
			return err
		}
		err = func() error {
			defer r.Close()
			for _, entry := range r.File {
				name := entry.Name
				if i == 0 && (strings.HasPrefix(name, "META-INF/") || name == "AndroidManifest.xml" || name == "resources.arsc" || strings.HasPrefix(name, "res/")) {
					continue
				}
				if seen[name] {
					return fmt.Errorf("mobilebuild: duplicate APK entry %s", name)
				}
				seen[name] = true
				// Copy preserves the method; zipalign aligns stored native libraries later.
				if err := w.Copy(entry); err != nil {
					return err
				}
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}
	return nil
}

// CheckAndroidTools checks paths only; builds and device tests remain separate.
func CheckAndroidTools() error {
	if _, _, err := androidTools(); err != nil {
		return err
	}
	for _, tool := range []string{"gomobile", "java", "keytool"} {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("mobilebuild: missing %s: %w", tool, err)
		}
	}
	return nil
}
