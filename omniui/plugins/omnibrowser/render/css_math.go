package render

import (
	"math"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser/css"
	"strconv"
	"strings"
	"unicode"
)

func cssMath(v string, relative, fallback float64, width, height, depth int) float64 {
	if depth > 24 {
		return fallback
	}
	v = strings.TrimSpace(v)
	for _, name := range []string{"clamp", "min", "max"} {
		if strings.HasPrefix(v, name+"(") && strings.HasSuffix(v, ")") {
			args := css.Split(v[len(name)+1:len(v)-1], ',')
			if len(args) == 0 || len(args) > 16 {
				return fallback
			}
			values := make([]float64, len(args))
			for i, a := range args {
				values[i] = cssMath(a, relative, fallback, width, height, depth+1)
			}
			switch name {
			case "clamp":
				if len(values) == 3 {
					return max(values[0], min(values[1], values[2]))
				}
			case "min":
				result := values[0]
				for _, x := range values[1:] {
					result = min(result, x)
				}
				return result
			case "max":
				result := values[0]
				for _, x := range values[1:] {
					result = max(result, x)
				}
				return result
			}
			return fallback
		}
	}
	if strings.HasPrefix(v, "calc(") && strings.HasSuffix(v, ")") {
		v = v[5 : len(v)-1]
	}
	// Binary operations outside parentheses, low precedence first.
	for _, operators := range []string{"+-", "*/"} {
		level := 0
		for i := len(v) - 1; i > 0; i-- {
			switch v[i] {
			case ')':
				level++
			case '(':
				level--
			}
			if level == 0 && strings.ContainsRune(operators, rune(v[i])) {
				if (v[i] == '+' || v[i] == '-') && (!unicode.IsSpace(rune(v[i-1]))) {
					continue
				}
				a := cssMath(v[:i], relative, fallback, width, height, depth+1)
				b := cssMath(v[i+1:], relative, fallback, width, height, depth+1)
				switch v[i] {
				case '+':
					return a + b
				case '-':
					return a - b
				case '*':
					return a * b
				case '/':
					if b != 0 {
						return a / b
					}
					return fallback
				}
			}
		}
	}
	if strings.HasPrefix(v, "(") && strings.HasSuffix(v, ")") {
		return cssMath(v[1:len(v)-1], relative, fallback, width, height, depth+1)
	}
	scale := 1.
	for _, unit := range []string{"rem", "vw", "vh", "em", "px", "%"} {
		if strings.HasSuffix(v, unit) {
			v = strings.TrimSpace(strings.TrimSuffix(v, unit))
			switch unit {
			case "rem", "em":
				scale = 16
			case "vw":
				scale = float64(width) / 100
			case "vh":
				scale = float64(height) / 100
			case "%":
				scale = relative / 100
			}
			break
		}
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return fallback
	}
	return max(-16384, min(16384, n*scale))
}
