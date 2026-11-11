package omniui

import (
	"fmt"
	"math"
	"reflect"
	"testing"
)

func node(key, text string, children ...*Node) *Node {
	n, err := NewNode(key, "omniui.text", Props{Text: text}, children...)
	if err != nil {
		panic(err)
	}
	return n
}

// The mutable model is an independent renderer oracle for the patch protocol.
type model struct {
	Key, Component string
	Props          Props
	Children       []*model
}

func modelOf(n *Node) *model {
	if n == nil {
		return nil
	}
	m := &model{Key: n.Key(), Component: n.Component(), Props: n.Properties()}
	for _, c := range n.Children() {
		m.Children = append(m.Children, modelOf(c))
	}
	return m
}
func apply(root **model, p Patch) {
	location := root
	for _, key := range p.Path {
		parent := *location
		index := -1
		for i, c := range parent.Children {
			if c != nil && c.Key == key {
				index = i
				break
			}
		}
		if index == -1 {
			parent.Children = append(parent.Children, &model{Key: key})
			index = len(parent.Children) - 1
		}
		location = &parent.Children[index]
	}
	switch p.Operation {
	case Replace:
		*location = modelOf(p.Node)
	case Remove:
		*location = nil
	case Update:
		(*location).Props = p.Node.Properties()
	case Order:
		m := *location
		lookup := map[string]*model{}
		for _, c := range m.Children {
			if c != nil {
				lookup[c.Key] = c
			}
		}
		m.Children = nil
		for _, key := range p.Keys {
			c := lookup[key]
			if c == nil {
				panic("order references absent child")
			}
			m.Children = append(m.Children, c)
		}
	}
}

func TestDiffProtocol(t *testing.T) {
	stable := node("stable", "keep")
	a := node("root", "", node("a", "old"), node("b", "delete"), stable)
	b := node("root", "new root", stable, node("a", "new"), node("c", "insert"))
	actual := modelOf(a)
	for _, p := range Diff(a, b) {
		apply(&actual, p)
	}
	if !reflect.DeepEqual(actual, modelOf(b)) {
		t.Fatalf("patches failed: %#v", Diff(a, b))
	}
	if got := Diff(b, b); len(got) != 0 {
		t.Fatal(got)
	}
	if allocs := testing.AllocsPerRun(100, func() { Diff(b, b) }); allocs != 0 {
		t.Fatalf("unchanged tree allocates %v", allocs)
	}
}

func TestTreeValidationAndOwnership(t *testing.T) {
	children := []*Node{node("a", "one")}
	root, err := NewNode("root", "omniui.column", Props{}, children...)
	if err != nil {
		t.Fatal(err)
	}
	children[0] = node("b", "two")
	copy := root.Children()
	copy[0] = children[0]
	if root.Children()[0].Key() != "a" {
		t.Fatal("external mutation changed tree")
	}
	if _, err := NewNode("root", "omniui.column", Props{}, children[0], children[0]); err == nil {
		t.Fatal("duplicate accepted")
	}
	if _, err := NewNode("root", "omniui.column", Props{Style: Style{Width: float32(math.NaN())}}); err == nil {
		t.Fatal("NaN accepted")
	}
}

func FuzzDiff(f *testing.F) {
	f.Add([]byte{4, 2, 0, 9})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 128 {
			return
		}
		var old, new []*Node
		for i := 0; i < 16; i++ {
			old = append(old, node(fmt.Sprint(i), "old"))
		}
		seen := map[byte]bool{}
		for _, v := range data {
			v %= 32
			if !seen[v] {
				new = append(new, node(fmt.Sprint(v), "new"))
				seen[v] = true
			}
		}
		a, b := node("root", "", old...), node("root", "", new...)
		actual := modelOf(a)
		for _, p := range Diff(a, b) {
			apply(&actual, p)
		}
		if !reflect.DeepEqual(actual, modelOf(b)) {
			t.Fatal("patch round-trip differs")
		}
	})
}

func TestVirtualRange(t *testing.T) {
	maxCount := int(^uint(0) >> 1)
	if a, b, _ := VisibleRange(maxCount, 1, 1e300, 1, 0); a != maxCount || b != maxCount {
		t.Fatal("integer overflow", a, b)
	}
	a, b, spacer := VisibleRange(1_000_000, 20, 1000, 400, 2)
	if a != 48 || b != 72 || spacer != 960 {
		t.Fatal(a, b, spacer)
	}
	a, b, _ = VisibleRange(10, 20, 1e300, 400, 2)
	if a != 8 || b != 10 {
		t.Fatal(a, b)
	}
	if a, b, _ := VisibleRange(10, 20, math.NaN(), 400, 2); a != 0 || b != 0 {
		t.Fatal("invalid geometry accepted")
	}
}

func BenchmarkDiffUnchanged(b *testing.B) {
	root := node("root", "", node("label", "Hello"))
	b.ReportAllocs()
	for b.Loop() {
		Diff(root, root)
	}
}

func BenchmarkDiffOneOf10000(b *testing.B) {
	children := make([]*Node, 10000)
	for i := range children {
		children[i] = node(fmt.Sprint(i), "value")
	}
	before := node("root", "", children...)
	children[5000] = node("5000", "changed")
	after := node("root", "", children...)
	b.ReportAllocs()
	for b.Loop() {
		Diff(before, after)
	}
}

func BenchmarkVirtualMillionRows(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		VisibleRange(1_000_000, 24, 12000, 800, 4)
	}
}
