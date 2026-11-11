package omniui

import (
	"reflect"
	"testing"
)

func TestRegistryDependencyPolicy(t *testing.T) {
	var registry Registry
	a := Extension{Descriptor: Descriptor{ID: "omniui.storage", Version: "1", Kind: Service, Permissions: []string{"files.read"}}}
	b := Extension{Descriptor: Descriptor{ID: "omniui.editor", Version: "1", Kind: Control, Requires: []string{"omniui.storage"}}}
	for _, e := range []Extension{a, b} {
		if err := registry.Register(e); err != nil {
			t.Fatal(err)
		}
	}
	b.Descriptor.Requires[0] = "mutated"
	if _, err := registry.Resolve([]string{"omniui.editor"}, nil); err == nil {
		t.Fatal("missing permission accepted")
	}
	plan, err := registry.Resolve([]string{"omniui.editor"}, []string{"files.read"})
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{plan[0].Descriptor.ID, plan[1].Descriptor.ID}; !reflect.DeepEqual(got, []string{"omniui.storage", "omniui.editor"}) {
		t.Fatal(got)
	}
	plan[0].Descriptor.Permissions[0] = "mutated"
	if _, err := registry.Resolve([]string{"omniui.editor"}, []string{"files.read"}); err != nil {
		t.Fatal("snapshot mutated registry:", err)
	}
	if err := registry.Register(a); err == nil {
		t.Fatal("duplicate accepted")
	}
}

func TestRegistryCycleAndMissing(t *testing.T) {
	var registry Registry
	for _, d := range []Descriptor{
		{ID: "test.a", Version: "1", Kind: Service, Requires: []string{"test.b"}},
		{ID: "test.b", Version: "1", Kind: Service, Requires: []string{"test.a"}},
	} {
		if err := registry.Register(Extension{Descriptor: d}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := registry.Resolve([]string{"test.a"}, nil); err == nil {
		t.Fatal("cycle accepted")
	}
	if _, err := registry.Resolve([]string{"test.missing"}, nil); err == nil {
		t.Fatal("missing accepted")
	}
}
