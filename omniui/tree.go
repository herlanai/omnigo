package omniui

import (
	"errors"
	"fmt"
	"math"
)

// Style uses logical pixels. Zero dimensions allow the renderer to size content.
type Style struct {
	Width, Height, Gap, Padding float32
	Foreground, Background      uint32
	Disabled                    bool
}

type Props struct {
	Text   string
	Action string
	Style  Style
}

// Node is immutable. A stable key identifies a sibling across updates. Reuse
// unchanged subtrees to let Diff skip them without traversal or allocation.
type Node struct {
	key, component string
	props          Props
	children       []*Node
}

func NewNode(key, component string, props Props, children ...*Node) (*Node, error) {
	if key == "" || !identifier.MatchString(component) {
		return nil, errors.New("omniui: key and namespaced component required")
	}
	for _, v := range []float32{props.Style.Width, props.Style.Height, props.Style.Gap, props.Style.Padding} {
		if v < 0 || math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil, errors.New("omniui: dimensions must be finite and nonnegative")
		}
	}
	seen := make(map[string]bool, len(children))
	for _, child := range children {
		if child == nil {
			return nil, errors.New("omniui: nil child")
		}
		if seen[child.key] {
			return nil, fmt.Errorf("omniui: duplicate sibling key %q", child.key)
		}
		seen[child.key] = true
	}
	return &Node{key: key, component: component, props: props, children: append([]*Node(nil), children...)}, nil
}

func (n *Node) Key() string       { return n.key }
func (n *Node) Component() string { return n.component }
func (n *Node) Properties() Props { return n.props }
func (n *Node) Children() []*Node { return append([]*Node(nil), n.children...) }

func (n *Node) WithProps(props Props) (*Node, error) {
	if n.props == props {
		return n, nil
	}
	return NewNode(n.key, n.component, props, n.children...)
}

func (n *Node) WithChildren(children ...*Node) (*Node, error) {
	return NewNode(n.key, n.component, n.props, children...)
}

type Operation string

const (
	Replace Operation = "replace"
	Remove  Operation = "remove"
	Update  Operation = "update"
	Order   Operation = "order"
)

// Patch paths are sibling-key paths from the root (the root path is empty).
// Replace inserts or replaces at Path; Remove deletes; Update changes props;
// Order sets the complete child order after child insertions/removals.
type Patch struct {
	Operation Operation
	Path      []string
	Node      *Node
	Keys      []string
}

func Diff(before, after *Node) []Patch {
	var patches []Patch
	var walk func(*Node, *Node, []string)
	emit := func(op Operation, path []string, node *Node, keys []string) {
		patches = append(patches, Patch{Operation: op, Path: append([]string(nil), path...), Node: node, Keys: keys})
	}
	walk = func(a, b *Node, path []string) {
		if a == b {
			return
		}
		if b == nil {
			emit(Remove, path, nil, nil)
			return
		}
		if a == nil || a.key != b.key || a.component != b.component {
			emit(Replace, path, b, nil)
			return
		}
		if a.props != b.props {
			emit(Update, path, b, nil)
		}
		// The common case needs no lookup map: unchanged order and stable keys.
		sameOrder := len(a.children) == len(b.children)
		if sameOrder {
			for i, c := range a.children {
				if c.key != b.children[i].key {
					sameOrder = false
					break
				}
			}
		}
		if sameOrder {
			for i, c := range b.children {
				if a.children[i] != c {
					walk(a.children[i], c, append(path, c.key))
				}
			}
			return
		}
		old := make(map[string]*Node, len(a.children))
		for _, c := range a.children {
			old[c.key] = c
		}
		keys := make([]string, len(b.children))
		for i, c := range b.children {
			keys[i] = c.key
			walk(old[c.key], c, append(path, c.key))
			delete(old, c.key)
		}
		for _, c := range a.children {
			if old[c.key] != nil {
				emit(Remove, append(path, c.key), nil, nil)
			}
		}
		emit(Order, path, nil, keys)
	}
	walk(before, after, nil)
	return patches
}
