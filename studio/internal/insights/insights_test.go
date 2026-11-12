package insights

import (
	"image/png"
	"math"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIconPreview(t *testing.T) {
	h := IconHandler()
	for _, dark := range []string{"false", "true"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/studio/icon.png?name=activity&size=24&stroke=2&dark="+dark, nil))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		img, e := png.Decode(w.Body)
		if e != nil || img.Bounds().Dx() != 48 {
			t.Fatal(e)
		}
		visible := false
		for y := 0; y < 48; y++ {
			for x := 0; x < 48; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				visible = visible || a > 0
			}
		}
		if !visible {
			t.Fatal("empty preview")
		}
	}
	for _, q := range []string{"name=missing&size=24&stroke=2", "name=../activity&size=24&stroke=2", "name=activity&size=999&stroke=2", "name=activity&size=24&stroke=NaN"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/studio/icon.png?"+q, nil))
		if w.Code < 400 {
			t.Fatal(q)
		}
	}
}
func TestCPUDelta(t *testing.T) {
	a := raw{cpuSeconds: 10, ticks: [4]uint32{10, 10, 60, 20}, processOK: true, systemOK: true}
	b := raw{cpuSeconds: 13, ticks: [4]uint32{20, 30, 110, 40}, processOK: true, systemOK: true}
	p, s := cpuDelta(a, b, 2, true)
	if p == nil || s == nil || *p != 150 || *s != 50 {
		t.Fatal(p, s)
	}
	p, s = cpuDelta(a, b, 2, false)
	if p != nil || s != nil {
		t.Fatal("first sample must be unavailable")
	}
	p, s = cpuDelta(a, raw{}, 2, true)
	if p != nil || s != nil {
		t.Fatal("failure must not become zero")
	}
	a.ticks = [4]uint32{math.MaxUint32 - 1, 0, 0, 0}
	b.ticks = [4]uint32{2, 0, 4, 0}
	_, s = cpuDelta(a, b, 2, true)
	if s == nil || *s != 50 {
		t.Fatal("tick rollover", s)
	}
}

type fakeSource struct{}

func (fakeSource) Read() raw { return raw{} }
func (fakeSource) Close()    {}
func TestBoundedHistory(t *testing.T) {
	m := &Monitor{source: fakeSource{}}
	start := time.Unix(1000, 0)
	for i := 0; i < 130; i++ {
		m.take(start.Add(time.Duration(i) * 2 * time.Second))
	}
	snapshot := m.Snapshot()
	if len(snapshot.Samples) != 120 || snapshot.Samples[0].At != start.Add(20*time.Second).UnixMilli() {
		t.Fatal("history not bounded")
	}
	snapshot.Samples[0].Heap = 0
	if m.Snapshot().Samples[0].Heap == 0 {
		t.Fatal("snapshot aliases history")
	}
}
