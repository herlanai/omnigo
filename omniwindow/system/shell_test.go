package system

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestOpenURLRejectsExecutableSchemes(t *testing.T) {
	for _, s := range []string{"file:///tmp/app", "javascript:alert(1)", "https:\n//example.com", "https:", ""} {
		if OpenURL(context.Background(), s) == nil {
			t.Fatalf("accepted %q", s)
		}
	}
}
func TestLinuxOpenerArgumentsAndFallback(t *testing.T) {
	input := "https://example.com/?a=1&b=$(touch%20bad)"
	find := func(name string) (string, error) {
		if name == "gio" {
			return "/usr/bin/gio", nil
		}
		return "", errors.New("missing")
	}
	name, args, err := opener("linux", input, find)
	if err != nil || name != "/usr/bin/gio" || !reflect.DeepEqual(args, []string{"open", input}) {
		t.Fatalf("%s %v %v", name, args, err)
	}
	if _, _, err := opener("linux", input, func(string) (string, error) { return "", errors.New("missing") }); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
