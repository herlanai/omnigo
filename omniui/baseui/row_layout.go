package baseui

// rowSizes assigns growing children their share of remaining width before
// measuring wrapped height. Intrinsic flexible children shrink when necessary;
// explicitly sized children retain their requested width.
func (s *Surface) rowSizes(e *Element, width float64) map[*Element]Rect {
	result := map[*Element]Rect{}
	fixed, grow := 0., 0.
	count := 0
	for _, c := range e.children {
		if c.hidden || overlay(c) || (c.toast != nil && !c.toast.active) {
			continue
		}
		w, h := s.measure(c, width)
		if c.grow > 0 {
			grow += c.grow
			w = 0
		} else {
			fixed += w
		}
		result[c] = Rect{W: w, H: h}
		count++
	}
	available := max(0, width-float64(max(0, count-1))*e.gap)
	if fixed > available {
		flexible := 0.
		for _, c := range e.children {
			r, ok := result[c]
			if !ok {
				continue
			}
			if c.width == 0 && c.grow == 0 {
				flexible += r.W
			}
		}
		reduction := min(flexible, fixed-available)
		if flexible > 0 {
			for _, c := range e.children {
				r, ok := result[c]
				if !ok {
					continue
				}
				if c.width == 0 && c.grow == 0 {
					r.W = max(0, r.W-reduction*r.W/flexible)
					result[c] = r
				}
			}
			fixed -= reduction
		}
	}
	for _, c := range e.children {
		r, ok := result[c]
		if !ok {
			continue
		}
		if c.grow > 0 {
			r.W = max(0, available-fixed) * c.grow / grow
		}
		_, r.H = s.measure(c, r.W)
		result[c] = r
	}
	return result
}
