package i18n

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
)

// Offsets are byte offsets, as reported by encoding/json. Newline indexing
// avoids repeatedly scanning the prefix of a large catalog for every message.
type sourceLines []int

func indexLines(data []byte) sourceLines {
	lines := sourceLines{0}
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, i+1)
		}
	}
	return lines
}
func (lines sourceLines) at(offset int64) int {
	if offset < 0 {
		offset = 0
	}
	index, found := slices.BinarySearch(lines, int(offset))
	if found {
		return index + 1
	}
	return index
}
func nextJSONToken(data []byte, offset int64) int64 {
	for offset < int64(len(data)) {
		switch data[offset] {
		case ' ', '\r', '\n', '\t', ',':
			offset++
		default:
			return offset
		}
	}
	return offset
}
func messageLines(data []byte) map[string]int {
	lines, _ := catalogLocations(data)
	return lines
}

// Keep the first field, matching the catalog decoder when it reports and skips
// duplicates. Later declarations must not silently change type diagnostics.
func catalogLocations(data []byte) (map[string]int, map[string]json.RawMessage) {
	positions := map[string]int{}
	metadata := map[string]json.RawMessage{}
	lines := indexLines(data)
	decoder := json.NewDecoder(bytes.NewReader(data))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return positions, metadata
	}
	for decoder.More() {
		position := nextJSONToken(data, decoder.InputOffset())
		token, err := decoder.Token()
		if err != nil {
			return positions, metadata
		}
		name, ok := token.(string)
		if !ok {
			return positions, metadata
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return positions, metadata
		}
		if _, seen := positions[name]; seen {
			continue
		}
		positions[name] = lines.at(position)
		if strings.HasPrefix(name, "@") {
			metadata[name] = value
		}
	}
	return positions, metadata
}

type catalogLocationError struct {
	cause error
	line  int
}

func (e *catalogLocationError) Error() string { return e.cause.Error() }
func (e *catalogLocationError) Unwrap() error { return e.cause }
