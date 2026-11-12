package apps

import "testing"

func TestPlatformSupportDoesNotConfuseBuildAndNative(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range Platforms() {
		key := p.OS + "/" + p.Arch
		if seen[key] || !p.CoreBuild {
			t.Fatal("bad target", key)
		}
		seen[key] = true
		if p.OS != "darwin" && (p.NativeWindow == "unavailable" || p.WebView != "unavailable") {
			t.Fatal("software driver missing or unimplemented WebView advertised", p)
		}
	}
	if len(seen) != 6 {
		t.Fatal("missing platform targets")
	}
	if Platform("plan9", "amd64").CoreBuild || Platform("linux", "386").CoreBuild {
		t.Fatal("unsupported target advertised")
	}
}
