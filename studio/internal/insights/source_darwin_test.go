//go:build darwin

package insights

import "testing"

func TestDarwinSampler(t *testing.T) {
	s := newSource()
	defer s.Close()
	r := s.Read()
	if len(r.errors) > 0 {
		t.Fatal(r.errors)
	}
	if !r.processOK || !r.systemOK || r.rss == nil || *r.rss == 0 || r.total == nil || *r.total == 0 || r.used == nil || *r.used > *r.total {
		t.Fatalf("invalid native sample: %+v", r)
	}
}
