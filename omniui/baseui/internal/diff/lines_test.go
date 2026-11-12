package diff

import (
	"strings"
	"testing"
)

func reconstruct(t *testing.T, a, b string) {
	t.Helper()
	r := Compare(a, b)
	at := 0
	var out strings.Builder
	for _, h := range r.Hunks {
		if h.OldStart < at || h.OldEnd < h.OldStart || h.OldEnd > len(r.OldLines) || h.NewStart < 0 || h.NewEnd < h.NewStart || h.NewEnd > len(r.NewLines) {
			t.Fatal("invalid hunk", h)
		}
		out.WriteString(strings.Join(r.OldLines[at:h.OldStart], ""))
		out.WriteString(strings.Join(r.NewLines[h.NewStart:h.NewEnd], ""))
		at = h.OldEnd
	}
	out.WriteString(strings.Join(r.OldLines[at:], ""))
	if out.String() != b {
		t.Fatalf("reconstruction mismatch: %q != %q", out.String(), b)
	}
	oldIndex, newIndex := 0, 0
	for _, row := range r.Rows {
		if row.Old >= 0 {
			if row.Old != oldIndex {
				t.Fatal("old row skipped")
			}
			oldIndex++
		}
		if row.New >= 0 {
			if row.New != newIndex {
				t.Fatal("new row skipped")
			}
			newIndex++
		}
	}
	if oldIndex != len(r.OldLines) || newIndex != len(r.NewLines) {
		t.Fatal("incomplete rows")
	}
}
func TestExactReconstructionAndCoarseBudget(t *testing.T) {
	for _, p := range [][2]string{{"", ""}, {"", "x\n"}, {"a\r\nb", "a\r\nc\n"}, {"甲\n乙\n", "甲\n丙\n"}, {"a\nb\na\n", "b\na\nb\n"}} {
		reconstruct(t, p[0], p[1])
	}
	a, b := strings.Repeat("old\n", 1100), strings.Repeat("new\n", 1100)
	reconstruct(t, a, b)
	if !Compare(a, b).Coarse {
		t.Fatal("missing coarse marker")
	}
	if Compare("", b).Coarse || Compare(a, a).Coarse {
		t.Fatal("exact comparison marked coarse")
	}
}
func FuzzReconstruction(f *testing.F) {
	f.Add("a\r\nb\n", "a\nb\n")
	f.Add("", "中文")
	f.Fuzz(func(t *testing.T, a, b string) {
		if len(a)+len(b) > 32768 {
			t.Skip()
		}
		reconstruct(t, a, b)
	})
}
