package watch

import (
	"errors"
	"io/fs"
)

// Outcome is the final result of a bound watch stream. An empty Code means the
// watch ended normally. Messages are fixed descriptions, never OS error text or
// filesystem paths. Transport failures and client cancellation remain stream
// errors and are distinct from these application results.
type Outcome struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// ErrLimit reports a snapshot that exceeds the configured entry budget.
var ErrLimit = errors.New("watch: entry limit exceeded")

func outcomeFor(err error) Outcome {
	switch {
	case err == nil:
		return Outcome{}
	case errors.Is(err, ErrDenied):
		return Outcome{Code: "denied", Message: "Watch request denied"}
	case errors.Is(err, ErrBusy), errors.Is(err, ErrLimit):
		return Outcome{Code: "limit", Message: "Watch limit reached"}
	case errors.Is(err, ErrEncoding):
		return Outcome{Code: "path-encoding", Message: "File path is not valid UTF-8"}
	case errors.Is(err, fs.ErrNotExist):
		return Outcome{Code: "not-found", Message: "Watch target not found"}
	case errors.Is(err, errors.ErrUnsupported):
		return Outcome{Code: "unsupported", Message: "Watch target is unsupported"}
	default:
		return Outcome{Code: "io", Message: "Filesystem watch failed"}
	}
}
