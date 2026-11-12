package automation

import (
	"context"
	"encoding/json"
	"net"
	"github.com/herlanai/omnigo/runtime/dispatch"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalProtocolUsesUIDispatch(t *testing.T) {
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	q := dispatch.New(4)
	defer q.Close()
	s, e := Listen(context.Background(), filepath.Join(dir, "ui.sock"), q, func(r Request) (any, error) { return r.Method, nil })
	if e != nil {
		t.Fatal(e)
	}
	defer func() { s.Close(); s.Wait() }()
	c, e := net.Dial("unix", filepath.Join(dir, "ui.sock"))
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	if _, e = c.Write([]byte("{\"id\":1,\"method\":\"snapshot\"}\n")); e != nil {
		t.Fatal(e)
	}
	<-q.Wake()
	if n := q.Drain(1); n != 1 {
		t.Fatal(n)
	}
	var r response
	if e = json.NewDecoder(c).Decode(&r); e != nil || r.Result != "snapshot" {
		t.Fatal(r, e)
	}
}
