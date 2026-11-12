// Package rope implements immutable UTF-8 documents using a persistent chunked
// treap. Versions share untouched branches and can be read concurrently.
package rope

import (
	"strings"
	"sync/atomic"
	"unicode/utf8"
)

const ChunkBytes = 2048

type Summary struct{ Bytes, Runes, UTF16, Newlines int }
type node struct {
	left, right *node
	chunk       string
	priority    uint64
	sum         Summary
	local       Summary
}
type Text struct{ root *node }

var sequence atomic.Uint64

func add(a, b Summary) Summary {
	return Summary{a.Bytes + b.Bytes, a.Runes + b.Runes, a.UTF16 + b.UTF16, a.Newlines + b.Newlines}
}
func count(s string) Summary {
	v := Summary{Bytes: len(s)}
	for _, r := range s {
		v.Runes++
		v.UTF16++
		if r > 0xffff {
			v.UTF16++
		}
		if r == '\n' {
			v.Newlines++
		}
	}
	return v
}
func summary(n *node) Summary {
	if n == nil {
		return Summary{}
	}
	return n.sum
}
func makeNode(l *node, s string, r *node, priority uint64) *node {
	n := &node{chunk: s, priority: priority, local: count(s)}
	return rebuild(n, l, r)
}
func rebuild(n, l, r *node) *node {
	copy := *n
	copy.left, copy.right = l, r
	copy.sum = add(add(summary(l), copy.local), summary(r))
	return &copy
}
func leaf(s string) *node {
	if s == "" {
		return nil
	}
	// SplitMix64 distributes monotonically assigned chunk IDs. No shared PRNG state.
	v := sequence.Add(0x9e3779b97f4a7c15)
	v = (v ^ (v >> 30)) * 0xbf58476d1ce4e5b9
	v = (v ^ (v >> 27)) * 0x94d049bb133111eb
	v ^= v >> 31
	return makeNode(nil, s, nil, v)
}
func join(a, b *node) *node {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.priority >= b.priority {
		return rebuild(a, a.left, join(a.right, b))
	}
	return rebuild(b, join(a, b.left), b.right)
}
func split(n *node, at int) (*node, *node) {
	if n == nil {
		return nil, nil
	}
	left := summary(n.left).Bytes
	if at < left {
		a, b := split(n.left, at)
		return a, rebuild(n, b, n.right)
	}
	if at > left+len(n.chunk) {
		a, b := split(n.right, at-left-len(n.chunk))
		return rebuild(n, n.left, a), b
	}
	at -= left
	a, b := n.left, n.right
	if at > 0 {
		a = makeNode(n.left, n.chunk[:at], nil, n.priority)
	}
	if at < len(n.chunk) {
		b = makeNode(nil, n.chunk[at:], n.right, n.priority)
	}
	return a, b
}
func New(s string) Text {
	s = strings.ToValidUTF8(s, "�")
	var root *node
	for len(s) > 0 {
		end := min(ChunkBytes, len(s))
		for end < len(s) && !utf8.RuneStart(s[end]) {
			end--
		}
		root = join(root, leaf(strings.Clone(s[:end])))
		s = s[end:]
	}
	return Text{root}
}
func (t Text) Summary() Summary { return summary(t.root) }
func (t Text) Len() int         { return summary(t.root).Bytes }
func (t Text) Same(u Text) bool { return t.root == u.root }
func (t Text) ByteAt(at int) byte {
	if at < 0 || at >= t.Len() {
		return 0
	}
	for n := t.root; n != nil; {
		left := summary(n.left).Bytes
		if at < left {
			n = n.left
		} else if at < left+len(n.chunk) {
			return n.chunk[at-left]
		} else {
			at -= left + len(n.chunk)
			n = n.right
		}
	}
	return 0
}
func (t Text) boundary(at int) int {
	at = max(0, min(at, t.Len()))
	for at > 0 && at < t.Len() && !utf8.RuneStart(t.ByteAt(at)) {
		at--
	}
	return at
}

