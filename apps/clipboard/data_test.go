package clipboard

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSnapshotsProvidersAndOwnership(t *testing.T) {
	bytes := []byte("original")
	var calls atomic.Int32
	d, err := New(Item{{Format: Text, Bytes: bytes}, {Format: HTML, Provide: func() ([]byte, error) { calls.Add(1); return []byte("<b>hi</b>"), nil }}})
	if err != nil {
		t.Fatal(err)
	}
	bytes[0] = 'X'
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, err := d.Read(context.Background(), 0, HTML)
			if err != nil || string(b) != "<b>hi</b>" {
				t.Error(err)
			}
			b[0] = 'X'
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal(calls.Load())
	}
	b, _ := d.Read(context.Background(), 0, Text)
	if string(b) != "original" {
		t.Fatal("retained input bytes")
	}
	snapshot := d.Snapshot()
	snapshot.Read(context.Background(), 0, HTML)
	if calls.Load() != 2 {
		t.Fatal("snapshot provider cache reused")
	}
	if _, err = d.Read(context.Background(), 0, PNG); !errors.Is(err, ErrFormat) {
		t.Fatal(err)
	}
}

type memoryAdapter struct {
	data           []map[string][]byte
	writes, closes int
	err            error
}

func (m *memoryAdapter) read([]string) ([]map[string][]byte, error) { return m.data, m.err }
func (m *memoryAdapter) write(data []map[string][]byte) error {
	if m.err != nil {
		return m.err
	}
	m.data = data
	m.writes++
	return nil
}
func (m *memoryAdapter) close() { m.closes++ }
func TestFailuresCancellationAndRelease(t *testing.T) {
	b := &memoryAdapter{}
	c := &Clipboard{backend: b}
	released := 0
	d, _ := New(Item{{Format: Text, Provide: func() ([]byte, error) { panic("bad provider") }}})
	if c.Write(context.Background(), d, func() { released++ }) == nil || released != 0 || b.writes != 0 {
		t.Fatal("failed provider changed ownership")
	}
	d, _ = New(Item{{Format: Text, Bytes: []byte("x")}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(c.Write(ctx, d, nil), context.Canceled) || b.writes != 0 {
		t.Fatal("cancelled write executed")
	}
	if err := c.Write(context.Background(), d, func() { released++ }); err != nil {
		t.Fatal(err)
	}
	if released != 1 || b.writes != 1 {
		t.Fatal("release contract")
	}
	c.Close()
	c.Close()
	if b.closes != 1 || !errors.Is(c.Write(context.Background(), d, nil), ErrClosed) {
		t.Fatal("closed backend reused")
	}
}
func TestNativeEncodings(t *testing.T) {
	fragment := []byte("<b>中文 😀</b>")
	encoded := htmlClipboard(fragment)
	got, err := htmlFragment(encoded)
	if err != nil || !bytes.Equal(got, fragment) {
		t.Fatal(string(got), err)
	}
	if _, err = htmlFragment([]byte("StartFragment:100\nEndFragment:1\n")); err == nil {
		t.Fatal("invalid offsets")
	}
	var out bytes.Buffer
	if err = png.Encode(&out, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	data := out.Bytes()
	got, err = pngBytes(append(append([]byte{}, data...), 0, 0))
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("PNG allocation padding", err)
	}
}
