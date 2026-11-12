package diff

// myersMatches computes a shortest edit path for similar large line sequences.
// The frontier and history are bounded by edit distance, not file line count.
// If either work or edit budget is exhausted the caller retains coarse fallback.
func myersMatches(a, b []string) ([]Row, bool) {
	limit := min(512, len(a)+len(b))
	offset := limit + 1
	frontier := make([]int, 2*limit+3)
	trace := make([][]int, 0, limit+1)
	work := 0
	for distance := 0; distance <= limit; distance++ {
		trace = append(trace, append([]int(nil), frontier...))
		for diagonal := -distance; diagonal <= distance; diagonal += 2 {
			work++
			if work > 8_000_000 {
				return nil, false
			}
			at := offset + diagonal
			var x int
			if diagonal == -distance || (diagonal != distance && frontier[at-1] < frontier[at+1]) {
				x = frontier[at+1]
			} else {
				x = frontier[at-1] + 1
			}
			y := x - diagonal
			for x < len(a) && y < len(b) && a[x] == b[y] {
				x++
				y++
				work++
				if work > 8_000_000 {
					return nil, false
				}
			}
			frontier[at] = x
			if x < len(a) || y < len(b) {
				continue
			}
			x, y = len(a), len(b)
			var matches []Row
			for d := distance; d > 0; d-- {
				previous := trace[d]
				k := x - y
				prevK := k - 1
				if k == -d || (k != d && previous[offset+k-1] < previous[offset+k+1]) {
					prevK = k + 1
				}
				px := previous[offset+prevK]
				py := px - prevK
				for x > px && y > py {
					x--
					y--
					matches = append(matches, Row{x, y})
				}
				x, y = px, py
			}
			for x > 0 && y > 0 {
				x--
				y--
				matches = append(matches, Row{x, y})
			}
			for left, right := 0, len(matches)-1; left < right; left, right = left+1, right-1 {
				matches[left], matches[right] = matches[right], matches[left]
			}
			return matches, true
		}
	}
	return nil, false
}
