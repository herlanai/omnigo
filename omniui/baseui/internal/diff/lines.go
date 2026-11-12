// Package diff separates pure text comparison from DiffView UI state/painting.
// It uses bounded LCS and Myers comparisons implemented independently in OmniGo.
package diff

import "strings"

type Hunk struct{ OldStart, OldEnd, NewStart, NewEnd int }
type Row struct{ Old, New int }
type Result struct {
	OldLines, NewLines []string
	Hunks              []Hunk
	Rows               []Row
	Coarse             bool
}

func lines(s string) []string {
	if s == "" {
		return nil
	}
	a := strings.SplitAfter(s, "\n")
	if a[len(a)-1] == "" {
		a = a[:len(a)-1]
	}
	return a
}

// Compare preserves CRLF and trailing newline bytes. Unmatched interiors that
// exceed the Myers budget become one coarse replacement, indicated by Coarse.
func Compare(old, next string) Result {
	a, b := lines(old), lines(next)
	r := Result{OldLines: a, NewLines: b}
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	ae, be := len(a), len(b)
	for ae > prefix && be > prefix && a[ae-1] == b[be-1] {
		ae--
		be--
	}
	for i := 0; i < prefix; i++ {
		r.Rows = append(r.Rows, Row{i, i})
	}
	add := func(os, oe, ns, ne int) {
		if os == oe && ns == ne {
			return
		}
		r.Hunks = append(r.Hunks, Hunk{os, oe, ns, ne})
		for k := 0; k < max(oe-os, ne-ns); k++ {
			row := Row{-1, -1}
			if os+k < oe {
				row.Old = os + k
			}
			if ns+k < ne {
				row.New = ns + k
			}
			r.Rows = append(r.Rows, row)
		}
	}
	n, m := ae-prefix, be-prefix
	if n > 0 && m > 0 && n <= 1000000/m {
		width := m + 1
		table := make([]uint32, (n+1)*width)
		for i := n - 1; i >= 0; i-- {
			for j := m - 1; j >= 0; j-- {
				at := i*width + j
				if a[prefix+i] == b[prefix+j] {
					table[at] = 1 + table[at+width+1]
				} else {
					table[at] = max(table[at+width], table[at+1])
				}
			}
		}
		i, j, os, ns := 0, 0, prefix, prefix
		for i < n && j < m {
			if a[prefix+i] == b[prefix+j] {
				add(os, prefix+i, ns, prefix+j)
				r.Rows = append(r.Rows, Row{prefix + i, prefix + j})
				i++
				j++
				os, ns = prefix+i, prefix+j
			} else if table[(i+1)*width+j] >= table[i*width+j+1] {
				i++
			} else {
				j++
			}
		}
		add(os, ae, ns, be)
	} else if n > 0 && m > 0 {
		matches, exact := myersMatches(a[prefix:ae], b[prefix:be])
		if !exact {
			r.Coarse = true
			add(prefix, ae, prefix, be)
		} else {
			os, ns := prefix, prefix
			for _, pair := range matches {
				old, next := prefix+pair.Old, prefix+pair.New
				add(os, old, ns, next)
				r.Rows = append(r.Rows, Row{old, next})
				os, ns = old+1, next+1
			}
			add(os, ae, ns, be)
		}
	} else {
		add(prefix, ae, prefix, be)
	}
	for ae < len(a) && be < len(b) {
		r.Rows = append(r.Rows, Row{ae, be})
		ae++
		be++
	}
	return r
}
