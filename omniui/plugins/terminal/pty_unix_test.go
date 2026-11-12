//go:build darwin || linux

package terminal

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestRealPTYAndExit(t *testing.T) {
	p, e := Start(context.Background(), Options{Command: "/bin/sh", Args: []string{"-c", "test -t 0 && test -t 1 && printf 'PTY_READY'"}})
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	b, e := io.ReadAll(p)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(b), "PTY_READY") {
		t.Fatalf("not a terminal: %q", b)
	}
	if e = p.Wait(); e != nil {
		t.Fatal(e)
	}
}
func TestCancelResizeAndCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p, e := Start(ctx, Options{Command: "/bin/sh", Args: []string{"-c", "read value"}})
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if e = p.Resize(120, 40); e != nil {
		t.Fatal(e)
	}
	if e = p.Resize(-1, 0); e == nil {
		t.Fatal("invalid size")
	}
	cancel()
	if e = p.Wait(); e == nil {
		t.Fatal("canceled process exited successfully")
	}
}

func TestPTYOutputFeedsNativeScreen(t *testing.T) {
	p, e := Start(context.Background(), Options{Command: "/bin/sh", Args: []string{"-c", "printf '\033[32mREADY\033[0m'"}})
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	output, e := io.ReadAll(p)
	if e != nil {
		t.Fatal(e)
	}
	if e = p.Wait(); e != nil {
		t.Fatal(e)
	}
	screen, _ := NewScreen(20, 4)
	if e = screen.Feed(output); e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(screen.Text(), "READY") || screen.Cells()[0].Foreground != indexedColor(2) {
		t.Fatal("PTY VT output not interpreted")
	}
}
