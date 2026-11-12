package catalog

import "testing"

func BenchmarkEntries(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = Entries()
	}
}
func BenchmarkHasDemo(b *testing.B) {
	_ = HasDemo("button")
	b.ReportAllocs()
	for b.Loop() {
		_ = HasDemo("button")
	}
}
func TestEntriesIsolation(t *testing.T) {
	a, b := Entries(), Entries()
	a[0].Title = "changed"
	if a[0].Implementation != nil {
		*a[0].Implementation = "changed"
	}
	if b[0].Title == "changed" || (b[0].Implementation != nil && *b[0].Implementation == "changed") {
		t.Fatal("shared snapshot")
	}
	for _, e := range b {
		if e.Demo != "" && !HasDemo(e.Demo) {
			t.Fatal(e.Demo)
		}
	}
	if HasDemo("") || HasDemo("nonexistent") {
		t.Fatal("invalid demo")
	}
}
