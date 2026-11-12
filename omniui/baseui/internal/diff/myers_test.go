package diff

import (
	"fmt"
	"strings"
	"testing"
)

func TestLargeSparseChangesStaySeparate(t *testing.T) {
	a := make([]string, 12000)
	for i := range a {
		a[i] = fmt.Sprintf("line %d\n", i)
	}
	b := append([]string(nil), a...)
	b[10] = "changed first\n"
	b[11990] = "changed last\n"
	old, next := strings.Join(a, ""), strings.Join(b, "")
	r := Compare(old, next)
	if r.Coarse || len(r.Hunks) != 2 {
		t.Fatal("sparse changes became coarse", r.Coarse, r.Hunks)
	}
	reconstruct(t, old, next)
}
func TestMyersMatchesAgainstLCS(t *testing.T) {
	var sequences [][]string
	for n := 0; n < 32; n++ {
		var s []string
		for bit := 0; bit < 5; bit++ {
			if n&(1<<bit) != 0 {
				s = append(s, "a")
			} else {
				s = append(s, "b")
			}
		}
		sequences = append(sequences, s)
	}
	for _, a := range sequences {
		for _, b := range sequences {
			pairs, ok := myersMatches(a, b)
			if !ok {
				t.Fatal("unexpected budget failure")
			}
			lastA, lastB := -1, -1
			for _, p := range pairs {
				if p.Old <= lastA || p.New <= lastB || a[p.Old] != b[p.New] {
					t.Fatal("invalid matching path", pairs)
				}
				lastA, lastB = p.Old, p.New
			}
			table := make([][]int, len(a)+1)
			for i := range table {
				table[i] = make([]int, len(b)+1)
			}
			for i := len(a) - 1; i >= 0; i-- {
				for j := len(b) - 1; j >= 0; j-- {
					if a[i] == b[j] {
						table[i][j] = table[i+1][j+1] + 1
					} else {
						table[i][j] = max(table[i+1][j], table[i][j+1])
					}
				}
			}
			if len(pairs) != table[0][0] {
				t.Fatal("nonminimal edit path", a, b, pairs)
			}
		}
	}
}
