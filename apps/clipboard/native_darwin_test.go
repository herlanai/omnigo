//go:build darwin && (amd64 || arm64)

package clipboard

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
)

func TestPrivatePasteboardRoundTrip(t *testing.T) {
	if os.Getenv("OMNIGO_NATIVE_TEST") != "1" {
		t.Skip("explicit native apps test")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	c, err := Open(Options{Name: fmt.Sprintf("org.omnigo.tests.%d", os.Getpid())})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	defer c.Clear(context.Background())
	d, err := New(Item{{Format: Text, Bytes: []byte("中文 😀")}, {Format: HTML, Bytes: []byte("<b>中文</b>")}}, Item{{Format: Text, Bytes: []byte("second")}})
	if err != nil {
		t.Fatal(err)
	}
	released := 0
	if err = c.Write(context.Background(), d, func() { released++ }); err != nil {
		t.Fatal(err)
	}
	if released != 1 {
		t.Fatal("release")
	}
	got, err := c.Read(context.Background(), Text, HTML)
	if err != nil || got.Len() != 2 {
		t.Fatal(got.Len(), err)
	}
	for i := 0; i < d.Len(); i++ {
		for _, f := range d.Formats(i) {
			want, _ := d.Read(context.Background(), i, f)
			b, err := got.Read(context.Background(), i, f)
			if err != nil || string(want) != string(b) {
				t.Fatal(i, f, string(b), err)
			}
		}
	}
}
