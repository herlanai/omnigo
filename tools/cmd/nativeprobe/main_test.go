package main

import (
	"strings"
	"testing"
)

func TestRequireNativeEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, log string
		valid     bool
	}{
		{"pass", `{"Action":"pass","Test":"Native"}`, true},
		{"missing", "", false},
		{"skip", `{"Action":"skip","Test":"Native"}`, false},
		{"run", `{"Action":"run","Test":"Native"}`, false},
		{"fail", `{"Action":"fail","Test":"Native"}`, false},
		{"package failure", "{\"Action\":\"pass\",\"Test\":\"Native\"}\n{\"Action\":\"fail\",\"Package\":\"host\"}", false},
		{"skip after pass", "{\"Action\":\"pass\",\"Test\":\"Native\"}\n{\"Action\":\"skip\",\"Test\":\"Native\"}", false},
		{"malformed", "not JSON", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := verify(strings.NewReader(tc.log), []string{"Native"})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, err=%v", tc.valid, err)
			}
		})
	}
}
