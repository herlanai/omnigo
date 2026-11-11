// Package theme provides the single token source for Go baseui controls.
package theme

import (
	_ "embed"
	"encoding/json"
)

//go:embed tokens.json
var tokens []byte

type Palette map[string]string
type Tokens struct {
	FontSize      float64 `json:"fontSize"`
	LineHeight    float64 `json:"lineHeight"`
	Radius        float64 `json:"radius"`
	Spacing       float64 `json:"spacing"`
	ControlHeight float64 `json:"controlHeight"`
	CompactHeight float64 `json:"compactHeight"`
	TouchHeight   float64 `json:"touchHeight"`
	FocusWidth    float64 `json:"focusWidth"`
	Light         Palette `json:"light"`
	Dark          Palette `json:"dark"`
}

// Default returns an independent copy, so application overrides cannot mutate
// another application's theme.
func Default() Tokens {
	var result Tokens
	if err := json.Unmarshal(tokens, &result); err != nil {
		panic(err)
	}
	return result
}
