package baseui

import (
	"math/big"
	"strings"
)

// DecimalInputState separates an editable draft from an exact, committed decimal.
// Value uses a period and no grouping separators. State belongs to the UI goroutine.
type DecimalInputState struct {
	Value        string
	Text         *TextBuffer
	Error        string
	last, config string
}
type DecimalInputOptions struct {
	// Empty Min/Max means unbounded. Step determines the number of decimal places.
	Min, Max, Step string
	OnChange       func(string)
}

// decimalUnits parses decimal text without converting through floating point.
// Grouping, when present, must consist of three-digit groups after the first.
func decimalUnits(text string, scale int) (*big.Int, bool) {
	text = strings.TrimSpace(text)
	if len(text) == 0 || len(text) > 4096 {
		return nil, false
	}
	negative := text[0] == '-'
	if text[0] == '-' || text[0] == '+' {
		text = text[1:]
	}
	parts := strings.Split(text, ".")
	if len(parts) > 2 {
		return nil, false
	}
	whole := parts[0]
	if strings.Contains(whole, ",") {
		groups := strings.Split(whole, ",")
		if len(groups[0]) < 1 || len(groups[0]) > 3 {
			return nil, false
		}
		for _, g := range groups[1:] {
			if len(g) != 3 {
				return nil, false
			}
		}
		whole = strings.Join(groups, "")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if whole == "" && fraction == "" {
		return nil, false
	}
	for _, c := range whole + fraction {
		if c < '0' || c > '9' {
			return nil, false
		}
	}
	if len(fraction) > scale {
		if strings.Trim(fraction[scale:], "0") != "" {
			return nil, false
		}
		fraction = fraction[:scale]
	}
	digits := whole + fraction + strings.Repeat("0", scale-len(fraction))
	n, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return nil, false
	}
	if negative {
		n.Neg(n)
	}
	return n, true
}
func decimalString(n *big.Int, scale int) string {
	text := new(big.Int).Abs(n).String()
	if scale > 0 {
		if len(text) <= scale {
			text = strings.Repeat("0", scale+1-len(text)) + text
		}
		text = text[:len(text)-scale] + "." + text[len(text)-scale:]
	}
	if n.Sign() < 0 {
		text = "-" + text
	}
	return text
}

// DecimalInput provides exact stepping, validated editing and keyboard commit /
// cancellation. Callers remount after events, as with other baseui controls.
func DecimalInput(id, label string, state *DecimalInputState, o DecimalInputOptions) *Element {
	if state == nil {
		panic("baseui: nil decimal state")
	}
	scale := 0
	if i := strings.IndexByte(o.Step, '.'); i >= 0 {
		scale = len(o.Step) - i - 1
	}
	if scale > 128 {
		panic("baseui: decimal precision exceeds 128")
	}
	unit, ok := decimalUnits(o.Step, scale)
	if !ok || unit.Sign() <= 0 {
		panic("baseui: invalid decimal step")
	}
	limit := func(s string) *big.Int {
		if s == "" {
			return nil
		}
		n, valid := decimalUnits(s, scale)
		if !valid {
			panic("baseui: invalid decimal limit")
		}
		return n
	}
	lo, hi := limit(o.Min), limit(o.Max)
	if lo != nil && hi != nil && lo.Cmp(hi) > 0 {
		panic("baseui: reversed decimal limits")
	}
	inRange := func(n *big.Int) bool { return (lo == nil || n.Cmp(lo) >= 0) && (hi == nil || n.Cmp(hi) <= 0) }
	value, valid := decimalUnits(state.Value, scale)
	if !valid || !inRange(value) {
		panic("baseui: invalid decimal value")
	}
	config := o.Min + "\x00" + o.Max + "\x00" + o.Step
	if state.Text == nil || state.Value != state.last || config != state.config {
		state.Value = decimalString(value, scale)
		state.last, state.config = state.Value, config
		state.Text = NewTextBuffer(state.Value)
		state.Error = ""
	}
	set := func(n *big.Int, normalize bool) {
		next := decimalString(n, scale)
		changed := next != state.Value
		state.Value, state.last, state.Error = next, next, ""
		if normalize {
			state.Text.Set(next)
		}
		if changed && o.OnChange != nil {
			o.OnChange(next)
		}
	}
	step := func(sign int64) {
		n, _ := decimalUnits(state.Value, scale)
		n.Add(n, new(big.Int).Mul(unit, big.NewInt(sign)))
		if lo != nil && n.Cmp(lo) < 0 {
			n.Set(lo)
		}
		if hi != nil && n.Cmp(hi) > 0 {
			n.Set(hi)
		}
		set(n, true)
	}
	input := Input(id+"/input", label, state.Text).Grow(1)
	input.onChange = func(text string) {
		n, valid := decimalUnits(text, scale)
		if strings.TrimSpace(text) == "" {
			state.Error = ""
			return
		}
		if !valid {
			state.Error = "Enter a decimal with at most " + new(big.Int).SetInt64(int64(scale)).String() + " decimal places"
			return
		}
		if !inRange(n) {
			state.Error = "Value is outside the allowed range"
			return
		}
		set(n, false)
	}
	input.onKey = func(ev KeyEvent) bool {
		if state.Text.Composition() != "" {
			return false
		}
		switch ev.Key {
		case "Up":
			step(1)
		case "Down":
			step(-1)
		case "Enter", "Escape":
			state.Text.Set(state.Value)
			state.Error = ""
		default:
			return false
		}
		return true
	}
	input.Invalid(state.Error != "").Description(state.Error)
	minus := Button(id+"/decrease", "−", func() { step(-1) }).Variant(Outline).Width(32).Size(ControlSmall).Label(label + " decrease").Disabled(lo != nil && value.Cmp(lo) <= 0)
	plus := Button(id+"/increase", "+", func() { step(1) }).Variant(Outline).Width(32).Size(ControlSmall).Label(label + " increase").Disabled(hi != nil && value.Cmp(hi) >= 0)
	root := Row(id, input, minus, plus).Gap(4).Label(label).Role("group")
	root.centerItems = true
	return root
}