// Chunks visits exact byte ranges without flattening the document. Returning false
// stops iteration. Slice/Chunks accept byte offsets; Replace rounds down to UTF-8.
func (t Text) Chunks(start, end int, visit func(string) bool) {
	start = max(0, min(start, t.Len()))
	end = max(start, min(end, t.Len()))
	var walk func(*node, int) bool
	walk = func(n *node, base int) bool {
		if n == nil || base >= end || base+n.sum.Bytes <= start {
			return true
		}
		pos := base + summary(n.left).Bytes
		if !walk(n.left, base) {
			return false
		}
		a, b := max(0, start-pos), min(len(n.chunk), end-pos)
		if a < b && !visit(n.chunk[a:b]) {
			return false
		}
		return walk(n.right, pos+len(n.chunk))
	}
	walk(t.root, 0)
}
func (t Text) Slice(start, end int) string {
	var b strings.Builder
	t.Chunks(start, end, func(s string) bool { b.WriteString(s); return true })
	return b.String()
}
func (t Text) String() string { return t.Slice(0, t.Len()) }
func (t Text) Replace(start, end int, s string) Text {
	start = t.boundary(start)
	end = max(start, t.boundary(end))
	s = strings.ToValidUTF8(s, "�")
	if end-start == len(s) && t.Slice(start, end) == s {
		return t
	}
	a, rest := split(t.root, start)
	_, b := split(rest, end-start)
	return Text{join(join(a, New(s).root), b)}
}
func (t Text) prefix(at int) Summary {
	at = t.boundary(at)
	var out Summary
	for n := t.root; n != nil; {
		left := summary(n.left)
		if at < left.Bytes {
			n = n.left
			continue
		}
		out = add(out, left)
		at -= left.Bytes
		if at <= len(n.chunk) {
			return add(out, count(n.chunk[:at]))
		}
		out = add(out, n.local)
		at -= len(n.chunk)
		n = n.right
	}
	return out
}
func (t Text) UTF16(at int) int      { return t.prefix(at).UTF16 }
func (t Text) LineOf(at int) int     { return t.prefix(at).Newlines }
func (t Text) RuneOffset(at int) int { return t.prefix(at).Runes }
func (t Text) unitOffset(units int, utf16 bool) int {
	units = max(0, units)
	pos := 0
	value := func(s Summary) int {
		if utf16 {
			return s.UTF16
		}
		return s.Runes
	}
	for n := t.root; n != nil; {
		left := summary(n.left)
		if units < value(left) {
			n = n.left
			continue
		}
		units -= value(left)
		pos += left.Bytes
		if units >= value(n.local) {
			units -= value(n.local)
			pos += len(n.chunk)
			n = n.right
			continue
		}
		for at, r := range n.chunk {
			size := 1
			if utf16 && r > 0xffff {
				size = 2
			}
			if units < size {
				return pos + at
			}
			units -= size
		}
		pos += len(n.chunk)
		n = n.right
	}
	return pos
}
func (t Text) FromUTF16(units int) int { return t.unitOffset(units, true) }
func (t Text) FromRune(runes int) int  { return t.unitOffset(runes, false) }
func (t Text) RuneAt(at int) (rune, int) {
	if at < 0 || at >= t.Len() {
		return 0, 0
	}
	return utf8.DecodeRuneInString(t.Slice(at, min(t.Len(), at+4)))
}
func (t Text) RuneBefore(at int) (rune, int) {
	if at <= 0 || at > t.Len() {
		return 0, 0
	}
	return utf8.DecodeLastRuneInString(t.Slice(max(0, at-4), at))
}
func (t Text) LineCount() int { return summary(t.root).Newlines + 1 }
func (t Text) LineStart(line int) int {
	if line <= 0 {
		return 0
	}
	if line >= t.LineCount() {
		return t.Len()
	}
	pos := 0
	for n := t.root; n != nil; {
		left := summary(n.left)
		if line <= left.Newlines {
			n = n.left
			continue
		}
		line -= left.Newlines
		pos += left.Bytes
		if line > n.local.Newlines {
			line -= n.local.Newlines
			pos += len(n.chunk)
			n = n.right
			continue
		}
		for i := 0; i < len(n.chunk); i++ {
			if n.chunk[i] == '\n' {
				line--
				if line == 0 {
					return pos + i + 1
				}
			}
		}
		pos += len(n.chunk)
		n = n.right
	}
	return pos
}
func (t Text) LineEnd(line int) int {
	if line < 0 {
		return 0
	}
	if line >= t.LineCount()-1 {
		return t.Len()
	}
	return t.LineStart(line+1) - 1
}
func (t Text) Line(line int) string { return t.Slice(t.LineStart(line), t.LineEnd(line)) }
func (t Text) EachLine(from, to int, visit func(line, start int, s string) bool) {
	for i := max(0, from); i < min(to, t.LineCount()); i++ {
		if !visit(i, t.LineStart(i), t.Line(i)) {
			return
		}
	}
}
func (t Text) EachLineBack(from, to int, visit func(line, start int, s string) bool) {
	for i := min(from, t.LineCount()-1); i >= max(0, to); i-- {
		if !visit(i, t.LineStart(i), t.Line(i)) {
			return
		}
	}
}
