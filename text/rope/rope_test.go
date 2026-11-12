package rope

import (
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

func checkTree(t *testing.T, n *node) Summary {
	t.Helper()
	if n == nil {
		return Summary{}
	}
	if len(n.chunk) == 0 || len(n.chunk) > ChunkBytes || !utf8.ValidString(n.chunk) {
		t.Fatal("invalid chunk")
	}
	for _, child := range []*node{n.left, n.right} {
		if child != nil && child.priority > n.priority {
			t.Fatal("heap invariant")
		}
	}
	want := add(add(checkTree(t, n.left), count(n.chunk)), checkTree(t, n.right))
	if n.sum != want {
		t.Fatal("summary mismatch", n.sum, want)
	}
	return want
}
func TestPersistentEditsAgainstString(t *testing.T) {
	rng := rand.New(rand.NewPCG(123, 456))
	want := strings.Repeat("hello 中文 😀\n", 600)
	doc := New(want)
	history := []Text{doc}
	expected := []string{want}
	inserts := []string{"", "Z", "界", "😀\n", "\r\n", "é"}
	for step := 0; step < 1200; step++ {
		r := []rune(want)
		a := rng.IntN(len(r) + 1)
		b := min(len(r), a+rng.IntN(16))
		insert := inserts[rng.IntN(len(inserts))]
		doc = doc.Replace(len(string(r[:a])), len(string(r[:b])), insert)
		want = string(r[:a]) + insert + string(r[b:])
		if doc.String() != want {
			t.Fatalf("edit %d", step)
		}
		if step%31 == 0 {
			checkTree(t, doc.root)
			history = append(history, doc)
			expected = append(expected, want)
		}
	}
	var wg sync.WaitGroup
	for i, version := range history {
		wg.Add(1)
		go func(i int, v Text) {
			defer wg.Done()
			if v.String() != expected[i] {
				t.Error("snapshot changed")
			}
		}(i, version)
	}
	wg.Wait()
}
func TestCoordinatesChunksAndLines(t *testing.T) {
	s := strings.Repeat("a", ChunkBytes-1) + "😀中\r\nsecond\n"
	doc := New(s)
	runes, units := 0, 0
	for at, r := range s {
		if doc.FromRune(runes) != at || doc.UTF16(at) != units || doc.FromUTF16(units) != at || doc.RuneOffset(at) != runes {
			t.Fatal("coordinate", at)
		}
		got, n := doc.RuneAt(at)
		if got != r || n != utf8.RuneLen(r) {
			t.Fatal("RuneAt")
		}
		before, size := doc.RuneBefore(at + n)
		if before != r || size != n {
			t.Fatal("RuneBefore")
		}
		if r > 0xffff {
			if doc.FromUTF16(units+1) != at {
				t.Fatal("surrogate midpoint")
			}
			units++
		}
		units++
		runes++
	}
	if doc.Summary().UTF16 != len(utf16.Encode([]rune(s))) || doc.FromUTF16(99999) != len(s) {
		t.Fatal("end")
	}
	lines := strings.Split(s, "\n")
	start := 0
	doc.EachLine(0, doc.LineCount(), func(i, at int, line string) bool {
		if at != start || line != lines[i] || doc.LineOf(at) != i {
			t.Fatal("line", i)
		}
		start += len(line) + 1
		return true
	})
	last := len(lines) - 1
	doc.EachLineBack(999, 0, func(i, at int, line string) bool {
		if i != last || line != lines[i] {
			t.Fatal("reverse")
		}
		last--
		return true
	})
	calls := 0
	doc.Chunks(0, doc.Len(), func(s string) bool { calls++; return false })
	if calls != 1 {
		t.Fatal("chunk cancellation")
	}
	if !doc.Replace(0, 1, "a").Same(doc) {
		t.Fatal("no-op version")
	}
	if New("😀!").Replace(1, 3, "x").String() != "x😀!" {
		t.Fatal("byte boundary rounding")
	}
}
func FuzzReplace(f *testing.F) {
	f.Add("a😀\nb", 2, 6, "中")
	f.Fuzz(func(t *testing.T, s string, a, b int, insert string) {
		if len(s) > 32000 || len(insert) > 4096 {
			return
		}
		doc := New(s)
		a = doc.boundary(a)
		b = max(a, doc.boundary(b))
		want := doc.Slice(0, a) + strings.ToValidUTF8(insert, "�") + doc.Slice(b, doc.Len())
		got := doc.Replace(a, b, insert)
		if got.String() != want {
			t.Fatal("replace")
		}
		checkTree(t, got.root)
	})
}
func BenchmarkPersistentEdit(b *testing.B) {
	source := New(strings.Repeat("line 😀\n", 100000))
	at := source.FromRune(200000)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = source.Replace(at, at, "x")
	}
}
